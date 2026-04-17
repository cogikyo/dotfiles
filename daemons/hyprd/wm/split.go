package wm

import (
	"fmt"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

// Split controls the master/slave mfact ratio via named presets (xs, default, lg) from cfg.Split.
type Split struct {
	hypr  *hypr.Client
	state *state.State
}

func NewSplit(h *hypr.Client, s *state.State) *Split {
	return &Split{hypr: h, state: s}
}

// Execute applies or cycles the master/slave split ratio.
//
// Flags:
//   - "xs"/"-x", "lg"/"-l", "default" — set that preset.
//   - "reapply"/"-r" — re-send the current preset and recenter the cursor.
//   - anything else — cycle xs → default → lg → xs.
//
// Floating windows are ignored.
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
