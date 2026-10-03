package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/ui"
)

type fake struct {
	t     *testing.T
	root  string
	lsblk []byte
	vm    bool
	block bool
	fail  string
	sb    []doctor.Result

	mu        sync.Mutex
	calls     [][]string
	mounts    []string
	confirmed int
	drained   bool
}

func (f *fake) record(args []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, slices.Clone(args))
}

func (f *fake) luksID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if len(c) > 1 && c[0] == "cryptsetup" && c[1] == "luksFormat" {
			return value(c, "--uuid")
		}
	}
	return ""
}

func (f *fake) mount(add string, drop func(string) bool) {
	f.mounts = slices.DeleteFunc(f.mounts, drop)
	if add != "" {
		f.mounts = append(f.mounts, add)
	}
	var b strings.Builder
	for i, m := range f.mounts {
		b.WriteString(strconv.Itoa(i) + " 1 0:1 / " + m + " rw - btrfs /dev/mapper/root rw\n")
	}
	put(f.t, filepath.Join(f.root, "proc", "self", "mountinfo"), []byte(b.String()), 0o644)
}

func (f *fake) run(_ context.Context, _ []byte, args ...string) error {
	f.record(args)
	line := strings.Join(args, " ")
	if f.fail != "" && strings.HasPrefix(line, f.fail) {
		return errors.New("injected failure: " + line)
	}
	last := args[len(args)-1]
	dev := filepath.Join(f.root, "dev", "mapper", mapper)
	switch {
	case strings.HasPrefix(line, "cryptsetup open"):
		put(f.t, dev, nil, 0o600)
	case line == "cryptsetup close "+mapper:
		os.Remove(dev)
	case line == "umount -R "+target:
		f.mount("", func(m string) bool { return m == target || strings.HasPrefix(m, target+"/") })
	case args[0] == "umount":
		f.mount("", func(m string) bool { return m == last })
	case args[0] == "mount" && strings.HasPrefix(last, target):
		f.mount(last, func(m string) bool { return m == last })
	case line == "arch-chroot /mnt limine-update":
		f.kernel()
	}
	return nil
}

func (f *fake) kernel() {
	esp := filepath.Join(f.root, "mnt", "boot")
	writeUKI(f.t, filepath.Join(esp, "EFI", "Linux", "0123_linux.efi"))
	conf := "path: boot():/EFI/Linux/0123_linux.efi\ncmdline: rd.luks.name=" + f.luksID() + "=root root=/dev/mapper/root\n"
	put(f.t, filepath.Join(esp, "limine.conf"), []byte(conf), 0o644)
}

func (f *fake) output(ctx context.Context, args ...string) ([]byte, error) {
	f.record(args)
	switch args[0] {
	case "lsblk":
		if f.block {
			<-ctx.Done()
			time.Sleep(time.Second)
			f.mu.Lock()
			f.drained = true
			f.mu.Unlock()
			return nil, ctx.Err()
		}
		return f.lsblk, nil
	case "systemd-detect-virt":
		if !f.vm {
			return []byte("none\n"), errors.New("exit status 1")
		}
		return []byte("kvm\n"), nil
	case "blkid":
		return []byte(f.luksID() + "\n"), nil
	case "arch-chroot":
		if args[len(args)-1] == "secureboot" {
			f.kernel()
			out, _ := json.Marshal(f.sb)
			return out, errors.New("exit status 1")
		}
	}
	return nil, errors.New("unexpected output command " + args[0])
}

func (f *fake) index(prefix string) int {
	return slices.IndexFunc(f.calls, func(c []string) bool { return strings.HasPrefix(strings.Join(c, " "), prefix) })
}

func (f *fake) before(t *testing.T, first, then string) {
	t.Helper()
	i, j := f.index(first), f.index(then)
	if i < 0 || j < 0 || i >= j {
		t.Errorf("want %q (at %d) before %q (at %d)", first, i, then, j)
	}
}

func readOnly(args []string) bool {
	switch args[0] {
	case "lsblk", "systemd-detect-virt", "blkid":
		return true
	case "mount":
		return slices.Contains(args, "ro")
	}
	return false
}

func put(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
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
	put(t, path, b.Bytes(), 0o644)
}

