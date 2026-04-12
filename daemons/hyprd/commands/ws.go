package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"dotfiles/daemons/hyprd/hypr"
)

// WS implements workspace switching with spatial animations and automatic master focus.
type WS struct {
	hypr  *hypr.Client   // Hyprland IPC client
	state StateManager   // Access to daemon config
}

// NewWS creates a workspace switcher command handler.
func NewWS(h *hypr.Client, s StateManager) *WS {
	return &WS{hypr: h, state: s}
}

// Execute switches to workspace wsArg with spatial animation direction
// and focuses its leftmost tiled window if one exists.
//
// Workspace spatial layout:
//
//	       1
//	    2  3  4
//	       5
//
// WS 1 and 5 use vertical slide; others use horizontal.
func (w *WS) Execute(wsArg string) (string, error) {
	ws, err := strconv.Atoi(wsArg)
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", wsArg)
	}

	// Get current workspace for animation direction
	currentWS := 0
	if data, err := w.hypr.Request("j/activeworkspace"); err == nil {
		var active struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(data, &active) == nil {
			currentWS = active.ID
		}
	}

	// Set animation direction based on spatial layout
	anim := "slide"
	if ws == 1 || ws == 5 || currentWS == 1 || currentWS == 5 {
		anim = "slidevert"
	}

	w.hypr.Request(fmt.Sprintf("keyword animation workspaces, 1, 3, default, %s", anim))

	if err := w.hypr.Dispatch(fmt.Sprintf("workspace %d", ws)); err != nil {
		return "", err
	}

	// Ensure wallpaper is running
	cfg := w.state.GetConfig()
	go EnsureBG(&cfg.Background)

	// Reset animation to default
	w.hypr.Request("keyword animation workspaces, 1, 3, default, slide")

	// Focus master window
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
