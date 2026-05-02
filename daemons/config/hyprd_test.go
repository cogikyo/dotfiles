package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHyprSessionsDeriveNameAndWorkspace(t *testing.T) {
	var cfg HyprConfig
	data := `
sessions:
  2:
    slack:
      command: slack
  4:
    leadpier:
      project: LeadPier
      body: [editor, browser, agents]
`

	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal hypr config: %v", err)
	}

	if got := cfg.Sessions["slack"]; got.Name != "slack" || got.Workspace != 2 || got.Command != "slack" {
		t.Fatalf("slack session = %#v, want derived name/workspace and command", got)
	}
	if got := cfg.Sessions["leadpier"]; got.Name != "leadpier" || got.Workspace != 4 || got.Project != "LeadPier" {
		t.Fatalf("leadpier session = %#v, want derived name/workspace and project", got)
	}
}

func TestHyprSessionsInitField(t *testing.T) {
	var cfg HyprConfig
	data := `
sessions:
  2:
    slack:
      init: true
      command: slack
    discord:
      command: discord
  5:
    dotfiles:
      init: true
      project: dotfiles
`

	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !cfg.Sessions["slack"].Init {
		t.Fatal("slack.Init should be true")
	}
	if cfg.Sessions["discord"].Init {
		t.Fatal("discord.Init should be false")
	}
	if got := cfg.Sessions.DefaultSession(2); got != "slack" {
		t.Fatalf("DefaultSession(2) = %q, want slack", got)
	}
	if got := cfg.Sessions.DefaultSession(3); got != "" {
		t.Fatalf("DefaultSession(3) = %q, want empty", got)
	}
}

func TestHyprBrowserSnapshotShorthand(t *testing.T) {
	var cfg HyprConfig
	data := `
sessions:
  2:
    slack:
      command: slack
      browser: coms
`

	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := cfg.Sessions["slack"].Browser.Snapshot; got != "coms" {
		t.Fatalf("browser snapshot = %q, want coms", got)
	}
}

func TestHyprSessionsRejectMultipleInit(t *testing.T) {
	var cfg HyprConfig
	data := `
sessions:
  2:
    slack:
      init: true
      command: slack
    discord:
      init: true
      command: discord
`

	err := yaml.Unmarshal([]byte(data), &cfg)
	if err == nil {
		t.Fatal("expected multiple init error")
	}
	if !strings.Contains(err.Error(), "multiple init sessions") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHyprSessionsRejectDuplicateNames(t *testing.T) {
	var cfg HyprConfig
	data := `
sessions:
  2:
    shared:
      command: one
  3:
    shared:
      command: two
`

	err := yaml.Unmarshal([]byte(data), &cfg)
	if err == nil {
		t.Fatal("expected duplicate session name error")
	}
	if !strings.Contains(err.Error(), `duplicate session name "shared"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
