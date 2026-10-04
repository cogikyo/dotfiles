package cli

import (
	"slices"
	"testing"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/ui"

	"github.com/alecthomas/kong"
)

func TestCatalog(t *testing.T) {
	var names, root []string
	for _, s := range catalog(nil, paths.Root{}) {
		names = append(names, s.Name)
		if s.Root {
			root = append(root, s.Name)
		}
	}
	if want := []string{"system", "packages", "home", "extra", "secrets", "repos", "firefox", "certs", "vpn", "tailscale", "keys", "secureboot"}; !slices.Equal(names, want) {
		t.Errorf("stages %v, want %v", names, want)
	}
	if want := []string{"system", "packages", "tailscale", "keys", "secureboot"}; !slices.Equal(root, want) {
		t.Errorf("root stages %v, want %v", root, want)
	}
}

func TestMode(t *testing.T) {
	for _, tc := range []struct {
		cmd  SetupCmd
		yes  bool
		want setup.Mode
	}{
		{SetupCmd{}, false, setup.Ask},
		{SetupCmd{All: true}, false, setup.All},
		{SetupCmd{}, true, setup.All},
		{SetupCmd{Stages: []string{"home"}}, true, setup.Force},
		{SetupCmd{Status: true, Stages: []string{"home"}}, false, setup.Status},
		{SetupCmd{Batch: setup.Force}, false, setup.Force},
	} {
		got, err := tc.cmd.mode(ui.New(ui.Options{Yes: tc.yes}))
		if err != nil || got != tc.want {
			t.Errorf("%+v yes=%v: %s %v, want %s", tc.cmd, tc.yes, got, err, tc.want)
		}
	}
	if _, err := (&SetupCmd{Batch: "fix"}).mode(ui.New(ui.Options{})); err == nil {
		t.Error("unknown batch mode accepted")
	}
}

func TestBatch(t *testing.T) {
	stages := []setup.Stage{{Name: "system", Root: true}, {Name: "keys", Root: true}}
	argv := batch("/home/cullyn/.local/bin/dctl", "/home/cullyn/dotfiles", globals(ui.New(ui.Options{JSON: true, Yes: true})), setup.All, stages)
	want := []string{"sudo", "env", "DOTFILES=/home/cullyn/dotfiles", "/home/cullyn/.local/bin/dctl", "--json", "--yes", "setup", "--batch=all", "system", "keys"}
	if !slices.Equal(argv, want) {
		t.Fatalf("argv %q, want %q", argv, want)
	}
	var c CLI
	parser, err := kong.New(&c)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := parser.Parse(argv[4:])
	if err != nil {
		t.Fatal(err)
	}
	mode, err := c.Setup.mode(ui.New(ui.Options{}))
	if ctx.Command() != "setup <stages>" || !c.JSON || !c.Yes || mode != setup.All || !slices.Equal(c.Setup.Stages, []string{"system", "keys"}) {
		t.Fatalf("child parsed %q: %+v, mode %s %v", ctx.Command(), c, mode, err)
	}
}
