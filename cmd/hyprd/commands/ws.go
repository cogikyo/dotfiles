package commands

// ================================================================================
// Workspace switching with automatic master focus
// ================================================================================

import (
	"fmt"
	"strconv"

	"dotfiles/cmd/hyprd/hypr"
)

// WS handles the workspace switch command.
type WS struct {
	hypr *hypr.Client
}

// NewWS creates a workspace command handler.
func NewWS(h *hypr.Client) *WS {
	return &WS{hypr: h}
}

// Execute switches to the specified workspace and focuses the master window.
func (w *WS) Execute(wsArg string) (string, error) {
	ws, err := strconv.Atoi(wsArg)
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", wsArg)
	}

	// Switch workspace
	if err := w.hypr.Dispatch(fmt.Sprintf("workspace %d", ws)); err != nil {
		return "", err
	}

	// Focus the master window
	master, err := GetMaster(w.hypr, ws)
	if err != nil {
		return fmt.Sprintf("ws %d (no focus)", ws), nil
	}
	if master == nil {
		return fmt.Sprintf("ws %d (empty)", ws), nil
	}

	w.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", master.Address))
	return fmt.Sprintf("ws %d (focused %s)", ws, master.Class), nil
}
