package home

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestLinks(t *testing.T) {
	r := setup(t)
	write(t, filepath.Join(r.Home, ".zshrc"), "old")
	if err := os.MkdirAll(filepath.Join(r.Home, ".config", "zsh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.Config("zsh", "zshrc"), filepath.Join(r.Home, ".config", "zsh", "zshrc")); err != nil {
		t.Fatal(err)
	}
	if err := checkLinks(r); err == nil {
		t.Fatal("unlinked home reported clean")
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := fixLinks(r, now); err != nil {
		t.Fatal(err)
	}
	if err := checkLinks(r); err != nil {
		t.Fatalf("after fix: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(r.Home, ".zshrc.backup.20260102-030405")); string(data) != "old" {
		t.Errorf("existing file not backed up: %q", data)
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

func TestLinksRefuseUserData(t *testing.T) {
	r := setup(t)
	write(t, filepath.Join(r.Home, ".config", "kitty", "mine.conf"), "user data")
	err := fixLinks(r, time.Now())
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("err %v, want refusal", err)
	}
	if data, _ := os.ReadFile(filepath.Join(r.Home, ".config", "kitty", "mine.conf")); string(data) != "user data" {
		t.Error("user data touched")
	}
	if checkLinks(r) == nil {
		t.Error("refused link reported clean")
	}
}

func TestSceneSeed(t *testing.T) {
	r := setup(t)
	src := r.Config("obs-studio", "basic", "scenes", "Costello.json")
	dst := filepath.Join(r.Home, ".config", "obs-studio", "basic", "scenes", "Costello.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}
	if checkScene(dst) == nil {
		t.Fatal("symlinked scene reported clean")
	}
	if err := seedScene(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := checkScene(dst); err != nil {
		t.Fatal(err)
	}
	write(t, dst, "edited")
	if err := seedScene(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "edited" {
		t.Error("existing scene overwritten")
	}
}
