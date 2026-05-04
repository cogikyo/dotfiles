package wm

// hide.go hides slave windows on a special workspace and restores them to their recorded slave position.

import (
	"fmt"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Hide parks the active slave on a special workspace and restores it with its original slave index on unhide.
//
// Refused for master windows, workspaces with fewer than 3 tiled windows, or workspaces running three-body.
type Hide struct {
	hypr  *hypr.Client
	state *state.State
}

func NewHide(h *hypr.Client, s *state.State) *Hide {
	return &Hide{hypr: h, state: s}
}

// Execute toggles hide/unhide on the active window.
func (h *Hide) Execute() (string, error) {
	win, err := h.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	if windows.IsOnHiddenWorkspace(win) {
		return h.unhide(win)
	}
	if win.Floating {
		return "ignored: floating window", nil
	}
	if tb := h.state.GetThreeBody(win.Workspace.ID); tb != nil {
		return "three-body active: use three-body focus to swap", nil
	}

	tiled, err := windows.GetTiledWindows(h.hypr, win.Workspace.ID)
	if err != nil {
		return "", err
	}
	if windows.IsMaster(tiled, win.Address) {
		return "cannot hide master window", nil
	}
	if len(tiled) <= 2 {
		return "need at least 3 tiled windows to hide", nil
	}
	return h.hide(win, tiled)
}

func (h *Hide) hide(win *hypr.Window, tiled []hypr.Window) (string, error) {
	slaves := windows.GetSlaves(tiled)
	slaveIndex := max(windows.SlaveIndex(slaves, win.Address), 0)

	h.state.AddHidden(&state.HiddenState{
		Address:    win.Address,
		OriginWS:   win.Workspace.ID,
		SlaveIndex: slaveIndex,
	})

	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", windows.HiddenWorkspace, win.Address)); err != nil {
		return "", fmt.Errorf("hide window: %w", err)
	}
	return fmt.Sprintf("hidden: %s (slave %d) to %s", win.Address, slaveIndex, windows.HiddenWorkspace), nil
}

func (h *Hide) unhide(win *hypr.Window) (string, error) {
	hidden := h.state.RemoveHidden(win.Address)
	destWS := 1
	if hidden != nil {
		destWS = hidden.OriginWS
	}

	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d,address:%s", destWS, win.Address)); err != nil {
		return "", fmt.Errorf("unhide window: %w", err)
	}
	if hidden != nil {
		h.restoreSlavePosition(destWS, hidden.SlaveIndex)
	}
	return fmt.Sprintf("unhidden: %s to ws%d", win.Address, destWS), nil
}

// restoreSlavePosition issues N `swapprev` dispatches to walk the window from the tail to its saved index.
func (h *Hide) restoreSlavePosition(wsID int, targetIndex int) {
	tiled, err := windows.GetTiledWindows(h.hypr, wsID)
	if err != nil || len(tiled) < 2 {
		return
	}

	slaveCount := len(tiled) - 1
	if slaveCount > 0 && targetIndex < slaveCount {
		swaps := slaveCount - 1 - targetIndex
		for range swaps {
			h.hypr.Dispatch("layoutmsg swapprev")
		}
	}
}

// UnhideByAddress unhides a known-hidden window to destWS (falls back to stored origin, then ws 1).
func (h *Hide) UnhideByAddress(addr string, destWS int) (string, error) {
	hidden := h.state.RemoveHidden(addr)
	if destWS <= 0 && hidden != nil {
		destWS = hidden.OriginWS
	}
	if destWS <= 0 {
		destWS = 1
	}

	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d,address:%s", destWS, addr)); err != nil {
		return "", fmt.Errorf("unhide window: %w", err)
	}
	if hidden != nil {
		h.restoreSlavePosition(destWS, hidden.SlaveIndex)
	}
	return fmt.Sprintf("unhidden: %s to ws%d", addr, destWS), nil
}
