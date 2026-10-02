package secureboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

const (
	global = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	bios   = "Secure Boot is off and the firmware is not in Setup Mode: in the BIOS, erase the Secure Boot keys (Setup Mode), boot, then run sudo dctl doctor --fix secureboot"
)

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

const Fallback = "EFI/BOOT/BOOTX64.EFI"

func Files(esp string) ([]string, error) {
	ukis, err := filepath.Glob(filepath.Join(esp, "EFI", "Linux", "*.efi"))
	if err != nil {
		return nil, err
	}
	if len(ukis) == 0 {
		return nil, fmt.Errorf("no UKI in %s", filepath.Join(esp, "EFI", "Linux"))
	}
	return append([]string{filepath.Join(esp, "EFI", "limine", "limine_x64.efi")}, ukis...), nil
}

func Verify(files []string) []string {
	return append([]string{"sbctl", "verify", "--json"}, files...)
}

func Unsigned(out []byte, files []string) ([]string, error) {
	var verified []struct {
		File   string `json:"file_name"`
		Signed int8   `json:"is_signed"`
	}
	if err := json.Unmarshal(out, &verified); err != nil {
		return nil, fmt.Errorf("sbctl verify --json: %w", err)
	}
	signed := map[string]bool{}
	for _, v := range verified {
		signed[v.File] = v.Signed == 1
	}
	var bad []string
	for _, f := range files {
		if !signed[f] {
			bad = append(bad, f)
		}
	}
	return bad, nil
}

func unsigned(ctx context.Context, run execx.Runner, esp string) error {
	files, err := Files(esp)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(esp, Fallback)); err == nil {
		files = append(files, filepath.Join(esp, Fallback))
	}
	args := Verify(files)
	out, err := run.Output(ctx, "", args[0], args[1:]...)
	if err != nil {
		return err
	}
	bad, err := Unsigned([]byte(out), files)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("not signed by the sbctl db key: %s", strings.Join(bad, ", "))
	}
	return nil
}

func Group(run execx.Runner, root string) doctor.Group {
	efi := filepath.Join(root, "sys", "firmware", "efi")
	esp := filepath.Join(root, "boot")
	limine := filepath.Join(root, "etc", "default", "limine")
	return doctor.Group{Name: "secureboot", Root: true, Checks: []doctor.Check{
		{
			Name: "secureboot-keys",
			Check: func(ctx context.Context) error {
				fw, err := Read(efi)
				switch {
				case err != nil:
					return err
				case !fw.UEFI:
					return doctor.Block("not booted with UEFI")
				case fw.Setup:
					return errors.New("firmware is in Setup Mode: no Secure Boot keys enrolled")
				case fw.Enabled:
					return nil
				}
				keys, err := installed(ctx, run)
				switch {
				case err != nil:
					return doctor.Block("%v", err)
				case keys:
					return doctor.Block("sbctl keys exist but Secure Boot is off: enable Secure Boot in the BIOS")
				}
				return doctor.Block("%s", bios)
			},
			Fix: func(ctx context.Context) error {
				fw, err := Read(efi)
				if err != nil {
					return err
				}
				if !fw.Setup {
					return doctor.Block("%s", bios)
				}
				keys, err := installed(ctx, run)
				if err != nil {
					return err
				}
				if !keys {
					if err := run.Run(ctx, "", "sbctl", "create-keys"); err != nil {
						return err
					}
				}
				if err := configure(limine); err != nil {
					return err
				}
				if err := os.Remove(filepath.Join(esp, Fallback)); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				if err := run.Run(ctx, "", "limine-update"); err != nil {
					return err
				}
				if err := unsigned(ctx, run, esp); err != nil {
					return fmt.Errorf("%w; keys not enrolled", err)
				}
				return run.Run(ctx, "", "sbctl", "enroll-keys", "-m")
			},
		},
		{
			Name: "secureboot-signed",
			Check: func(ctx context.Context) error {
				keys, err := installed(ctx, run)
				switch {
				case err != nil:
					return doctor.Block("%v", err)
				case !keys:
					return doctor.Block("no sbctl keys yet; see secureboot-keys")
				}
				if err := unsigned(ctx, run, esp); err != nil {
					return err
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
				return nil
			},
		},
	}}
}

func installed(ctx context.Context, run execx.Runner) (bool, error) {
	out, err := run.Output(ctx, "", "sbctl", "setup", "--print-state", "--json")
	if err != nil {
		return false, err
	}
	var state struct {
		Installed bool `json:"installed"`
	}
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		return false, fmt.Errorf("sbctl setup --print-state: %w", err)
	}
	return state.Installed, nil
}

var settings = []string{"ENABLE_ENROLL_LIMINE_CONFIG=yes", "ENABLE_LIMINE_FALLBACK=no"}

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
