package session

import (
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
