package commands

import (
	"fmt"

	"dotfiles/daemons/hyprd/hypr"
)

// Split manages master/slave split ratios with cycling and direct ratio selection.
type Split struct {
	hypr  *hypr.Client   // Hyprland IPC client
	state StateManager   // Persistent state storage
}

// NewSplit creates a Split handler with the given Hyprland client and state manager.
func NewSplit(h *hypr.Client, s StateManager) *Split {
	return &Split{hypr: h, state: s}
}

// Execute sets or cycles the split ratio. Supported flags: "xs", "default", "lg",
// "toggle" (toggles default/xs), "reapply" (reapplies current and centers cursor),
// or empty to cycle xs -> default -> lg. Ignored for floating windows.
func (s *Split) Execute(flag string) (string, error) {
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
		if current == "default" {
			return s.setRatio("xs")
		}
		return s.setRatio("default")
	case "reapply", "-r":
		result, err := s.setRatio(current)
		if err != nil {
			return "", err
		}
		centerCursor(s.hypr)
		return result, nil
	default:
		return s.cycle(current)
	}
}

// setRatio applies the specified split ratio by dispatching the mfact layoutmsg to Hyprland.
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

// cycle advances to the next split ratio in the sequence: xs -> default -> lg -> xs.
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
