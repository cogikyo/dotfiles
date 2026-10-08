package secureboot

import (
	"bytes"
	"context"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"
)

type fake struct {
	t        *testing.T
	root     string
	keys     bool
	unsigned bool
	conf     string
	totp     error
	calls    []string
}

const luks = "0d9e8f7a-6b5c-4d3e-9f2a-1b0c9d8e7f6a"

func (f *fake) Run(_ context.Context, _ string, name string, args ...string) error {
	line := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, line)
	switch line {
	case "sbctl create-keys":
		f.keys = true
		put(f.t, filepath.Join(f.root, sbctlPK), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("own PK")}))
	case "limine-update":
		writeUKI(f.t, filepath.Join(f.root, "boot", "EFI", "Linux", "linux.efi"))
		put(f.t, filepath.Join(f.root, "boot", "EFI", "limine", "limine_x64.efi"), []byte("limine"))
		if f.conf != "" {
			put(f.t, filepath.Join(f.root, "boot", "limine.conf"), []byte(f.conf))
		}
	case "sbctl enroll-keys":
		efivar(f.t, f.root, "SetupMode", 0)
		put(f.t, filepath.Join(f.root, "sys", "firmware", "efi", "efivars", "PK-"+global), []byte("\x27\x00\x00\x00list own PK"))
	}
	return nil
}

func (f *fake) Output(_ context.Context, _ string, name string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	switch {
	case name == "tpm2-totp" && args[len(args)-1] == "show":
		return "123456", f.totp
	case name == "sbctl" && args[0] == "status":
		return fmt.Sprintf(`{"installed": %t}`, f.keys), nil
	case name == "sbctl" && args[0] == "verify":
		signed := 1
		if f.unsigned {
			signed = 0
		}
		var verified []map[string]any
		for _, file := range args[2:] {
			verified = append(verified, map[string]any{"file_name": file, "is_signed": signed})
		}
		out, err := json.Marshal(verified)
		return string(out), err
	}
	return "", errors.New("unexpected command " + name)
}

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func efivar(t *testing.T, root, name string, v byte) {
	put(t, filepath.Join(root, "sys", "firmware", "efi", "efivars", name+"-"+global), []byte{7, 0, 0, 0, v})
}