func setup(t *testing.T) (*session, *fake) {
	t.Helper()
	root := t.TempDir()
	pkg := []byte("package bytes")
	sum := sha256.Sum256(pkg)
	put(t, filepath.Join(root, "run", "archiso", "bootmnt", ".keep"), nil, 0o644)
	put(t, filepath.Join(root, "opt", "dctl", "payload", "base-1-1-any.pkg.tar.zst"), pkg, 0o644)
	put(t, filepath.Join(root, "opt", "dctl", "payload", "SHA256SUMS"), []byte(hex.EncodeToString(sum[:])+"  base-1-1-any.pkg.tar.zst\n"), 0o644)
	put(t, filepath.Join(root, "opt", "dctl", "targets"), []byte("base\nlinux\n"), 0o644)
	put(t, filepath.Join(root, "opt", "dctl", "dotfiles.bundle"), []byte("bundle"), 0o644)
	for _, name := range binaries.Names {
		put(t, filepath.Join(root, "usr", "local", "bin", name), []byte(name), 0o755)
	}
	put(t, filepath.Join(root, "proc", "cmdline"), []byte("archisobasedir=arch archisosearchuuid=2026-10-01-12-00-00-00 copytoram\n"), 0o444)
	efivars := filepath.Join(root, "sys", "firmware", "efi", "efivars")
	put(t, filepath.Join(efivars, "SetupMode-8be4df61-93ca-11d2-aa0d-00e098032b8c"), []byte{7, 0, 0, 0, 1}, 0o644)
	put(t, filepath.Join(efivars, "SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"), []byte{7, 0, 0, 0, 0}, 0o644)
	put(t, filepath.Join(root, "usr", "share", "zoneinfo", "America", "Denver"), []byte("TZif"), 0o644)
	if err := os.MkdirAll(filepath.Join(root, "mnt", "var", "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}

	f := &fake{t: t, root: root, lsblk: fixture(t, "nvme"), sb: []doctor.Result{
		{Group: "secureboot", Check: "secureboot-keys", Status: doctor.Fixed},
		{Group: "secureboot", Check: "secureboot-signed", Status: doctor.Passed},
		{Group: "secureboot", Check: "secureboot-enforced", Status: doctor.Blocked},
	}}
	f.mount("", func(string) bool { return false })
	s := &session{u: ui.New(ui.Options{JSON: true, Yes: true}), sh: f, root: root, exe: filepath.Join(root, "usr", "local", "bin", "dctl")}
	s.ask = func(context.Context) (iso.Answers, error) {
		return iso.Answers{Password: "pw", Zone: "America/Denver", LUKS: "luks"}, nil
	}
	s.confirm = func(disk) error {
		f.confirmed = len(f.calls)
		return nil
	}
	return s, f
}

func (f *fake) mutated(t *testing.T) {
	t.Helper()
	for _, c := range f.calls {
		if !readOnly(c) {
			t.Errorf("ran mutating %q", c)
		}
	}
}

func TestDeclinedConsentMutatesNothing(t *testing.T) {
	s, f := setup(t)
	declined := errors.New("declined")
	s.confirm = func(disk) error { return declined }
	if err := s.main(t.Context()); !errors.Is(err, declined) {
		t.Fatalf("main: %v, want %v", err, declined)
	}
	f.mutated(t)
}

func TestEdges(t *testing.T) {
	s, f := setup(t)
	if err := s.main(t.Context()); err != nil {
		t.Fatal(err)
	}
	first := slices.IndexFunc(f.calls, func(c []string) bool { return !readOnly(c) })
	recheck := slices.IndexFunc(f.calls[f.confirmed:], func(c []string) bool { return c[0] == "lsblk" })
	if recheck < 0 || first < f.confirmed+recheck {
		t.Errorf("consent at %d, recheck at %d, first mutation at %d", f.confirmed, f.confirmed+recheck, first)
	}
	f.before(t, "pacstrap", "umount /mnt/var/cache/pacman/pkg")
	f.before(t, "arch-chroot /mnt useradd", "arch-chroot /mnt env DOTFILES=")
	f.before(t, "arch-chroot /mnt runuser -u cullyn -- git clone", "arch-chroot /mnt env DOTFILES=")
	secureboot := "arch-chroot /mnt env DOTFILES=/home/cullyn/dotfiles /home/cullyn/.local/bin/dctl --json doctor --fix --offline secureboot"
	f.before(t, "arch-chroot /mnt limine-install", secureboot)
	f.before(t, secureboot, "blkid")
	f.before(t, "blkid", "umount -R /mnt")
	if f.index("arch-chroot /mnt limine-update") >= 0 || f.index("arch-chroot /mnt sbctl") >= 0 {
		t.Error("installer signed or built UKIs itself instead of through the secureboot group")
	}
	f.before(t, "umount -R /mnt", "cryptsetup close root")
	f.before(t, "cryptsetup close root", "systemctl reboot")
	if i := slices.IndexFunc(s.times, func(tm iso.Phase) bool { return tm.Name == "keyring" }); i < 0 {
		t.Error("keyring is not a timed phase")
	}
}

func TestFailureReleasesTarget(t *testing.T) {
	for _, fail := range []string{"mount --bind", "pacstrap", "arch-chroot /mnt limine-install"} {
		t.Run(fail, func(t *testing.T) {
			s, f := setup(t)
			f.fail = fail
			if err := s.main(t.Context()); err == nil {
				t.Fatal("install succeeded")
			}
			if entries, _ := os.ReadDir(filepath.Join(s.root, hooks)); len(entries) > 0 {
				t.Errorf("live hooks left masked: %v", entries)
			}
			if fail == "pacstrap" {
				f.before(t, "pacstrap", "umount /mnt/var/cache/pacman/pkg")
			}
			if busy, err := s.busy(); busy || err != nil {
				t.Errorf("target still mounted: %v %v", f.mounts, err)
			}
			if exists(filepath.Join(s.root, "dev", "mapper", mapper)) {
				t.Error("LUKS mapping left open")
			}
			if f.index("systemctl reboot") >= 0 {
				t.Error("rebooted after a failed install")
			}
		})
	}
}

func TestPreexistingTargetRefused(t *testing.T) {
	t.Run("mapper", func(t *testing.T) {
		s, f := setup(t)
		dev := filepath.Join(s.root, "dev", "mapper", mapper)
		put(t, dev, nil, 0o600)
		if err := s.main(t.Context()); err == nil || !strings.Contains(err.Error(), "already open") {
			t.Fatalf("main: %v, want a mapper refusal", err)
		}
		f.mutated(t)
		if !exists(dev) {
			t.Error("closed a mapping the installer did not open")
		}
	})
	t.Run("mount", func(t *testing.T) {
		s, f := setup(t)
		f.mount("/mnt/boot", func(string) bool { return false })
		if err := s.main(t.Context()); err == nil || !strings.Contains(err.Error(), "mounted under") {
			t.Fatalf("main: %v, want a mount refusal", err)
		}
		f.mutated(t)
	})
}

func TestCancelJoinsPrep(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, f := setup(t)
		f.block = true
		s.ask = func(context.Context) (iso.Answers, error) {
			synctest.Wait()
			return iso.Answers{}, ui.ErrCanceled
		}
		if err := s.main(t.Context()); !errors.Is(err, ui.ErrCanceled) {
			t.Fatalf("main: %v, want %v", err, ui.ErrCanceled)
		}
		if !f.drained {
			t.Fatal("main returned before the disk survey worker exited")
		}
	})
}

func TestCancelSignalsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	ready, trapped, child := filepath.Join(dir, "ready"), filepath.Join(dir, "trapped"), filepath.Join(dir, "child")
	script := `sleep 30 & echo $! > "$3"; trap 'echo > "$2"; exit 0' TERM; echo > "$1"; wait`
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- host{ui.New(ui.Options{JSON: true})}.run(ctx, nil, "sh", "-c", script, "sh", ready, trapped, child)
	}()
	for !exists(ready) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if !exists(trapped) {
		t.Error("wrapper TERM trap did not run")
	}
	data, err := os.ReadFile(child)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d survived cancellation", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDCTLTEST(t *testing.T) {
	answer := func(t *testing.T, s *session, body string) {
		put(t, filepath.Join(s.root, testLabel), nil, 0o644)
		put(t, filepath.Join(s.root, testMount, "answers.json"), []byte(body), 0o644)
	}
	valid := func(serial string) string {
		return `{"password":"pw","zone":"America/Denver","luks":"luks","disk_serial":"` + serial + `"}`
	}
	t.Run("outside a VM", func(t *testing.T) {
		s, f := setup(t)
		answer(t, s, valid("24123A800123"))
		if err := s.main(t.Context()); err == nil || !strings.Contains(err.Error(), "not a VM") {
			t.Fatalf("main: %v, want a VM refusal", err)
		}
		if f.index("mount") >= 0 {
			t.Error("mounted the DCTLTEST drive outside a VM")
		}
	})
	for name, body := range map[string]string{"serial mismatch": valid("NOT-THIS-DISK"), "malformed answers": `{"extra":1}`} {
		t.Run(name, func(t *testing.T) {
			s, f := setup(t)
			f.vm = true
			answer(t, s, body)
			if err := s.main(t.Context()); err == nil {
				t.Fatal("main accepted bad DCTLTEST answers")
			}
			f.before(t, "mount -o ro", "umount "+testMount)
			for _, c := range f.calls {
				if !readOnly(c) && strings.Join(c, " ") != "umount "+testMount {
					t.Errorf("ran mutating %q", c)
				}
			}
		})
	}
}

