// Package wm implements hyprd window-management commands backed by Hyprland IPC and shared state.
package wm

import (
	"fmt"
	"strconv"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

const (
	chatWorkspace  = 1
	musicWorkspace = 5
	lastWorkspace  = 6
)

// WS dispatches workspace switches (numeric) and window moves ("up"/"down") across managed workspaces.
type WS struct {
	hypr  *hypr.Client
	state *state.State
}

func NewWS(h *hypr.Client, s *state.State) *WS {
	return &WS{hypr: h, state: s}
}

// Execute accepts "up", "down", or a workspace ID.
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
	if err := w.hypr.FocusWorkspace(ws); err != nil {
		return "", err
	}
	return fmt.Sprintf("ws %d", ws), nil
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
	targetWS := carryTarget(currentWS, delta)
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

	if err := w.hypr.MoveActiveToWorkspace(targetWS, true); err != nil {
		return "", err
	}
	return fmt.Sprintf("moved %s: ws %d -> %d", win.Class, currentWS, targetWS), nil
}

// normalizeWorkspaceState unwinds three-body and displaced-master state before a cross-workspace move.
func (w *WS) normalizeWorkspaceState(wsID int) error {
	if tb := w.state.GetThreeBody(wsID); tb != nil {
		if err := w.hypr.MoveWindowToWorkspace(tb.Shadow, strconv.Itoa(wsID), false); err != nil {
			return fmt.Errorf("restore three-body shadow on ws %d: %w", wsID, err)
		}
		w.state.ClearThreeBody(wsID)
	}

	w.state.SetDisplacedMaster(wsID, "")
	return nil
}

func carryTarget(ws, delta int) int {
	next := ws + delta
	if next == musicWorkspace {
		next += delta
	}
	if next < 1 || next > lastWorkspace {
		return ws
	}
	return next
}
