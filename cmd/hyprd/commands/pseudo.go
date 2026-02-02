package commands

// ================================================================================
// Pseudo-master mode for floating overlay of slave area
// ================================================================================

import (
	"fmt"

	"dotfiles/cmd/hyprd/hypr"
)

// Pseudo handles the pseudo-master command execution.
type Pseudo struct {
	hypr  *hypr.Client
	state StateManager
}

// NewPseudo creates a pseudo command handler.
func NewPseudo(h *hypr.Client, s StateManager) *Pseudo {
	return &Pseudo{hypr: h, state: s}
}

// Execute toggles pseudo-master mode on the active window.
func (p *Pseudo) Execute() (string, error) {
	win, err := p.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	pseudo := p.state.GetPseudo()

	// Case 1: Window is floating and has pseudo state - restore it
	if win.Floating && pseudo != nil && pseudo.Address == win.Address {
		return p.restore(win, pseudo)
	}

	// Case 2: Window is floating but no pseudo state - ignore (might be monocle)
	if win.Floating {
		return "ignored: floating window without pseudo state", nil
	}

	// Get tiled windows on this workspace
	var tiled []hypr.Window
	tiled, err = GetTiledWindows(p.hypr, win.Workspace.ID)
	if err != nil {
		return "", err
	}

	// Case 3: Only 2 windows - just cycle focus
	if len(tiled) == 2 {
		p.hypr.Dispatch("cyclenext prev")
		return "cycled: only 2 windows", nil
	}

	// Case 4: Window is master - move focus to slave first
	if IsMaster(tiled, win.Address) {
		p.hypr.Dispatch("movefocus r")
		// Re-get active window after focus change
		win, err = p.hypr.ActiveWindow()
		if err != nil {
			return "", err
		}
	}

	// Case 5: Enter pseudo-master mode
	return p.enter(win)
}

// enter puts a slave window into pseudo-master mode.
func (p *Pseudo) enter(win *hypr.Window) (string, error) {
	tiled, err := GetTiledWindows(p.hypr, win.Workspace.ID)
	if err != nil {
		return "", err
	}

	if len(tiled) < 2 {
		return "need at least 2 tiled windows", nil
	}

	// Calculate slave area (area to the right of master)
	slaveX, slaveW := getSlaveArea(tiled)
	if slaveW == 0 {
		return "no slave area found", nil
	}

	// Get slave index for restoration
	slaves := GetSlaves(tiled)
	slaveIndex := SlaveIndex(slaves, win.Address)
	if slaveIndex < 0 {
		slaveIndex = 0
	}

	// Save state
	p.state.SetPseudo(&PseudoState{
		Address:    win.Address,
		SlaveIndex: slaveIndex,
	})

	// Float and position over slave area
	batch := fmt.Sprintf(
		"dispatch togglefloating;"+
			"dispatch moveactive exact %d %d;"+
			"dispatch resizeactive exact %d %d",
		slaveX, ReservedTop,
		slaveW, UsableHeight,
	)

	if _, err := p.hypr.Request("[[BATCH]]" + batch); err != nil {
		return "", fmt.Errorf("enter pseudo: %w", err)
	}

	return fmt.Sprintf("pseudo: %s (slave %d)", win.Address, slaveIndex), nil
}

// restore returns a pseudo-master window to the tiled stack.
func (p *Pseudo) restore(win *hypr.Window, pseudo *PseudoState) (string, error) {
	// Unfloat
	if err := p.hypr.Dispatch("togglefloating"); err != nil {
		return "", err
	}

	// Get current tiled windows to calculate swap count
	tiled, err := GetTiledWindows(p.hypr, win.Workspace.ID)
	if err != nil {
		return "", err
	}

	// Swap back to original position
	// After unfloating, window goes to end of stack
	// Need to swap it back to saved index
	slaveCount := len(tiled) - 1 // Exclude master
	if slaveCount > 0 && pseudo.SlaveIndex < slaveCount {
		swaps := slaveCount - 1 - pseudo.SlaveIndex
		for range swaps {
			p.hypr.Dispatch("layoutmsg swapprev")
		}
	}

	// Clear state
	p.state.SetPseudo(nil)

	// Move focus back to master
	p.hypr.Dispatch("movefocus l")

	return fmt.Sprintf("restored pseudo: %s to slave %d", win.Address, pseudo.SlaveIndex), nil
}

// getSlaveArea returns the X position and width of the slave stack area.
func getSlaveArea(tiled []hypr.Window) (x, width int) {
	if len(tiled) < 2 {
		return 0, 0
	}

	// Master is first (leftmost), slaves are to the right
	masterX := tiled[0].At[0]

	// Find first slave (first window with X > master X)
	for _, w := range tiled[1:] {
		if w.At[0] > masterX {
			x = w.At[0]
			width = MonitorWidth - x - ReservedRight
			return
		}
	}

	return 0, 0
}