func TestSecondSessionRefused(t *testing.T) {
	s, f := setup(t)
	if err := s.acquire(); err != nil {
		t.Fatal(err)
	}
	defer s.lock.Close()
	if err := s.main(t.Context()); err == nil || !strings.Contains(err.Error(), "another dctl install") {
		t.Fatalf("main: %v, want a lock refusal", err)
	}
	if len(f.calls) > 0 {
		t.Errorf("second session ran %q", f.calls)
	}
}

func TestReleaseSkipsUnacquired(t *testing.T) {
	s, f := setup(t)
	put(t, filepath.Join(s.root, "dev", "mapper", mapper), nil, 0o600)
	f.mount(target, func(string) bool { return false })
	if err := s.release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if f.index("umount") >= 0 || f.index("cryptsetup") >= 0 {
		t.Errorf("released resources this session never acquired: %q", f.calls)
	}
}

func TestOverrideExecsTmpfsCopy(t *testing.T) {
	s, f := setup(t)
	f.vm = true
	put(t, filepath.Join(s.root, testLabel), nil, 0o644)
	put(t, filepath.Join(s.root, testMount, "dctl"), []byte("override"), 0o755)
	var got string
	execve = func(path string, _ []string, _ []string) error {
		got = path
		return errors.New("exec stubbed")
	}
	defer func() { execve = syscall.Exec }()
	if err := s.main(t.Context()); err == nil {
		t.Fatal("main succeeded with a stubbed exec")
	}
	if want := filepath.Join(s.root, overrideCopy); got != want {
		t.Errorf("exec %q, want %q", got, want)
	}
	if data, _ := os.ReadFile(got); string(data) != "override" {
		t.Errorf("copy holds %q", data)
	}
	f.before(t, "mount -o ro", "umount "+testMount)
}

func TestSecureBootFailureFailsInstall(t *testing.T) {
	for name, tt := range map[string]struct {
		mutate func([]doctor.Result) []doctor.Result
		want   string
	}{
		"unsigned": {func(rs []doctor.Result) []doctor.Result {
			rs[1].Status, rs[1].Detail = doctor.Failed, "not signed by the sbctl db key"
			return rs
		}, "secureboot-signed: not signed"},
		"enforcement failed": {func(rs []doctor.Result) []doctor.Result {
			rs[2].Status, rs[2].Detail = doctor.Failed, "efivar SecureBoot: 3 bytes"
			return rs
		}, "secureboot-enforced: efivar"},
		"keys missing":     {func(rs []doctor.Result) []doctor.Result { return rs[1:] }, "no secureboot-keys check"},
		"signed missing":   {func(rs []doctor.Result) []doctor.Result { return slices.Delete(rs, 1, 2) }, "no secureboot-signed check"},
		"enforced missing": {func(rs []doctor.Result) []doctor.Result { return rs[:2] }, "no secureboot-enforced check"},
	} {
		t.Run(name, func(t *testing.T) {
			s, f := setup(t)
			f.sb = tt.mutate(f.sb)
			err := s.main(t.Context())
			if err == nil || !strings.Contains(err.Error(), tt.want) || !errors.Is(err, errModified) {
				t.Fatalf("main: %v, want %q and the modified-disk notice", err, tt.want)
			}
			if f.index("systemctl reboot") >= 0 {
				t.Error("rebooted after a failed Secure Boot step")
			}
		})
	}
}

func TestDoctorUnitOnlyInTestMode(t *testing.T) {
	for _, test := range []bool{false, true} {
		s, f := setup(t)
		s.testMounted = test
		if err := os.MkdirAll(filepath.Join(s.root, target, "etc/systemd/system"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := s.main(t.Context()); err != nil {
			t.Fatal(err)
		}
		unit := filepath.Join(s.root, target, "etc/systemd/system", testUnit)
		data, err := os.ReadFile(unit)
		enabled := f.index("arch-chroot /mnt systemctl enable "+testUnit) >= 0
		if !test {
			if err == nil || enabled {
				t.Errorf("hardware install wrote %s (enabled %v)", testUnit, enabled)
			}
			continue
		}
		if err != nil || !enabled {
			t.Fatalf("test install: unit %v, enabled %v", err, enabled)
		}
		for _, want := range []string{
			"ExecStart=-/home/cullyn/.local/bin/dctl --json doctor --offline system packages secureboot\n",
			"ExecStart=-/usr/bin/runuser -u cullyn -- /home/cullyn/.local/bin/dctl --json doctor --offline home binaries\n",
			`echo "` + iso.Greeter + `$$(systemctl is-active display-manager)"`,
			"TTYPath=/dev/ttyS0\n",
		} {
			if !strings.Contains(string(data), want) {
				t.Errorf("unit lacks %q", want)
			}
		}
		f.before(t, "arch-chroot /mnt systemctl enable "+testUnit, "umount -R /mnt")
	}
}
