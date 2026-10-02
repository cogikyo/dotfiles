package wm

import (
	"fmt"
	"strconv"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Split controls the master/slave mfact ratio via named presets from cfg.Windows.Split.
//
// One preset is global; Hyprland stores mfact per workspace, so each workspace is marked
// with the preset and share mode last applied there and refreshed when it goes stale.
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
		return s.Apply("xs")
	case "lg", "-l":
		return s.Apply("lg")
	case "default":
		return s.Apply("default")
	case "reapply", "-r":
		result, err := s.Apply(current)
		if err != nil {
			return "", err
		}
		windows.CenterCursor(s.hypr)
		return result, nil
	default:
		return s.cycle(current)
	}
}

// Apply makes preset the global split, reseeds new master nodes, and sets the active workspace's mfact.
func (s *Split) Apply(preset string) (string, error) {
	share := s.state.GetScreenShare()
	name, mfact := s.state.GetConfig().Windows.Split.Ratio(preset, share)
	s.state.SetSplitRatio(name)
	if err := s.Reseed(); err != nil {
		return "", err
	}

	ws, err := s.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	if err := s.hypr.LayoutMsg(fmt.Sprintf("mfact exact %s", mfact)); err != nil {
		return "", fmt.Errorf("set mfact: %w", err)
	}
	s.state.SetSplitMark(ws, state.SplitMark{Preset: name, Share: share})
	return fmt.Sprintf("split: %s (%s)", name, mfact), nil
}

// Reseed sets Hyprland's master.mfact to the global preset for the current share mode.
func (s *Split) Reseed() error {
	_, mfact := s.state.GetConfig().Windows.Split.Ratio(s.state.GetSplitRatio(), s.state.GetScreenShare())
	f, err := strconv.ParseFloat(mfact, 64)
	if err != nil {
		return fmt.Errorf("split ratio %q: %w", mfact, err)
	}
	if err := s.hypr.SetMasterFactor(f); err != nil {
		return fmt.Errorf("seed mfact: %w", err)
	}
	return nil
}

// Refresh reapplies the global preset when the active workspace's mark is stale.
//
// Monocle workspaces and floating or missing active windows are skipped and left stale.
func (s *Split) Refresh() error {
	ws := s.state.GetWorkspace()
	want := state.SplitMark{Preset: s.state.GetSplitRatio(), Share: s.state.GetScreenShare()}
	if s.state.GetSplitMark(ws) == want || s.state.GetMonocle(ws) != nil {
		return nil
	}
	win, err := s.hypr.ActiveWindow()
	if err != nil {
		return err
	}
	if win == nil || win.Floating || win.Workspace.ID != ws {
		return nil
	}
	_, err = s.Apply(want.Preset)
	return err
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
	return s.Apply(next)
}
