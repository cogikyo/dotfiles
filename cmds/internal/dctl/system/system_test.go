package system

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/ui"
)

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
}

func TestApplyReplacesAtomically(t *testing.T) {
	src, root := overlay(t), t.TempDir()
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
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestApplyRefusesSymlinkEscape(t *testing.T) {
	src, root, outside := overlay(t), t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "etc")); err != nil {
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
	if _, err := files(src); err == nil {
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

func TestUnreadableTargetIsBlocked(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads through permissions")
	}
	src, root := overlay(t), t.TempDir()
	if err := apply(root, src); err != nil {
		t.Fatal(err)
	}
	usr := filepath.Join(root, "usr")
	if err := os.Chmod(usr, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(usr, 0o755) })
	statuses := func() doctor.Status {
		t.Helper()
		results, err := doctor.Run(context.Background(), ui.New(ui.Options{JSON: true}), []doctor.Group{{
			Name:   "t",
			Checks: []doctor.Check{{Name: "files", Check: func(context.Context) error { return check(root, src) }}},
		}}, doctor.Options{})
		if err != nil || len(results) != 1 {
			t.Fatalf("results %v, err %v", results, err)
		}
		if !strings.Contains(results[0].Detail, "usr/bin/tool (needs root to verify)") {
			t.Errorf("detail %q", results[0].Detail)
		}
		return results[0].Status
	}
	if got := statuses(); got != doctor.Blocked {
		t.Errorf("unreadable only: status %s, want blocked", got)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "a.conf"), []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := statuses(); got != doctor.Failed {
		t.Errorf("unreadable plus drift: status %s, want failed", got)
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
