package wm

// split.go applies and cycles configured mfact presets for the current tiling layout.

import (
	"fmt"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Split controls the master/slave mfact ratio via named presets from cfg.Windows.Split.
type Split struct {
	hypr  *hypr.Client
	state *state.State
}

func NewSplit(h *hypr.Client, s *state.State) *Split {
	return &Split{hypr: h, state: s}
}

// Execute applies or cycles the split ratio: "xs"/"-x", "lg"/"-l", "default", "reapply"/"-r", or cycle.
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
	case "reapply", "-r":
		result, err := s.setRatio(current)
		if err != nil {
			return "", err
		}
		windows.CenterCursor(s.hypr)
		return result, nil
	default:
		return s.cycle(current)
	}
}

func (s *Split) setRatio(ratio string) (string, error) {
	cfg := s.state.GetConfig()

	var mfact string
	switch ratio {
	case "xs":
		mfact = cfg.Windows.Split.XS
	case "lg":
		mfact = cfg.Windows.Split.LG
	default:
		ratio = "default"
		mfact = cfg.Windows.Split.Default
	}

	if err := s.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", mfact)); err != nil {
		return "", fmt.Errorf("set mfact: %w", err)
	}

	s.state.SetSplitRatio(ratio)
	return fmt.Sprintf("split: %s (%s)", ratio, mfact), nil
}

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
