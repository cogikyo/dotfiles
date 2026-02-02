package commands

import (
	"fmt"
	"strings"

	"dotfiles/cmd/hyprd/hypr"
)

// Monocle toggles fullscreen floating mode for focused work on a dedicated workspace.
type Monocle struct {
	hypr  *hypr.Client
	state StateManager
}

// NewMonocle returns a new Monocle command handler.
func NewMonocle(h *hypr.Client, s StateManager) *Monocle {
	return &Monocle{hypr: h, state: s}
}

// Execute toggles monocle mode on the active window. Entering monocle mode
// floats the window, resizes it, and moves it to a dedicated workspace.
// Exiting restores the window to its original workspace and tiling position.
func (m *Monocle) Execute() (string, error) {
	// Get active window
	win, err := m.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	// Check current monocle state
	monocle := m.state.GetMonocle()

	cfg := m.state.GetConfig()

	// Case 1: Window is on monocle workspace and floating - restore it
	if win.Workspace.ID == cfg.Monocle.Workspace && win.Floating {
		if monocle != nil && monocle.Address == win.Address {
			return m.restore(monocle)
		}
		// Floating on WS6 but no state - just unfloat
		m.hypr.Dispatch("togglefloating")
		return "unfloated orphan monocle window", nil
	}

	// Case 2: Not on monocle WS, but monocle exists - restore existing first
	if monocle != nil && win.Address != monocle.Address {
		if _, err := m.restore(monocle); err != nil {
			return "", err
		}
		return "restored existing monocle", nil
	}

	// Case 3: Enter monocle mode
	return m.enter(win)
}

// enter puts a window into monocle mode.
func (m *Monocle) enter(win *hypr.Window) (string, error) {
	cfg := m.state.GetConfig()

	// Determine position (master or slave index)
	position, err := m.getPosition(win)
	if err != nil {
		return "", err
	}

	// Save state
	m.state.SetMonocle(&MonocleState{
		Address:  win.Address,
		OriginWS: win.Workspace.ID,
		Position: position,
	})

	// Get current monitor geometry
	geo := m.state.GetGeometry()

	// Execute Hyprland commands
	// Use movetoworkspacesilent to move window without switching view,
	// then workspace to switch view (keeps focus on monocle window)
	batch := fmt.Sprintf(
		"dispatch togglefloating; "+
			"dispatch resizeactive exact %d %d; "+
			"dispatch centerwindow; "+
			"dispatch movetoworkspacesilent %d; "+
			"dispatch workspace %d; "+
			"dispatch moveactive 0 25; "+
			"keyword general:col.active_border %s; "+
			"keyword decoration:shadow:color %s",
		geo.MonocleW, geo.MonocleH,
		cfg.Monocle.Workspace,
		cfg.Monocle.Workspace,
		cfg.Style.Border.Monocle, cfg.Style.Shadow.Monocle,
	)

	if _, err := m.hypr.Request("[[BATCH]]" + batch); err != nil {
		return "", fmt.Errorf("enter monocle: %w", err)
	}

	// Move cursor to window center
	centerCursor(m.hypr)

	return fmt.Sprintf("monocle: %s from ws%d (%s)", win.Address, win.Workspace.ID, position), nil
}

// restore returns a monocle window to its original position.
func (m *Monocle) restore(monocle *MonocleState) (string, error) {
	cfg := m.state.GetConfig()

	// Reset colors, unfloat, move back to origin, and switch view
	batch := fmt.Sprintf(
		"dispatch focuswindow address:%s; "+
			"dispatch togglefloating; "+
			"dispatch movetoworkspacesilent %d; "+
			"dispatch workspace %d; "+
			"keyword general:col.active_border %s; "+
			"keyword decoration:shadow:color %s",
		monocle.Address,
		monocle.OriginWS,
		monocle.OriginWS,
		cfg.Style.Border.Default, cfg.Style.Shadow.Default,
	)

	if _, err := m.hypr.Request("[[BATCH]]" + batch); err != nil {
		return "", fmt.Errorf("restore monocle: %w", err)
	}

	// Restore position in tiling layout
	switch monocle.Position {
	case "master":
		m.hypr.Dispatch("movewindow u")
		m.hypr.Dispatch("movewindow l")
	case "0":
		m.hypr.Dispatch("movewindow u")
	}

	// Clear state
	m.state.SetMonocle(nil)

	return fmt.Sprintf("restored: %s to ws%d (%s)", monocle.Address, monocle.OriginWS, monocle.Position), nil
}

// getPosition determines if window is master or its slave index.
func (m *Monocle) getPosition(win *hypr.Window) (string, error) {
	if win.Floating {
		return "floating", nil
	}

	cfg := m.state.GetConfig()
	tiled, err := GetTiledWindows(m.hypr, win.Workspace.ID, cfg.Windows.IgnoredClasses)
	if err != nil {
		return "", err
	}

	if len(tiled) == 0 || IsMaster(tiled, win.Address) {
		return "master", nil
	}

	slaves := GetSlaves(tiled)
	idx := SlaveIndex(slaves, win.Address)
	if idx >= 0 {
		return fmt.Sprintf("%d", idx), nil
	}

	return "0", nil
}

// FormatAddress returns the address with a "0x" prefix if not already present.
func FormatAddress(addr string) string {
	if !strings.HasPrefix(addr, "0x") {
		return "0x" + addr
	}
	return addr
}
