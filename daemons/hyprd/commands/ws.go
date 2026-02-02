package commands

import (
	"fmt"
	"strconv"

	"dotfiles/daemons/hyprd/hypr"
)

// WS switches workspaces with automatic master window focusing.
type WS struct {
	hypr  *hypr.Client
	state StateManager
}

// NewWS returns a new WS command handler.
func NewWS(h *hypr.Client, s StateManager) *WS {
	return &WS{hypr: h, state: s}
}

// Execute switches to the workspace specified by wsArg and focuses the master window.
func (w *WS) Execute(wsArg string) (string, error) {
	ws, err := strconv.Atoi(wsArg)
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", wsArg)
	}

	// Switch workspace
	if err := w.hypr.Dispatch(fmt.Sprintf("workspace %d", ws)); err != nil {
		return "", err
	}

	cfg := w.state.GetConfig()

	// Focus the master window
	master, err := GetMaster(w.hypr, ws, cfg.Windows.IgnoredClasses)
	if err != nil {
		return fmt.Sprintf("ws %d (no focus)", ws), nil
	}
	if master == nil {
		return fmt.Sprintf("ws %d (empty)", ws), nil
	}

	w.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", master.Address))
	return fmt.Sprintf("ws %d (focused %s)", ws, master.Class), nil
}
