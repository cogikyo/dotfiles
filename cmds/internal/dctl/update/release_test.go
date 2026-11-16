package update

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"dotfiles/cmds/internal/ui"
)

const checked = `[
  {
    "pkgbase": "snapcast",
    "status": "success",
    "message": null,
    "local_version": "0.34.0",
    "upstream_version": "0.35.0",
    "out_of_date": true
  },
  {
    "pkgbase": "eww",
    "status": "warning",
    "message": "local version is newer than upstream",
    "local_version": "0.6.0.r91.g48f5aa8",
    "upstream_version": "0.6.0",
    "out_of_date": false
  },
  {
    "pkgbase": "catppuccin-macchiato-light-cursors",
    "status": "success",
    "message": null,
    "local_version": "2.0.0",
    "upstream_version": "2.0.0",
    "out_of_date": false
  },
  {
    "pkgbase": "offline",
    "status": "failure",
    "message": "command exited with error: fatal: unable to access 'https://127.0.0.1:9/x.git/': Failed to connect to 127.0.0.1:9 after 0 ms: Could not connect to server\nno-result: GetVersionError('command exited with error')",
    "local_version": "0.35.0",
    "upstream_version": null,
    "out_of_date": false
  }
]`

func TestReleases(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	tool := func(name string) {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dirs := []string{"/p/snapcast", "/p/eww", "/p/catppuccin-macchiato-light-cursors", "/p/offline"}
	f := &fake{t: t, release: checked}
	for _, c := range []struct {
		tool string
		want []row
	}{
		{"", []row{{ui.Info, "release check skipped: pkgctl is not installed"}}},
		{"pkgctl", []row{{ui.Info, "release check skipped: nvchecker is not installed"}}},
		{"nvchecker", []row{
			{ui.Warn, "snapcast 0.35.0 released upstream; recipe pins 0.34.0"},
			{ui.Info, "offline release check failed: command exited with error: fatal: unable to access 'https://127.0.0.1:9/x.git/': Failed to connect to 127.0.0.1:9 after 0 ms: Could not connect to server"},
			{ui.OK, "no newer upstream release: eww, catppuccin-macchiato-light-cursors"},
		}},
	} {
		if c.tool != "" {
			tool(c.tool)
		}
		f.calls = nil
		if got := releases(t.Context(), f, dirs); !slices.Equal(got, c.want) {
			t.Errorf("after installing %q: rows %v, want %v", c.tool, got, c.want)
		}
	}
	if want := "env GIT_TERMINAL_PROMPT=0 pkgctl version check --json -v /p/snapcast /p/eww /p/catppuccin-macchiato-light-cursors /p/offline"; !slices.Equal(f.calls, []string{want}) {
		t.Errorf("calls %q, want one %q", f.calls, want)
	}
	if got := releases(t.Context(), f, nil); got != nil {
		t.Errorf("no watched recipes: rows %v", got)
	}
}
