package commands

// ================================================================================
// Master/slave split ratio cycling and management
// ================================================================================

import (
	"fmt"

	"hyprd/hypr"
)

// Split handles the split command execution.
type Split struct {
	hypr  *hypr.Client
	state StateManager
}

// NewSplit creates a split command handler.
func NewSplit(h *hypr.Client, s StateManager) *Split {
	return &Split{hypr: h, state: s}
}

// Execute runs the split command with given flags.
// Flags: "xs", "default", "lg", "toggle", "reapply", or "" (cycle)
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
		CenterCursor(s.hypr)
		return result, nil
	default:
		// Cycle: xs → default → lg → xs
		return s.cycle(current)
	}
}

// setRatio applies a split ratio.
func (s *Split) setRatio(ratio string) (string, error) {
	var mfact string
	switch ratio {
	case "xs":
		mfact = SplitXS
	case "lg":
		mfact = SplitLG
	default:
		ratio = "default"
		mfact = SplitDefault
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
