package secureboot

import (
	"bytes"
	"context"
	"debug/pe"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"

	"gopkg.in/yaml.v3"
)

const (
	global    = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	security  = "d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	fallback  = "EFI/BOOT/BOOTX64.EFI"
	bios      = "reboot; F2 → Erase all Secure Boot Settings → F10 to save; then run dctl setup"
	sbctlPK   = "var/lib/sbctl/keys/PK/PK.pem"
	sbctlConf = "etc/sbctl/sbctl.conf"
	measured  = "sys/kernel/security/tpm0/binary_bios_measurements"

	Keys     = "secureboot-keys"
	Signed   = "secureboot-signed"
	Enforced = "secureboot-enforced"
)

var settings = []string{"ENABLE_ENROLL_LIMINE_CONFIG=yes", "ENABLE_LIMINE_FALLBACK=no"}

var microsoft = []byte{0xbd, 0x9a, 0xfa, 0x77, 0x59, 0x03, 0x32, 0x4d, 0xbd, 0x60, 0x28, 0xf4, 0xe7, 0x8f, 0x78, 0x4b}

type Firmware struct {
	UEFI    bool
	Setup   bool
	Enabled bool
}

func Read(efi string) (Firmware, error) {
	if _, err := os.Stat(efi); errors.Is(err, fs.ErrNotExist) {
		return Firmware{}, nil
	} else if err != nil {
		return Firmware{}, err
	}
	setup, err := variable(efi, "SetupMode")
	if err != nil {
		return Firmware{}, err
	}
	enabled, err := variable(efi, "SecureBoot")
	if err != nil {
		return Firmware{}, err
	}
	return Firmware{UEFI: true, Setup: setup, Enabled: enabled}, nil
}

func variable(efi, name string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(efi, "efivars", name+"-"+global))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(data) != 5 {
		return false, fmt.Errorf("efivar %s: %d bytes, want 4 attribute bytes and 1 value byte", name, len(data))
	}
	return data[4] == 1, nil
}

func Stage(run execx.Runner, root string) setup.Stage {
	efi := filepath.Join(root, "sys", "firmware", "efi")
	esp := filepath.Join(root, "boot")
	limine := filepath.Join(root, "etc", "default", "limine")
	errReady := errors.New("firmware in Setup Mode; keys not enrolled")
	keys := func(context.Context) error {
		fw, err := Read(efi)
		switch {
		case err != nil:
			return err
		case !fw.UEFI:
			return setup.Manual("booted without UEFI; reboot in UEFI mode")
		}
		ours, err := own(root)
		switch {
		case err != nil:
			return err
		case ours:
			return vendors(efi)
		}
		if err := preflight(root); err != nil {
			return err
		}
		switch {
		case fw.Setup:
			return errReady
		case fw.Enabled:
			return setup.Manual("Secure Boot is on with keys dctl did not create; %s", bios)
		}
		return setup.Later("%s", bios)
	}
	sign := func(ctx context.Context) error {
		if err := configure(limine); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(esp, fallback)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return run.Run(ctx, "", "limine-update")
	}
	signed := func(ctx context.Context) error {
		ours, err := own(root)
		switch {
		case err != nil:
			return err
		case !ours:
			return setup.Later("after key enrollment")
		}
		return verify(ctx, run, root)
	}
	return setup.Stage{Name: "secureboot", Root: true, Items: []setup.Item{
		{
			Name:  Keys,
			Check: keys,
			Fix: func(ctx context.Context) error {
				if !errors.Is(keys(ctx), errReady) {
					return nil
				}
				if err := additions(root); err != nil {
					return err
				}
				created, err := installed(ctx, run)
				if err != nil {
					return err
				}
				if !created {
					if err := run.Run(ctx, "", "sbctl", "create-keys"); err != nil {
						return err
					}
				}
				if err := sign(ctx); err != nil {
					return err
				}
				if err := verify(ctx, run, root); err != nil {
					return fmt.Errorf("%w; keys not enrolled", err)
				}
				if err := run.Run(ctx, "", "sbctl", "enroll-keys"); err != nil {
					return fmt.Errorf("%w; keys not enrolled; do not add Microsoft keys or force sbctl", err)
				}
				return nil
			},
		},
		{
			Name:  Signed,
			Check: signed,
			Fix: func(ctx context.Context) error {
				if ours, err := own(root); err != nil || !ours || signed(ctx) == nil {
					return nil
				}
				return sign(ctx)
			},
		},
		{
			Name: Enforced,
			Check: func(context.Context) error {
				fw, err := Read(efi)
				switch {
				case err != nil:
					return err
				case fw.Enabled && !fw.Setup:
					return nil
				}
				ours, err := own(root)
				switch {
				case err != nil:
					return err
				case !ours:
					return setup.Later("after key enrollment")
				}
				if log, err := readLog(root); err == nil && log.pk {
					return setup.Manual("Secure Boot still off; reboot, press F2, enable Secure Boot, save with F10, then run dctl setup")
				}
				return setup.Later("reboot; if Secure Boot is still off, enable it in the BIOS with F2 and save with F10; then run dctl setup")
			},
		},
	}}
}