func writeUKI(t *testing.T, path string) {
	t.Helper()
	var b bytes.Buffer
	dos := make([]byte, 64)
	copy(dos, "MZ")
	binary.LittleEndian.PutUint32(dos[0x3c:], 64)
	b.Write(dos)
	b.WriteString("PE\x00\x00")
	opt := pe.OptionalHeader64{Magic: 0x20b, NumberOfRvaAndSizes: 16}
	for _, v := range []any{pe.FileHeader{Machine: pe.IMAGE_FILE_MACHINE_AMD64, SizeOfOptionalHeader: uint16(binary.Size(opt))}, opt} {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	put(t, path, b.Bytes())
}

func machine(t *testing.T) *fake {
	f := &fake{t: t, root: t.TempDir(), conf: "/Arch Linux\n    protocol: efi\n    path: boot():/EFI/Linux/linux.efi\n    cmdline: rd.luks.name=" + luks + "=root root=/dev/mapper/root rootflags=subvol=/@ rw\n"}
	efivar(t, f.root, "SetupMode", 1)
	efivar(t, f.root, "SecureBoot", 0)
	put(t, filepath.Join(f.root, "etc", "default", "limine"), []byte("ESP_PATH=/boot\nENABLE_LIMINE_FALLBACK=yes\n"))
	put(t, filepath.Join(f.root, "boot", fallback), []byte("fallback"))
	put(t, filepath.Join(f.root, "dev", "nvme0n1p2"), nil)
	put(t, filepath.Join(f.root, "dev", "tpmrm0"), nil)
	put(t, filepath.Join(f.root, measured), synthetic(0, false))
	put(t, filepath.Join(f.root, "sys", "class", "block", "nvme0n1p2", "holders", "dm-0", "dm", "name"), []byte("root\n"))
	if err := os.MkdirAll(filepath.Join(f.root, "dev", "disk", "by-uuid"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../nvme0n1p2", filepath.Join(f.root, "dev", "disk", "by-uuid", luks)); err != nil {
		t.Fatal(err)
	}
	return f
}

func run(t *testing.T, f *fake, fix bool) map[string]setup.Result {
	t.Helper()
	mode := setup.Status
	if fix {
		mode = setup.Force
	}
	u := ui.New(ui.Options{JSON: true})
	reports, err := setup.Run(t.Context(), u, []setup.Stage{Stage(f, f.root), TOTP(u, f, f.root)}, mode)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]setup.Result{}
	for _, report := range reports {
		for _, r := range report.Items {
			byName[r.Item] = r
		}
	}
	return byName
}

func TestFixEnrolls(t *testing.T) {
	f := machine(t)
	rs := run(t, f, true)
	if rs[Keys].State != setup.Done || rs[Signed].State != setup.Done {
		t.Fatalf("after enrollment: %+v", rs)
	}
	if r := rs[Enforced]; r.State == setup.Done || !strings.HasPrefix(r.Detail, "reboot") {
		t.Errorf("before reboot %s = %+v, want a wait for the reboot", Enforced, r)
	}
	if r := rs["totp-sealed"]; r.State == setup.Done || !strings.Contains(r.Detail, "after reboot") {
		t.Errorf("before reboot totp-sealed = %+v, want a wait for the reboot", r)
	}
	order := []string{"sbctl create-keys", "limine-update", "sbctl verify", "sbctl enroll-keys"}
	at := -1
	for _, c := range order {
		i := slices.IndexFunc(f.calls, func(call string) bool { return strings.HasPrefix(call, c) })
		if i <= at {
			t.Fatalf("calls %q: want %q in order", f.calls, order)
		}
		at = i
	}
	if _, err := os.Stat(filepath.Join(f.root, "boot", fallback)); err == nil {
		t.Error("fallback loader left in place")
	}
	data, _ := os.ReadFile(filepath.Join(f.root, "etc", "default", "limine"))
	if want := "ESP_PATH=/boot\n" + strings.Join(settings, "\n") + "\n"; string(data) != want {
		t.Errorf("limine settings %q, want %q", data, want)
	}

	efivar(t, f.root, "SecureBoot", 1)
	put(t, filepath.Join(f.root, marker), []byte("label=test\n"))
	put(t, filepath.Join(f.root, built), []byte("label=test\n"))
	for name, r := range run(t, f, false) {
		if r.State != setup.Done {
			t.Errorf("after reboot %s = %+v", name, r)
		}
	}
	f.calls = nil
	for name, r := range run(t, f, true) {
		if r.State != setup.Done {
			t.Errorf("forced on an enforced machine %s = %+v", name, r)
		}
	}
	if slices.ContainsFunc(f.calls, func(c string) bool {
		return strings.HasPrefix(c, "sbctl create-keys") || strings.HasPrefix(c, "sbctl enroll-keys") || c == "limine-update"
	}) {
		t.Errorf("forced apply on an enforced machine ran %q", f.calls)
	}
}

func TestFixRefusesUnbootable(t *testing.T) {
	for name, tt := range map[string]struct {
		mutate func(*fake)
		want   string
	}{
		"unsigned":   {func(f *fake) { f.unsigned = true }, "not signed"},
		"no entry":   {func(f *fake) { f.conf = strings.ReplaceAll(f.conf, "linux.efi", "other.efi") }, "no entry for linux.efi"},
		"no cmdline": {func(f *fake) { f.conf = "/Arch Linux\n    path: boot():/EFI/Linux/linux.efi\n" }, "no cmdline"},
		"wrong uuid": {func(f *fake) { f.conf = strings.ReplaceAll(f.conf, luks, "6f1c2a7e-3b4d-4e5f-8a9b-0c1d2e3f4a5b") }, "names LUKS UUID 6f1c2a7e"},
		"wrong mapping": {func(f *fake) {
			f.conf = strings.ReplaceAll(f.conf, "=root root=/dev/mapper/root", "=crypt root=/dev/mapper/crypt")
		}, "not open as crypt"},
		"missing conf": {func(f *fake) { f.conf = "" }, "limine.conf: no such file"},
	} {
		t.Run(name, func(t *testing.T) {
			f := machine(t)
			tt.mutate(f)
			rs := run(t, f, true)
			if r := rs[Keys]; r.State != setup.Failed || !strings.Contains(r.Detail, tt.want) || !strings.Contains(r.Detail, "keys not enrolled") {
				t.Errorf("%s = %+v, want a refused enrollment naming %q", Keys, r, tt.want)
			}
			if slices.Contains(f.calls, "sbctl enroll-keys") {
				t.Error("enrolled keys over an unbootable configuration")
			}
		})
	}
}

func TestUnencryptedRootVerifies(t *testing.T) {
	f := machine(t)
	f.conf = "/Arch Linux\n    path: boot():/EFI/Linux/linux.efi\n    cmdline: root=UUID=" + luks + " rw\n"
	if r := run(t, f, true)[Keys]; r.State != setup.Done {
		t.Errorf("%s = %+v, want enrollment over an unencrypted root", Keys, r)
	}
	f.conf = "/Arch Linux\n    path: boot():/EFI/Linux/linux.efi\n    cmdline: rw quiet\n"
	f.Run(t.Context(), "", "limine-update")
	if err := verify(t.Context(), f, f.root); err == nil || !strings.Contains(err.Error(), "has no root=") {
		t.Errorf("verify without root= = %v", err)
	}
}

func TestSignedRepairs(t *testing.T) {
	f := machine(t)
	run(t, f, true)
	efivar(t, f.root, "SecureBoot", 1)
	put(t, filepath.Join(f.root, "boot", fallback), []byte("fallback"))
	put(t, filepath.Join(f.root, marker), []byte("label=test\n"))
	put(t, filepath.Join(f.root, built), []byte("label=test\n"))
	if r := run(t, f, false)[Signed]; r.State != setup.Pending || !strings.Contains(r.Detail, "fallback") {
		t.Fatalf("drifted %s = %+v, want pending", Signed, r)
	}
	f.calls = nil
	for name, r := range run(t, f, true) {
		if r.State != setup.Done {
			t.Errorf("after repair %s = %+v", name, r)
		}
	}
	if !slices.Contains(f.calls, "limine-update") || slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, "sbctl enroll-keys") }) {
		t.Errorf("repair calls %q, want limine-update without enrollment", f.calls)
	}
}

