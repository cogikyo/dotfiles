package commands

import (
	"fmt"
	"strconv"

	"dotfiles/daemons/hyprd/hypr"
)

// WS implements workspace switching with automatic focus on the leftmost tiled window.
type WS struct {
	hypr  *hypr.Client   // Hyprland IPC client
	state StateManager   // Access to daemon config
}

// NewWS creates a workspace switcher command handler.
func NewWS(h *hypr.Client, s StateManager) *WS {
	return &WS{hypr: h, state: s}
}

// Execute switches to workspace wsArg and focuses its leftmost tiled window if one exists.
// Returns a status message indicating the result.
func (w *WS) Execute(wsArg string) (string, error) {
	ws, err := strconv.Atoi(wsArg)
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", wsArg)
	}

	if err := w.hypr.Dispatch(fmt.Sprintf("workspace %d", ws)); err != nil {
		return "", err
	}

	cfg := w.state.GetConfig()

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