func readLog(root string) (eventlog, error) {
	data, err := os.ReadFile(filepath.Join(root, measured))
	if err != nil {
		return eventlog{}, err
	}
	return parseLog(data)
}

func preflight(root string) error {
	log, err := readLog(root)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return setup.Manual("TPM event log missing, so sbctl refuses to enroll; check that the TPM is enabled in the BIOS")
	case err != nil:
		return setup.Manual("cannot read the TPM event log: %v", err)
	case log.oproms > 0:
		return setup.Manual("%d option ROMs measured, so sbctl refuses to enroll; trusting them needs sbctl enroll-keys --tpm-eventlog, which adds their hashes to db; dctl does not run it, so that decision is yours", log.oproms)
	}
	return nil
}

func vendors(efi string) error {
	var found []string
	for name, guid := range map[string]string{"KEK": global, "db": security} {
		data, err := os.ReadFile(filepath.Join(efi, "efivars", name+"-"+guid))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if bytes.Contains(data, microsoft) {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		slices.Sort(found)
		return setup.Manual("Microsoft certificates remain in %s; %s", strings.Join(found, " and "), bios)
	}
	return nil
}

func additions(root string) error {
	data, err := os.ReadFile(filepath.Join(root, sbctlConf))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var conf struct {
		Additions []string `yaml:"db_additions"`
	}
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return fmt.Errorf("/%s: %w", sbctlConf, err)
	}
	if len(conf.Additions) > 0 {
		return setup.Manual("refusing to enroll: /%s adds %s to db; remove db_additions, then run dctl setup", sbctlConf, strings.Join(conf.Additions, ", "))
	}
	return nil
}

func own(root string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(root, sbctlPK))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false, fmt.Errorf("%s contains no PEM certificate", sbctlPK)
	}
	pk, err := os.ReadFile(filepath.Join(root, "sys", "firmware", "efi", "efivars", "PK-"+global))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bytes.Contains(pk, block.Bytes), nil
}

func verify(ctx context.Context, run execx.Runner, root string) error {
	esp := filepath.Join(root, "boot")
	limine := filepath.Join(root, "etc", "default", "limine")
	if _, err := os.Stat(filepath.Join(esp, fallback)); err == nil {
		return fmt.Errorf("fallback %s exists; it is neither signed nor config-enrolled", fallback)
	}
	data, err := os.ReadFile(limine)
	if err != nil {
		return err
	}
	for _, want := range settings {
		if !strings.Contains("\n"+string(data), "\n"+want+"\n") {
			return fmt.Errorf("%s lacks %s", limine, want)
		}
	}
	ukis, err := filepath.Glob(filepath.Join(esp, "EFI", "Linux", "*.efi"))
	if err != nil {
		return err
	}
	if len(ukis) == 0 {
		return fmt.Errorf("no UKI in %s", filepath.Join(esp, "EFI", "Linux"))
	}
	errs := []error{entries(root, esp, ukis)}
	for _, uki := range ukis {
		_, embedded, err := Cmdline(uki)
		switch {
		case err != nil:
			errs = append(errs, err)
		case embedded:
			errs = append(errs, fmt.Errorf("%s embeds a cmdline; snapshot entries cannot boot under Secure Boot", filepath.Base(uki)))
		}
	}
	files := append([]string{filepath.Join(esp, "EFI", "limine", "limine_x64.efi")}, ukis...)
	out, err := run.Output(ctx, "", "sbctl", append([]string{"verify", "--json"}, files...)...)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	var verified []struct {
		File   string `json:"file_name"`
		Signed int8   `json:"is_signed"`
	}
	if err := json.Unmarshal([]byte(out), &verified); err != nil {
		return errors.Join(append(errs, fmt.Errorf("sbctl verify --json: %w", err))...)
	}
	signed := map[string]bool{}
	for _, v := range verified {
		signed[v.File] = v.Signed == 1
	}
	if bad := slices.DeleteFunc(files, func(f string) bool { return signed[f] }); len(bad) > 0 {
		errs = append(errs, fmt.Errorf("not signed by the sbctl db key: %s", strings.Join(bad, ", ")))
	}
	return errors.Join(errs...)
}

