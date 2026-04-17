package wm

import (
	"encoding/json"
	"fmt"
	"strconv"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/session"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

const (
	minManagedWorkspace = 2
	maxManagedWorkspace = 5
)

// WS dispatches workspace navigation and per-workspace window moves.
//
// "up"/"down" shift the active window between managed workspaces, clamped to [min,max]ManagedWorkspace.
// A numeric arg switches the active workspace and focuses its master.
type WS struct {
	hypr  *hypr.Client
	state *state.State
}

func NewWS(h *hypr.Client, s *state.State) *WS {
	return &WS{hypr: h, state: s}
}

// Execute runs a workspace command.
//
// wsArg accepts "up", "down", or a workspace ID as a decimal string.
// Switching across the 1↔5 boundary animates as `slidevert` to match the virtual row/column layout.
// Triggers a background refresh via session.EnsureBG on switch.
func (w *WS) Execute(wsArg string) (string, error) {
	switch wsArg {
	case "up":
		return w.moveActiveWindow(1)
	case "down":
		return w.moveActiveWindow(-1)
	}

	ws, err := strconv.Atoi(wsArg)
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %s", wsArg)
	}

	currentWS := 0
	if data, err := w.hypr.Request("j/activeworkspace"); err == nil {
		var active struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(data, &active) == nil {
			currentWS = active.ID
		}
	}

	anim := "slide"
	if ws == 1 || ws == 5 || currentWS == 1 || currentWS == 5 {
		anim = "slidevert"
	}
	w.hypr.Request(fmt.Sprintf("keyword animation workspaces, 1, 3, default, %s", anim))

	if err := w.hypr.Dispatch(fmt.Sprintf("workspace %d", ws)); err != nil {
		return "", err
	}

	cfg := w.state.GetConfig()
	go session.EnsureBG(&cfg.Background)

	w.hypr.Request("keyword animation workspaces, 1, 3, default, slide")

	master, err := windows.GetMaster(w.hypr, ws, cfg.Windows.IgnoredClasses)
	if err != nil {
		return fmt.Sprintf("ws %d (no focus)", ws), nil
	}
	if master == nil {
		return fmt.Sprintf("ws %d (empty)", ws), nil
	}

	w.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", master.Address))
	return fmt.Sprintf("ws %d (focused %s)", ws, master.Class), nil
}

func (w *WS) moveActiveWindow(delta int) (string, error) {
	win, err := w.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	currentWS := win.Workspace.ID
	targetWS := clampWorkspace(currentWS + delta)
	if targetWS == currentWS {
		return fmt.Sprintf("window already at ws %d bound", currentWS), nil
	}

	if w.state.GetMonocle(currentWS) != nil {
		return fmt.Sprintf("monocle active on ws %d: toggle it off first", currentWS), nil
	}
	if w.state.GetMonocle(targetWS) != nil {
		return fmt.Sprintf("monocle active on ws %d: move blocked", targetWS), nil
	}

	if err := w.normalizeWorkspaceState(currentWS); err != nil {
		return "", err
	}
	if targetWS != currentWS {
		if err := w.normalizeWorkspaceState(targetWS); err != nil {
			return "", err
		}
	}

	if err := w.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d", targetWS)); err != nil {
		return "", err
	}
	return fmt.Sprintf("moved %s: ws %d -> %d", win.Class, currentWS, targetWS), nil
}

// normalizeWorkspaceState unwinds three-body and displaced-master state before a cross-workspace move.
func (w *WS) normalizeWorkspaceState(wsID int) error {
	if tb := w.state.GetThreeBody(wsID); tb != nil {
		if err := w.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, tb.Shadow)); err != nil {
			return fmt.Errorf("restore three-body shadow on ws %d: %w", wsID, err)
		}
		w.state.ClearThreeBody(wsID)
	}

	w.state.SetDisplacedMaster(wsID, "")
	return nil
}

func clampWorkspace(ws int) int {
	if ws < minManagedWorkspace {
		return minManagedWorkspace
	}
	if ws > maxManagedWorkspace {
		return maxManagedWorkspace
	}
	return ws
}