func TestPreflight(t *testing.T) {
	for name, tc := range map[string]struct {
		setup bool
		log   []byte
		want  string
		on    bool
	}{
		"option ROMs":         {true, synthetic(2, false), "2 option ROMs measured, so sbctl refuses to enroll", false},
		"option ROMs in BIOS": {false, synthetic(2, true), "2 option ROMs", false},
		"no event log":        {true, nil, "TPM event log missing", false},
		"factory keys":        {false, synthetic(0, true), "Erase all Secure Boot Settings", false},
		"factory keys on":     {false, synthetic(0, true), "keys dctl did not create", true},
		"ready":               {true, synthetic(0, false), "keys not enrolled", false},
	} {
		t.Run(name, func(t *testing.T) {
			f := machine(t)
			efivar(t, f.root, "SetupMode", map[bool]byte{true: 1}[tc.setup])
			efivar(t, f.root, "SecureBoot", map[bool]byte{true: 1}[tc.on])
			if tc.log == nil {
				os.Remove(filepath.Join(f.root, measured))
			} else {
				put(t, filepath.Join(f.root, measured), tc.log)
			}
			if r := run(t, f, false)[Keys]; !strings.Contains(r.Detail, tc.want) {
				t.Errorf("%s = %+v, want %q", Keys, r, tc.want)
			}
			if name == "ready" {
				return
			}
			run(t, f, true)
			if slices.ContainsFunc(f.calls, func(c string) bool {
				return strings.HasPrefix(c, "sbctl create-keys") || strings.HasPrefix(c, "sbctl enroll-keys")
			}) {
				t.Errorf("forced apply ran %q", f.calls)
			}
		})
	}
}

