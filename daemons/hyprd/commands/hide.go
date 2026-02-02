package commands

import (
	"fmt"
	"strings"

	"dotfiles/daemons/hyprd/hypr"
)

// Hide toggles window visibility by moving slaves to/from special workspaces while preserving their position in the stack.
type Hide struct {
	hypr  *hypr.Client  // Hyprland IPC client
	state StateManager  // Tracks hidden window metadata for restoration
}

// NewHide creates a Hide command handler.
func NewHide(h *hypr.Client, s StateManager) *Hide {
	return &Hide{hypr: h, state: s}
}

// Execute toggles active window visibility, moving slaves to/from special workspace.
// Master windows, floating windows, and workspaces with fewer than 3 tiled windows are rejected.
func (h *Hide) Execute() (string, error) {
	win, err := h.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	cfg := h.state.GetConfig()

	if h.isOnHiddenWorkspace(win, cfg.Windows.HiddenWorkspace) {
		return h.unhide(win)
	}

	if win.Floating {
		return "ignored: floating window", nil
	}

	tiled, err := GetTiledWindows(h.hypr, win.Workspace.ID, cfg.Windows.IgnoredClasses)
	if err != nil {
		return "", err
	}

	if IsMaster(tiled, win.Address) {
		return "cannot hide master window", nil
	}

	if len(tiled) <= 2 {
		return "need at least 3 tiled windows to hide", nil
	}

	return h.hide(win, tiled)
}

// hide moves a slave to the hidden special workspace, saving its position for later restoration.
func (h *Hide) hide(win *hypr.Window, tiled []hypr.Window) (string, error) {
	cfg := h.state.GetConfig()
	hiddenWS := cfg.Windows.HiddenWorkspace

	slaves := GetSlaves(tiled)
	slaveIndex := SlaveIndex(slaves, win.Address)
	if slaveIndex < 0 {
		slaveIndex = 0
	}

	h.state.AddHidden(&HiddenState{
		Address:    win.Address,
		OriginWS:   win.Workspace.ID,
		SlaveIndex: slaveIndex,
	})

	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		hiddenWS, win.Address)); err != nil {
		return "", fmt.Errorf("hide window: %w", err)
	}

	return fmt.Sprintf("hidden: %s (slave %d) to %s", win.Address, slaveIndex, hiddenWS), nil
}

// unhide restores a window to its original workspace and slave position.
func (h *Hide) unhide(win *hypr.Window) (string, error) {
	hidden := h.state.RemoveHidden(win.Address)

	destWS := 1
	if hidden != nil {
		destWS = hidden.OriginWS
	}

	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d,address:%s",
		destWS, win.Address)); err != nil {
		return "", fmt.Errorf("unhide window: %w", err)
	}

	if hidden != nil {
		h.restoreSlavePosition(win.Address, destWS, hidden.SlaveIndex)
	}

	return fmt.Sprintf("unhidden: %s to ws%d", win.Address, destWS), nil
}

// restoreSlavePosition swaps the window back to its saved position in the slave stack.
func (h *Hide) restoreSlavePosition(addr string, wsID int, targetIndex int) {
	cfg := h.state.GetConfig()

	tiled, err := GetTiledWindows(h.hypr, wsID, cfg.Windows.IgnoredClasses)
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

func (h *Hide) isOnHiddenWorkspace(win *hypr.Window, hiddenWS string) bool {
	return strings.HasPrefix(win.Workspace.Name, hiddenWS)
}

// UnhideByAddress restores a hidden window by address to the specified workspace (or its saved origin if destWS <= 0).
func (h *Hide) UnhideByAddress(addr string, destWS int) (string, error) {
	hidden := h.state.RemoveHidden(addr)

	if destWS <= 0 && hidden != nil {
		destWS = hidden.OriginWS
	}
	if destWS <= 0 {
		destWS = 1
	}

	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d,address:%s",
		destWS, addr)); err != nil {
		return "", fmt.Errorf("unhide window: %w", err)
	}

	if hidden != nil {
		h.restoreSlavePosition(addr, destWS, hidden.SlaveIndex)
	}

	return fmt.Sprintf("unhidden: %s to ws%d", addr, destWS), nil
}
