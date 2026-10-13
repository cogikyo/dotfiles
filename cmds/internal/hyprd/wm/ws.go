// Package wm implements hyprd window-management commands backed by Hyprland IPC and shared state.
package wm

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

const (
	chatWorkspace  = 1
	musicWorkspace = 6
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
	if ws == chatWorkspace {
		return w.enterChat()
	}
	if err := w.hypr.FocusWorkspace(ws); err != nil {
		return "", err
	}
	return fmt.Sprintf("ws %d", ws), nil
}

// chatUrgency orders the chat bodies whose unseen notifications a switch to the chat workspace focuses.
var chatUrgency = []string{"editor", "agents", "browser"}

// enterChat switches to the chat workspace and focuses the most urgent body with unseen notifications.
//
// The arrival is recorded before any focus so the workspace event that follows publishes it.
// Focusing the body switches workspace in one step, so no other chat window takes focus first and clears its attention.
func (w *WS) enterChat() (string, error) {
	prev := w.state.GetArrival()
	attention := w.state.GetAttention()
	tb := NewThreeBody(w.hypr, w.state)
	for _, name := range chatUrgency {
		spec, _ := bodySpec(name, chatWorkspace)
		if !slices.Contains(attention, strings.ToLower(spec.Class)) {
			continue
		}
		w.state.SetArrival(chatWorkspace, spec.Class)
		if _, err := tb.Focus(chatWorkspace, name, spec.Class, spec.Title, ""); err != nil {
			w.state.SetArrival(chatWorkspace, prev)
			return "", err
		}
		if active, err := w.hypr.ActiveWindow(); err == nil && active != nil &&
			active.Workspace.ID == chatWorkspace && strings.EqualFold(active.Class, spec.Class) {
			return fmt.Sprintf("ws %d: %s", chatWorkspace, active.Class), nil
		}
	}

	w.state.SetArrival(chatWorkspace, prev)
	if err := w.hypr.FocusWorkspace(chatWorkspace); err != nil {
		return "", err
	}
	return fmt.Sprintf("ws %d", chatWorkspace), nil
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
		clients, err := w.hypr.Clients()
		if err != nil {
			return err
		}
		if shadow := windows.Parked(tb, clients); shadow != nil {
			if err := windows.Unpark(w.hypr, shadow.Address, wsID); err != nil {
				return fmt.Errorf("restore three-body shadow on ws %d: %w", wsID, err)
			}
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