func entries(root, esp string, ukis []string) error {
	conf, err := os.ReadFile(filepath.Join(esp, "limine.conf"))
	if err != nil {
		return err
	}
	var errs []error
	for _, uki := range ukis {
		if !strings.Contains(string(conf), "/EFI/Linux/"+filepath.Base(uki)) {
			errs = append(errs, fmt.Errorf("limine.conf has no entry for %s", filepath.Base(uki)))
		}
	}
	cmdlines := 0
	for line := range strings.Lines(string(conf)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "cmdline:"); ok {
			cmdlines++
			errs = append(errs, opened(root, strings.Fields(v)))
		}
	}
	if cmdlines == 0 {
		errs = append(errs, errors.New("limine.conf has no cmdline"))
	}
	return errors.Join(errs...)
}

func opened(root string, cmdline []string) error {
	var id, name, dev string
	for _, f := range cmdline {
		if v, ok := strings.CutPrefix(f, "rd.luks.name="); ok {
			id, name, _ = strings.Cut(v, "=")
		}
		if v, ok := strings.CutPrefix(f, "root="); ok {
			dev = v
		}
	}
	switch {
	case dev == "":
		return fmt.Errorf("limine.conf cmdline %q has no root=", strings.Join(cmdline, " "))
	case id == "":
		return nil
	case name == "" || dev != "/dev/mapper/"+name:
		return fmt.Errorf("limine.conf cmdline %q: want rd.luks.name=<uuid>=<name> and root=/dev/mapper/<name>", strings.Join(cmdline, " "))
	}
	part, err := filepath.EvalSymlinks(filepath.Join(root, "dev", "disk", "by-uuid", id))
	if err != nil {
		return fmt.Errorf("limine.conf cmdline names LUKS UUID %s: %w", id, err)
	}
	holders, err := filepath.Glob(filepath.Join(root, "sys", "class", "block", filepath.Base(part), "holders", "*", "dm", "name"))
	if err != nil {
		return err
	}
	for _, h := range holders {
		if data, err := os.ReadFile(h); err == nil && strings.TrimSpace(string(data)) == name {
			return nil
		}
	}
	return fmt.Errorf("limine.conf cmdline names LUKS UUID %s, but %s is not open as %s", id, filepath.Base(part), name)
}

// Cmdline reads the UKI's .cmdline section; the boolean reports section presence, even when its contents are empty.
func Cmdline(uki string) (string, bool, error) {
	f, err := pe.Open(uki)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	sec := f.Section(".cmdline")
	if sec == nil {
		return "", false, nil
	}
	data, err := sec.Data()
	if err != nil {
		return "", false, err
	}
	return strings.TrimRight(string(data), "\x00\n "), true, nil
}

func installed(ctx context.Context, run execx.Runner) (bool, error) {
	out, err := run.Output(ctx, "", "sbctl", "status", "--json")
	if err != nil {
		return false, err
	}
	var state struct {
		Installed bool `json:"installed"`
	}
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		return false, fmt.Errorf("sbctl status: %w", err)
	}
	return state.Installed, nil
}

func configure(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var lines []string
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSuffix(line, "\n")
		if !slices.ContainsFunc(settings, func(s string) bool { key, _, _ := strings.Cut(s, "="); return strings.HasPrefix(line, key+"=") }) {
			lines = append(lines, line)
		}
	}
	lines = append(lines, settings...)
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
