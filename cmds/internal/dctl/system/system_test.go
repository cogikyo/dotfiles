package system

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/machine"
)

func record(t *testing.T, root string) {
	t.Helper()
	p := machine.Profile{Preset: machine.Custom, CPU: machine.AMD, Form: machine.Desktop, WLAN: []string{"mt7925e"}, YubiKeys: 2, Lane: []int{3, 7}}
	if err := machine.Write(filepath.Join(root, machine.Path), p); err != nil {
		t.Fatal(err)
	}
}

func overlay(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, mode := range map[string]os.FileMode{"etc/a.conf": 0o600, "usr/bin/tool": 0o700} {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), mode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	st, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st.Mode()
}

func TestApply(t *testing.T) {
	src, root := overlay(t), t.TempDir()
	record(t, root)
	if err := check(root, src); err == nil {
		t.Fatal("empty root reported clean")
	}
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "etc", "a.conf")); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	if err := check(root, src); err != nil {
		t.Fatalf("after apply: %v", err)
	}
	if got := mode(t, filepath.Join(root, "etc", "a.conf")); got != 0o644 {
		t.Errorf("etc/a.conf mode %v, want regular 0644", got)
	}
	if got := mode(t, filepath.Join(root, "usr", "bin", "tool")); got != 0o755 {
		t.Errorf("usr/bin/tool mode %v, want regular 0755", got)
	}
	if data, _ := os.ReadFile(outside); string(data) != "keep" {
		t.Errorf("leaf symlink target was written through: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "etc", "modprobe.d", "blacklist-wifi.conf")); string(data) != "blacklist mt7925e\n" {
		t.Errorf("rendered blacklist %q", data)
	}
	p, err := machine.Read(filepath.Join(root, machine.Path))
	if err != nil {
		t.Fatal(err)
	}
	p.WiFi = true
	if err := machine.Write(filepath.Join(root, machine.Path), p); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "etc", "modprobe.d", "blacklist-wifi.conf")); !os.IsNotExist(err) {
		t.Errorf("blacklist kept with Wi-Fi on: %v", err)
	}
}

func TestApplyReplacesAtomically(t *testing.T) {
	src, root := overlay(t), t.TempDir()
	record(t, root)
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "etc", "a.conf")
	before := mode(t, target)
	old, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "etc", "a.conf"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	now, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(old, now) {
		t.Error("file rewritten in place, want rename over target")
	}
	if data, _ := os.ReadFile(target); string(data) != "new" || mode(t, target) != before {
		t.Errorf("content %q mode %v", data, mode(t, target))
	}
	entries, _ := os.ReadDir(filepath.Join(root, "etc"))
	if slices.ContainsFunc(entries, func(e os.DirEntry) bool { return strings.HasPrefix(e.Name(), ".") }) {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestApplyRefusesSymlinkEscape(t *testing.T) {
	src, root, outside := overlay(t), t.TempDir(), t.TempDir()
	record(t, root)
	if err := os.Symlink(outside, filepath.Join(root, "usr")); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, src); err == nil {
		t.Fatal("apply followed a symlink out of the root")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("wrote outside the root: %v", entries)
	}
}

func TestOverlayRejectsSymlinks(t *testing.T) {
	src := overlay(t)
	if err := os.Symlink("a.conf", filepath.Join(src, "etc", "b.conf")); err != nil {
		t.Fatal(err)
	}
	if _, err := files(t.TempDir(), src); err == nil {
		t.Fatal("overlay symlink accepted")
	}
}

