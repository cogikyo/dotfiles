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
	stages := catalog(nil, paths.Root{})
	var names, root []string
	for i, s := range stages {
		names = append(names, s.Name)
		if s.Root {
			root = append(root, s.Name)
		}
		for _, need := range s.Needs {
			if j := slices.IndexFunc(stages, func(t setup.Stage) bool { return t.Name == need }); j < 0 || j >= i {
				t.Errorf("%s needs %s, which is not an earlier stage", s.Name, need)
			}
		}
	}
	if want := []string{"system", "network", "packages", "home", "extra", "secrets", "ssh", "repos", "firefox", "certs", "tailscale", "vpn", "luks", "secureboot", "totp"}; !slices.Equal(names, want) {
		t.Errorf("stages %v, want %v", names, want)
	}
	if want := []string{"system", "packages", "tailscale", "luks", "secureboot", "totp"}; !slices.Equal(root, want) {
		t.Errorf("root stages %v, want %v", root, want)
	}
}

func TestBatch(t *testing.T) {
	stages := []setup.Stage{{Name: "system", Root: true}, {Name: "keys", Root: true}}
	argv := batch("/home/cullyn/.local/bin/dctl", "/home/cullyn/dotfiles", nil, globals(ui.New(ui.Options{JSON: true, Yes: true})), setup.All, "/tmp/dctl-setup-1.json", stages)
	want := []string{"sudo", "env", "DOTFILES=/home/cullyn/dotfiles", "/home/cullyn/.local/bin/dctl", "--json", "--yes", "setup", "--batch=all", "--report=/tmp/dctl-setup-1.json", "system", "keys"}
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
	if ctx.Command() != "setup <stages>" || !c.JSON || !c.Yes || c.Setup.Batch != setup.All || c.Setup.Report != "/tmp/dctl-setup-1.json" || !slices.Equal(c.Setup.Stages, []string{"system", "keys"}) {
		t.Fatalf("child parsed %q: %+v", ctx.Command(), c)
	}
}
