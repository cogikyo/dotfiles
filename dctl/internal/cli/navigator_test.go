package cli

import (
	"testing"

	"dotfiles/dctl/internal/tui"

	"github.com/alecthomas/kong"
)

func TestBuildCommandCatalogGroupsCommands(t *testing.T) {
	var root CLI
	parser, err := kong.New(&root, kong.Name("dctl"))
	if err != nil {
		t.Fatal(err)
	}

	catalog := BuildCommandCatalog(parser)
	if len(catalog.Children) == 0 {
		t.Fatal("expected catalog children")
	}

	update := childNamed(catalog.Children, "update")
	if update == nil {
		t.Fatal("expected update command")
	}
	if !update.IsGroup() {
		t.Fatal("expected update to be a command group")
	}
	if update.GroupKey != "lifecycle" {
		t.Fatalf("update group = %q, want lifecycle", update.GroupKey)
	}
	if childNamed(update.Children, "run") == nil {
		t.Fatal("expected update run child")
	}

	if childNamed(catalog.Children, "help") != nil || childNamed(catalog.Children, "?") != nil {
		t.Fatal("catalog should not include Kong help meta-commands")
	}
}

func TestShouldLaunchNavigatorOnlyForBareTTYShape(t *testing.T) {
	if ShouldLaunchNavigator([]string{"check"}) {
		t.Fatal("explicit commands must not launch navigator")
	}
	if ShouldLaunchNavigator([]string{"--plain"}) {
		t.Fatal("explicit flags must not launch navigator")
	}
}

func TestNormalizeArgsSupportsPlainAlias(t *testing.T) {
	got := NormalizeArgs([]string{"plain", "install", "list"})
	want := []string{"--plain", "install", "list"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeArgs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeArgs = %#v, want %#v", got, want)
		}
	}
}

func TestNormalizeArgsLeavesCommandsAlone(t *testing.T) {
	got := NormalizeArgs([]string{"check"})
	if len(got) != 1 || got[0] != "check" {
		t.Fatalf("NormalizeArgs changed command: %#v", got)
	}
}

func childNamed(children []tui.CommandNode, name string) *tui.CommandNode {
	for i := range children {
		if children[i].Name == name {
			return &children[i]
		}
	}
	return nil
}
