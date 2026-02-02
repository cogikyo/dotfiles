package commands

import (
	"fmt"

	"dotfiles/daemons/hyprd/hypr"
)

// Split manages master/slave split ratios with cycling and direct ratio selection.
type Split struct {
	hypr  *hypr.Client
	state StateManager
}

// NewSplit returns a new Split command handler.
func NewSplit(h *hypr.Client, s StateManager) *Split {
	return &Split{hypr: h, state: s}
}

// Execute sets or cycles the split ratio. Supported flags are "xs", "default",
// "lg", "toggle" (between default and xs), "reapply", or empty string to cycle
// through xs, default, and lg in order.
func (s *Split) Execute(flag string) (string, error) {
	// Ignore if active window is floating
	win, err := s.hypr.ActiveWindow()
	if err != nil {
		return "", err
	}
	if win != nil && win.Floating {
		return "ignored: floating window", nil
	}

	current := s.state.GetSplitRatio()

	switch flag {
	case "xs", "-x":
		return s.setRatio("xs")
	case "lg", "-l":
		return s.setRatio("lg")
	case "default":
		return s.setRatio("default")
	case "toggle", "-d":
		// Toggle between default and xs
		if current == "default" {
			return s.setRatio("xs")
		}
		return s.setRatio("default")
	case "reapply", "-r":
		// Reapply current ratio and center cursor
		result, err := s.setRatio(current)
		if err != nil {
			return "", err
		}
		centerCursor(s.hypr)
		return result, nil
	default:
		// Cycle: xs → default → lg → xs
		return s.cycle(current)
	}
}

// setRatio applies a split ratio.
func (s *Split) setRatio(ratio string) (string, error) {
	cfg := s.state.GetConfig()

	var mfact string
	switch ratio {
	case "xs":
		mfact = cfg.Split.XS
	case "lg":
		mfact = cfg.Split.LG
	default:
		ratio = "default"
		mfact = cfg.Split.Default
	}

	if err := s.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", mfact)); err != nil {
		return "", fmt.Errorf("set mfact: %w", err)
	}

	s.state.SetSplitRatio(ratio)
	return fmt.Sprintf("split: %s (%s)", ratio, mfact), nil
}

// cycle moves to the next split ratio.
func (s *Split) cycle(current string) (string, error) {
	var next string
	switch current {
	case "xs":
		next = "default"
	case "default":
		next = "lg"
	default:
		next = "xs"
	}
	return s.setRatio(next)
}