func TestVendorKeys(t *testing.T) {
	f := machine(t)
	put(t, filepath.Join(f.root, sbctlConf), []byte("db_additions:\n- microsoft\n"))
	if r := run(t, f, true)[Keys]; r.State != setup.ManualState || !strings.Contains(r.Detail, "adds microsoft to db") || slices.Contains(f.calls, "sbctl enroll-keys") {
		t.Fatalf("%s = %+v with db_additions, calls %q", Keys, r, f.calls)
	}
	os.Remove(filepath.Join(f.root, sbctlConf))
	run(t, f, true)
	db := append([]byte{7, 0, 0, 0}, microsoft...)
	put(t, filepath.Join(f.root, "sys", "firmware", "efi", "efivars", "db-"+security), db)
	if r := run(t, f, false)[Keys]; r.State != setup.ManualState || !strings.Contains(r.Detail, "Microsoft certificates remain in db") {
		t.Errorf("%s = %+v, want Microsoft certificates flagged", Keys, r)
	}
}

func TestStillOffAfterReboot(t *testing.T) {
	f := machine(t)
	run(t, f, true)
	put(t, filepath.Join(f.root, measured), synthetic(0, true))
	if r := run(t, f, false)[Enforced]; r.State != setup.ManualState || !strings.Contains(r.Detail, "Secure Boot still off") {
		t.Errorf("%s = %+v, want manual", Enforced, r)
	}
}

func TestSealed(t *testing.T) {
	none := errors.New("tpm2-totp: exit status 1: No TOTP secret is currently stored, use 'init' to generate and store one.")
	for name, tc := range map[string]struct {
		err    error
		tpm    bool
		marked bool
		want   error
		text   string
	}{
		"sealed":          {nil, true, true, nil, ""},
		"sealed unmarked": {nil, true, false, errUnmarked, ""},
		"unsealed":        {none, true, false, errUnsealed, ""},
		"gone":            {none, true, true, nil, "NO BOOT TOTP: sealed secret missing"},
		"changed":         {errors.New("tpm2-totp: exit status 1: The system state has changed, no TOTP could be calculated."), true, false, nil, "NO BOOT TOTP: PCR 0 or 7 changed"},
		"lockout":         {errors.New("tpm2-totp: exit status 1: The password has been entered wrongly too many times and the TPM is in lockout mode."), true, true, nil, "lockout"},
		"no TPM":          {nil, false, true, nil, "no TPM at"},
		"tcti fail":       {errors.New("tpm2-totp: exit status 1: ERROR in main: 0xa000a - tcti:IO failure"), true, true, nil, "IO failure"},
	} {
		f := machine(t)
		efivar(t, f.root, "SetupMode", 0)
		efivar(t, f.root, "SecureBoot", 1)
		if !tc.tpm {
			os.Remove(filepath.Join(f.root, "dev", "tpmrm0"))
		}
		if tc.marked {
			put(t, filepath.Join(f.root, marker), []byte("label=test\n"))
			put(t, filepath.Join(f.root, built), []byte("label=test\n"))
		}
		f.totp = tc.err
		err := sealed(t.Context(), f, f.root)
		switch {
		case tc.text != "":
			if r := run(t, f, false)["totp-sealed"]; r.State != setup.ManualState || !strings.Contains(r.Detail, tc.text) {
				t.Errorf("%s: %+v, want manual naming %q", name, r, tc.text)
			}
		case !errors.Is(err, tc.want) || (tc.want == nil) != (err == nil):
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
}

func TestMarkerRepair(t *testing.T) {
	f := machine(t)
	run(t, f, true)
	efivar(t, f.root, "SecureBoot", 1)
	put(t, filepath.Join(f.root, marker), []byte("label=test\n"))
	put(t, filepath.Join(f.root, built), []byte("label=old\n"))
	if r := run(t, f, false)["totp-sealed"]; r.State != setup.Pending {
		t.Fatalf("totp-sealed with stale images = %+v, want pending", r)
	}
	f.calls = nil
	if r := run(t, f, true)["totp-sealed"]; r.State != setup.Done {
		t.Fatalf("totp-sealed = %+v, want the images rebuilt", r)
	}
	if stamp, err := os.ReadFile(filepath.Join(f.root, built)); err != nil || string(stamp) != "label=test\n" || !slices.Contains(f.calls, "limine-update") {
		t.Errorf("stamp %q %v, calls %q", stamp, err, f.calls)
	}
	if slices.ContainsFunc(f.calls, func(c string) bool { return strings.Contains(c, " init") || strings.Contains(c, " clean") }) {
		t.Errorf("rebuilding the images resealed: %q", f.calls)
	}
}
