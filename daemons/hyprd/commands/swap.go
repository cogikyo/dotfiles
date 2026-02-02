package commands

import (
	"fmt"

	"dotfiles/daemons/hyprd/hypr"
)

// Swap exchanges windows between master and slave positions with undo support.
type Swap struct {
	hypr  *hypr.Client   // Hyprland IPC client
	state StateManager   // Persistent state storage
}

// NewSwap creates a Swap handler with the given Hyprland client and state manager.
func NewSwap(h *hypr.Client, s StateManager) *Swap {
	return &Swap{hypr: h, state: s}
}

// Execute swaps the active window with the master, or restores the previously
// displaced master if called from the master position. Ignored for floating windows.
func (s *Swap) Execute() (string, error) {
	win, err := s.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}
	if win.Floating {
		return "ignored: floating window", nil
	}

	wsID := win.Workspace.ID
	masterAddr, err := s.getMasterAddr(wsID)
	if err != nil {
		return "", err
	}

	if win.Address == masterAddr {
		return s.restoreDisplaced(win, wsID)
	}

	return s.takeoverMaster(win, wsID, masterAddr)
}

// restoreDisplaced moves the previously displaced master back to the master position.
func (s *Swap) restoreDisplaced(currentMaster *hypr.Window, wsID int) (string, error) {
	displaced := s.state.GetDisplacedMaster(wsID)
	if displaced == "" {
		return "no displaced master to restore", nil
	}

	if displaced == currentMaster.Address {
		s.state.SetDisplacedMaster(wsID, "")
		return "displaced master is current master, cleared", nil
	}

	s.state.SetDisplacedMaster(wsID, currentMaster.Address)
	s.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", displaced))
	s.hypr.Dispatch("movewindow l")
	s.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", currentMaster.Address))

	return fmt.Sprintf("restored: %s to master, displaced %s", displaced, currentMaster.Address), nil
}

// takeoverMaster moves a slave window to the master position, displacing the current master.
func (s *Swap) takeoverMaster(slave *hypr.Window, wsID int, masterAddr string) (string, error) {
	s.state.SetDisplacedMaster(wsID, masterAddr)
	s.hypr.Dispatch("movewindow l")

	return fmt.Sprintf("takeover: %s to master, displaced %s", slave.Address, masterAddr), nil
}

// getMasterAddr finds the master window address for the given workspace.
func (s *Swap) getMasterAddr(wsID int) (string, error) {
	cfg := s.state.GetConfig()
	master, err := GetMaster(s.hypr, wsID, cfg.Windows.IgnoredClasses)
	if err != nil {
		return "", err
	}
	if master == nil {
		return "", nil
	}
	return master.Address, nil
}
