package commands

// ================================================================================
// Hide/show mode for managing window visibility via special workspaces
// ================================================================================

import (
	"fmt"
	"strings"

	"dotfiles/cmd/hyprd/hypr"
)

// Hide handles the hide/show command execution.
type Hide struct {
	hypr  *hypr.Client
	state StateManager
}

// NewHide creates a hide command handler.
func NewHide(h *hypr.Client, s StateManager) *Hide {
	return &Hide{hypr: h, state: s}
}

// Execute toggles hide/show on the active window.
// - If window is on special:hiddenSlaves: bring it back to tiled
// - If window is a tiled slave: hide it to special:hiddenSlaves
// - If window is master: do nothing (only slaves can be hidden)
func (h *Hide) Execute() (string, error) {
	win, err := h.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	// Case 1: Window is on special workspace (hidden) - unhide it
	if isOnHiddenWorkspace(win) {
		return h.unhide(win)
	}

	// Case 2: Window is floating - ignore (might be monocle)
	if win.Floating {
		return "ignored: floating window", nil
	}

	// Get tiled windows on this workspace
	tiled, err := GetTiledWindows(h.hypr, win.Workspace.ID)
	if err != nil {
		return "", err
	}

	// Case 3: Window is master - can't hide master
	if IsMaster(tiled, win.Address) {
		return "cannot hide master window", nil
	}

	// Case 4: Only 2 windows - hiding slave would leave just master
	if len(tiled) <= 2 {
		return "need at least 3 tiled windows to hide", nil
	}

	// Case 5: Hide the slave
	return h.hide(win, tiled)
}

// hide moves a slave window to special:hiddenSlaves workspace.
func (h *Hide) hide(win *hypr.Window, tiled []hypr.Window) (string, error) {
	// Calculate slave index for later restoration
	slaves := GetSlaves(tiled)
	slaveIndex := SlaveIndex(slaves, win.Address)
	if slaveIndex < 0 {
		slaveIndex = 0
	}

	// Save state
	h.state.AddHidden(&HiddenState{
		Address:    win.Address,
		OriginWS:   win.Workspace.ID,
		SlaveIndex: slaveIndex,
	})

	// Move to special workspace (silent - doesn't switch view)
	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		HiddenWorkspace, win.Address)); err != nil {
		return "", fmt.Errorf("hide window: %w", err)
	}

	return fmt.Sprintf("hidden: %s (slave %d) to %s", win.Address, slaveIndex, HiddenWorkspace), nil
}

// unhide brings a window back from special:hiddenSlaves to its origin workspace.
func (h *Hide) unhide(win *hypr.Window) (string, error) {
	hidden := h.state.RemoveHidden(win.Address)

	// Determine destination workspace
	destWS := 1 // Default fallback
	if hidden != nil {
		destWS = hidden.OriginWS
	}

	// Move back to origin workspace
	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d,address:%s",
		destWS, win.Address)); err != nil {
		return "", fmt.Errorf("unhide window: %w", err)
	}

	// If we have state, try to restore slave position
	if hidden != nil {
		h.restoreSlavePosition(win.Address, destWS, hidden.SlaveIndex)
	}

	return fmt.Sprintf("unhidden: %s to ws%d", win.Address, destWS), nil
}

// restoreSlavePosition attempts to move the window back to its original slave index.
func (h *Hide) restoreSlavePosition(addr string, wsID int, targetIndex int) {
	// Re-query tiled windows after unhide
	tiled, err := GetTiledWindows(h.hypr, wsID)
	if err != nil || len(tiled) < 2 {
		return
	}

	// After unhiding, window goes to end of stack
	// Need to swap it back to saved index
	slaveCount := len(tiled) - 1 // Exclude master
	if slaveCount > 0 && targetIndex < slaveCount {
		swaps := slaveCount - 1 - targetIndex
		for range swaps {
			h.hypr.Dispatch("layoutmsg swapprev")
		}
	}
}

// isOnHiddenWorkspace checks if a window is on the hidden special workspace.
func isOnHiddenWorkspace(win *hypr.Window) bool {
	return strings.HasPrefix(win.Workspace.Name, "special:hiddenSlaves")
}

// UnhideByAddress brings a specific hidden window back to tiled.
// Used by focus command when focusing a hidden window.
func (h *Hide) UnhideByAddress(addr string, destWS int) (string, error) {
	hidden := h.state.RemoveHidden(addr)

	// Use saved origin if available and destWS not specified
	if destWS <= 0 && hidden != nil {
		destWS = hidden.OriginWS
	}
	if destWS <= 0 {
		destWS = 1 // Fallback
	}

	// Move back to workspace
	if err := h.hypr.Dispatch(fmt.Sprintf("movetoworkspace %d,address:%s",
		destWS, addr)); err != nil {
		return "", fmt.Errorf("unhide window: %w", err)
	}

	// Restore position if we have state
	if hidden != nil {
		h.restoreSlavePosition(addr, destWS, hidden.SlaveIndex)
	}

	return fmt.Sprintf("unhidden: %s to ws%d", addr, destWS), nil
}
