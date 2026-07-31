package wm

// float.go toggles the active window between tiled and a centered floating window at monocle geometry.

import (
	"fmt"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

// Float toggles floating on the active window, sizing it from cfg.Windows.Monocle on the way out of tiling.
//
// Stateless: the toggle direction comes from Window.Floating, so nothing is stored across calls.
type Float struct {
	hypr  *hypr.Client
	state *state.State
}

func NewFloat(h *hypr.Client, s *state.State) *Float {
	return &Float{hypr: h, state: s}
}

// Execute tiles a floating window, or floats a tiled window at monocle size and centers it.
func (f *Float) Execute() (string, error) {
	win, err := f.hypr.ActiveWindow()
	if err != nil {
		return "", fmt.Errorf("get active window: %w", err)
	}
	if win == nil {
		return "no active window", nil
	}

	if win.Floating {
		if err := f.hypr.ToggleFloatActive(); err != nil {
			return "", fmt.Errorf("tile window: %w", err)
		}
		return "float off: tiled", nil
	}

	w, h := f.state.GetConfig().MonocleSize()
	if err := f.hypr.ToggleFloatActive(); err != nil {
		return "", fmt.Errorf("float window: %w", err)
	}
	if err := f.hypr.ResizeActiveExact(w, h); err != nil {
		return "", fmt.Errorf("resize floating window: %w", err)
	}
	if err := f.hypr.CenterActive(); err != nil {
		return "", fmt.Errorf("center floating window: %w", err)
	}
	return fmt.Sprintf("float: %dx%d centered", w, h), nil
}
