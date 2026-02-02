package commands

// ================================================================================
// Master/slave position swapping with undo support
// ================================================================================

import (
	"fmt"

	"dotfiles/cmd/hyprd/hypr"
)

// Swap handles the swap-master command execution.
type Swap struct {
	hypr  *hypr.Client
	state StateManager
}

// NewSwap creates a swap command handler.
func NewSwap(h *hypr.Client, s StateManager) *Swap {
	return &Swap{hypr: h, state: s}
}

// Execute toggles swap between master and slave positions.
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
		// On master: restore the displaced master (now a slave)
		return s.restoreDisplaced(win, wsID)
	}

	// On slave: save current master and take master position
	return s.takeoverMaster(win, wsID, masterAddr)
}

// restoreDisplaced restores the previously displaced master.
func (s *Swap) restoreDisplaced(currentMaster *hypr.Window, wsID int) (string, error) {
	displaced := s.state.GetDisplacedMaster(wsID)
	if displaced == "" {
		return "no displaced master to restore", nil
	}

	// Sanity check
	if displaced == currentMaster.Address {
		s.state.SetDisplacedMaster(wsID, "")
		return "displaced master is current master, cleared", nil
	}

	// Save ourselves (we'll become a slave after this)
	s.state.SetDisplacedMaster(wsID, currentMaster.Address)

	// Focus displaced master (now slave) and move it back to master
	s.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", displaced))
	s.hypr.Dispatch("movewindow l")

	// Focus back to ourselves (now in slave area)
	s.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", currentMaster.Address))

	return fmt.Sprintf("restored: %s to master, displaced %s", displaced, currentMaster.Address), nil
}

// takeoverMaster moves a slave to master position.
func (s *Swap) takeoverMaster(slave *hypr.Window, wsID int, masterAddr string) (string, error) {
	// Save current master (will be displaced)
	s.state.SetDisplacedMaster(wsID, masterAddr)

	// Move slave to master position
	s.hypr.Dispatch("movewindow l")

	return fmt.Sprintf("takeover: %s to master, displaced %s", slave.Address, masterAddr), nil
}

// getMasterAddr returns the master window address for a workspace.
func (s *Swap) getMasterAddr(wsID int) (string, error) {
	master, err := GetMaster(s.hypr, wsID)
	if err != nil {
		return "", err
	}
	if master == nil {
		return "", nil
	}
	return master.Address, nil
}