func TestParsePreset(t *testing.T) {
	got, err := parsePreset(strings.NewReader("# units\n\nenable a.service\n; note\n  enable b.socket  \n"))
	if err != nil || !slices.Equal(got, []string{"a.service", "b.socket"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []string{"disable a.service", "enable *.service", "enable", "enable a b"} {
		if _, err := parsePreset(strings.NewReader(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestUnreadableTarget(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads through permissions")
	}
	src, root := overlay(t), t.TempDir()
	record(t, root)
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	usr := filepath.Join(root, "usr")
	if err := os.Chmod(usr, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(usr, 0o755) })
	if err := check(root, src); err == nil || !strings.Contains(err.Error(), "cannot inspect 1 of 11 files") {
		t.Errorf("unreadable only: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "a.conf"), []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := check(root, src); err == nil || !strings.Contains(err.Error(), "1 of 11 files differ") {
		t.Errorf("unreadable plus drift: %v", err)
	}
}

func TestResolv(t *testing.T) {
	root := t.TempDir()
	if err := checkResolv(root); err == nil {
		t.Fatal("missing resolv.conf passed")
	}
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "resolv.conf"), []byte("nameserver 1.1.1.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkResolv(root); err == nil || !strings.Contains(err.Error(), "restart NetworkManager") {
		t.Fatalf("regular file: %v", err)
	}
	if err := linkResolv(root); err != nil {
		t.Fatal(err)
	}
	if err := checkResolv(root); err != nil {
		t.Fatalf("after link: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "etc", "resolv.conf")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/run/NetworkManager/resolv.conf", filepath.Join(root, "etc", "resolv.conf")); err != nil {
		t.Fatal(err)
	}
	if err := checkResolv(root); err == nil {
		t.Fatal("wrong symlink target passed")
	}
}

func TestParseSysusers(t *testing.T) {
	got, err := parseSysusers(strings.NewReader("# users\n\nu tunnel - \"SSH reverse tunnel\" / /usr/bin/nologin\nu! locked -\ng group -\nm tunnel group\n#u commented -\n"))
	if err != nil || !slices.Equal(got, []string{"tunnel", "locked"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestUsers(t *testing.T) {
	if _, err := exec.LookPath("systemd-sysusers"); err != nil {
		t.Skip("systemd-sysusers not installed")
	}
	src, root := overlay(t), t.TempDir()
	record(t, root)
	conf := filepath.Join(src, sysusers, "tunnel.conf")
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, []byte("u tunnel - \"SSH reverse tunnel\" / /usr/bin/nologin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	if err := checkUsers(root, src); err == nil || !strings.Contains(err.Error(), "tunnel") {
		t.Fatalf("before sysusers: %v", err)
	}
	if err := addUsers(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	if err := checkUsers(root, src); err != nil {
		t.Fatalf("after sysusers: %v", err)
	}
	passwd := filepath.Join(root, "etc", "passwd")
	before, err := os.ReadFile(passwd)
	if err != nil {
		t.Fatal(err)
	}
	if err := addUsers(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(passwd); !bytes.Equal(before, after) {
		t.Errorf("second run changed passwd:\n%s\n%s", before, after)
	}
}

type systemctl struct {
	active bool
	calls  []string
}

func (s *systemctl) Run(ctx context.Context, dir, name string, args ...string) error {
	_, err := s.Output(ctx, dir, name, args...)
	return err
}

func (s *systemctl) Output(_ context.Context, _, name string, args ...string) (string, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	s.calls = append(s.calls, cmd)
	switch {
	case cmd == "systemctl start sshd.service":
		s.active = true
	case cmd == "systemctl is-active sshd.service" && !s.active:
		return "inactive", errors.New("exit status 3")
	case cmd == "systemctl is-active sshd.service":
		return "active", nil
	}
	return "", nil
}

func TestRunning(t *testing.T) {
	s := &systemctl{}
	if err := checkRunning(t.Context(), s); err == nil || !strings.Contains(err.Error(), "sshd.service (inactive)") {
		t.Fatalf("inactive: %v", err)
	}
	if err := start(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	if err := checkRunning(t.Context(), s); err != nil {
		t.Fatalf("after start: %v", err)
	}
	want := []string{"systemctl is-active sshd.service", "systemctl start sshd.service", "systemctl is-active sshd.service"}
	if !slices.Equal(s.calls, want) {
		t.Errorf("calls %q, want %q", s.calls, want)
	}
}
