package wm

// swap.go toggles master ownership by promoting the active slave or restoring the displaced master.

import (
	"fmt"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

// Swap promotes the active slave to master, or restores the displaced master.
//
// Per-workspace displaced-master state is tracked in state.State so the toggle survives focus changes.
type Swap struct {
	hypr  *hypr.Client
	state *state.State
}

func NewSwap(h *hypr.Client, s *state.State) *Swap {
	return &Swap{hypr: h, state: s}
}

// Execute toggles master for the active window.
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

func (s *Swap) takeoverMaster(slave *hypr.Window, wsID int, masterAddr string) (string, error) {
	s.state.SetDisplacedMaster(wsID, masterAddr)
	s.hypr.Dispatch("movewindow l")
	return fmt.Sprintf("takeover: %s to master, displaced %s", slave.Address, masterAddr), nil
}

func (s *Swap) getMasterAddr(wsID int) (string, error) {
	master, err := windows.GetMaster(s.hypr, wsID)
	if err != nil {
		return "", err
	}
	if master == nil {
		return "", nil
	}
	return master.Address, nil
}
