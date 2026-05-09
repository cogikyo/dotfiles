package session

import (
	"os"
	"path/filepath"
	"testing"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"

	"gopkg.in/yaml.v3"
)

func TestPreserveSessionBrowserWindowMatchesSnapshotTitle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	snapshotDir := filepath.Join(home, "dotfiles", "cmds", "internal", "hyprd", "browser", "sessions", "coms")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot: %v", err)
	}
	data, err := yaml.Marshal(map[string]any{
		"window": map[string]any{
			"selected_tab":   1,
			"selected_title": "Slack",
		},
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "snapshot.yaml"), data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	s := config.Session{Browser: config.BrowserConfig{Snapshot: "coms"}}
	window := hypr.Window{Class: "firefox-developer-edition", Title: "Slack — Firefox Developer Edition"}
	if !preserveSessionBrowserWindow(s, window) {
		t.Fatalf("matching restored Firefox window should be preserved during cleanup")
	}

	window.Title = "Other — Firefox Developer Edition"
	if !preserveSessionBrowserWindow(s, window) {
		t.Fatalf("all Firefox windows should be preserved during shared-profile cleanup")
	}

	window.Class = "kitty"
	window.Title = "Slack — Firefox Developer Edition"
	if preserveSessionBrowserWindow(s, window) {
		t.Fatalf("non-Firefox window should not be preserved")
	}
}
