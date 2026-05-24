package session

import (
	"strings"
	"testing"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

func TestShadowEditorForWorkspaceRequiresStoredShadowAddress(t *testing.T) {
	clients := []hypr.Window{
		{
			Address:      "other",
			Class:        "kitty",
			InitialTitle: "editor",
			Workspace:    hypr.WsRef{Name: windows.ShadowWorkspace},
		},
	}

	if got := shadowEditorForWorkspace(clients, &state.ThreeBodyState{Shadow: "owned"}); got != nil {
		t.Fatalf("shadowEditorForWorkspace returned %q, want nil for another workspace's shadow", got.Address)
	}

	clients = append(clients, hypr.Window{
		Address:      "owned",
		Class:        "kitty",
		InitialTitle: "editor",
		Workspace:    hypr.WsRef{Name: windows.ShadowWorkspace},
	})

	got := shadowEditorForWorkspace(clients, &state.ThreeBodyState{Shadow: "owned"})
	if got == nil || got.Address != "owned" {
		t.Fatalf("shadowEditorForWorkspace returned %+v, want owned shadow editor", got)
	}
}

func TestLuaQuoteEscapesNvimPath(t *testing.T) {
	got := luaQuote(`/tmp/a "quoted" path\name.md` + "\nnext")
	want := `"/tmp/a \"quoted\" path\\name.md\nnext"`
	if got != want {
		t.Fatalf("luaQuote() = %q, want %q", got, want)
	}
}

func TestNvimOpenFileUsesEscapedEditCommand(t *testing.T) {
	got := nvimOpenFile(`/tmp/a "quoted" path.md`)

	for _, want := range []string{
		"\x1b:lua local p=",
		`/tmp/a \"quoted\" path.md`,
		`vim.cmd("edit "..vim.fn.fnameescape(p))`,
		"\r\x0c",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("nvimOpenFile() missing %q in %q", want, got)
		}
	}
}
