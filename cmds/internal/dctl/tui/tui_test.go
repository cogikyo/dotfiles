package tui

// tui_test.go covers launcher navigation, command selection, and raw-screen formatting.

import (
	"strings"
	"testing"
)

func TestLauncherSelectsNestedCommand(t *testing.T) {
	model := NewLauncher(CommandNode{Children: []CommandNode{
		{Name: "check", Help: "Run healthchecks"},
		{Name: "install", Children: []CommandNode{
			{Name: "link", Help: "Symlink configs"},
			{Name: "go", Help: "Build Go binaries"},
		}},
	}})

	model.cursor[0] = 1
	if done := model.selectCurrent(); done {
		t.Fatal("group selection should not finish")
	}
	if len(model.stack) != 2 {
		t.Fatalf("stack depth = %d, want 2", len(model.stack))
	}

	model.cursor[1] = 1
	if done := model.selectCurrent(); !done {
		t.Fatal("leaf selection should finish")
	}
	want := []string{"install", "go"}
	if len(model.Chosen) != len(want) {
		t.Fatalf("chosen = %#v, want %#v", model.Chosen, want)
	}
	for i := range want {
		if model.Chosen[i] != want[i] {
			t.Fatalf("chosen = %#v, want %#v", model.Chosen, want)
		}
	}
}

func TestLauncherMoveClamps(t *testing.T) {
	model := NewLauncher(CommandNode{Children: []CommandNode{{Name: "check"}, {Name: "install"}}})

	model.move(-1)
	if model.cursor[0] != 0 {
		t.Fatalf("cursor = %d, want 0", model.cursor[0])
	}
	model.move(10)
	if model.cursor[0] != 1 {
		t.Fatalf("cursor = %d, want 1", model.cursor[0])
	}
}

func TestRawScreenUsesCarriageReturns(t *testing.T) {
	got := rawScreen("one\ntwo\n")
	if got != "one\r\ntwo\r\n" {
		t.Fatalf("rawScreen = %q", got)
	}
	if strings.Contains(got, "one\ntwo") {
		t.Fatalf("rawScreen left bare newlines: %q", got)
	}
}
