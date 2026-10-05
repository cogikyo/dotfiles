package secureboot

import (
	"bytes"
	"context"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
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
	calls    []string
}

const luks = "0d9e8f7a-6b5c-4d3e-9f2a-1b0c9d8e7f6a"

func (f *fake) Run(_ context.Context, _ string, name string, args ...string) error {
	line := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, line)
	switch line {
	case "sbctl create-keys":
		f.keys = true
	case "limine-update":
		writeUKI(f.t, filepath.Join(f.root, "boot", "EFI", "Linux", "linux.efi"))
		put(f.t, filepath.Join(f.root, "boot", "EFI", "limine", "limine_x64.efi"), []byte("limine"))
		if f.conf != "" {
			put(f.t, filepath.Join(f.root, "boot", "limine.conf"), []byte(f.conf))
		}
	case "sbctl enroll-keys -m":
		efivar(f.t, f.root, "SetupMode", 0)
	}
	return nil
}

func (f *fake) Output(_ context.Context, _ string, name string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	switch {
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
	reports, err := setup.Run(t.Context(), ui.New(ui.Options{JSON: true}), []setup.Stage{Stage(f, f.root)}, mode)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]setup.Result{}
	for _, r := range reports[0].Items {
		byName[r.Item] = r
	}
	return byName
}

func TestFixEnrolls(t *testing.T) {
	f := machine(t)
	rs := run(t, f, true)
	if rs[Keys].State != setup.Done || rs[Signed].State != setup.Done {
		t.Fatalf("after enrollment: %+v", rs)
	}
	if r := rs[Enforced]; r.State != setup.ManualState || !strings.Contains(r.Detail, "reboot") {
		t.Errorf("before reboot %s = %+v, want manual on a reboot", Enforced, r)
	}
	order := []string{"sbctl create-keys", "limine-update", "sbctl verify", "sbctl enroll-keys -m"}
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
			if slices.Contains(f.calls, "sbctl enroll-keys -m") {
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
