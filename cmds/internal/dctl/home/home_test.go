package home

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/paths"
)

func setup(t *testing.T) paths.Root {
	t.Helper()
	r := paths.Root{Dotfiles: t.TempDir(), Home: t.TempDir()}
	for _, rel := range []string{
		"config/kitty/kitty.conf",
		"config/firefox/user.js",
		"config/obs-studio/basic/profiles/Costello/basic.ini",
		"config/obs-studio/basic/scenes/Costello.json",
		"config/zsh/zshrc",
		"config/zsh/zshenv",
		"bin/tool",
	} {
		write(t, filepath.Join(r.Dotfiles, rel), rel)
	}
	return r
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func check(t *testing.T, r paths.Root, name string) doctor.Check {
	t.Helper()
	checks := Group(r, nil).Checks
	i := slices.IndexFunc(checks, func(c doctor.Check) bool { return c.Name == name })
	if i < 0 {
		t.Fatalf("no check %s", name)
	}
	return checks[i]
}

func TestLinks(t *testing.T) {
	r := setup(t)
	write(t, filepath.Join(r.Home, ".zshrc"), "old")
	if err := os.MkdirAll(filepath.Join(r.Home, ".config", "zsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.Config("zsh", "zshrc"), filepath.Join(r.Home, ".config", "zsh", "zshrc")); err != nil {
		t.Fatal(err)
	}
	c := check(t, r, "home-links")
	if err := c.Check(t.Context()); err == nil {
		t.Fatal("unlinked home reported clean")
	}
	if err := c.Fix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(t.Context()); err != nil {
		t.Fatalf("after fix: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(r.Home, ".config", "firefox")); err == nil {
		t.Error("firefox linked wholesale")
	}
	for _, dst := range []string{".config/kitty", ".config/obs-studio/basic/profiles/Costello/basic.ini", ".zshenv", ".local/bin/tool"} {
		if st, err := os.Lstat(filepath.Join(r.Home, dst)); err != nil || st.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s not a symlink: %v", dst, err)
		}
	}
}

func TestApplyBackup(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	write(t, src, "new")
	write(t, dst, "old")
	if err := (link{src, dst}).apply(dir, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst + ".backup.20260102-030405"); string(data) != "old" {
		t.Errorf("existing file not backed up: %q", data)
	}
	if data, _ := os.ReadFile(dst); string(data) != "new" {
		t.Errorf("dst reads %q after apply", data)
	}
}

func TestLinksRefuseUserData(t *testing.T) {
	r := setup(t)
	write(t, filepath.Join(r.Home, ".config", "kitty", "mine.conf"), "user data")
	c := check(t, r, "home-links")
	err := c.Fix(t.Context())
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("err %v, want refusal", err)
	}
	if data, _ := os.ReadFile(filepath.Join(r.Home, ".config", "kitty", "mine.conf")); string(data) != "user data" {
		t.Error("user data touched")
	}
	if c.Check(t.Context()) == nil {
		t.Error("refused link reported clean")
	}
}

func TestSeed(t *testing.T) {
	r := setup(t)
	src := r.Config("obs-studio", "basic", "scenes", "Costello.json")
	dst := filepath.Join(r.Home, "scene.json")
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}
	if localFile(dst) == nil {
		t.Fatal("symlinked seed reported clean")
	}
	if err := seed(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := localFile(dst); err != nil {
		t.Fatal(err)
	}
	write(t, dst, "edited")
	if err := seed(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "edited" {
		t.Error("existing file overwritten")
	}
	other := filepath.Join(r.Home, "zoomus.conf")
	if err := os.Symlink(filepath.Join(r.Home, "elsewhere"), other); err != nil {
		t.Fatal(err)
	}
	if seed(src, other) == nil {
		t.Error("foreign symlink replaced")
	}
}
