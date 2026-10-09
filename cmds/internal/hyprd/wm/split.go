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
// Each workspace keeps its own preset, marked with the share mode it was last applied in and refreshed when stale.
// master.mfact follows the active workspace's preset, so master nodes Hyprland creates there start at it.
type Split struct {
	hypr  *hypr.Client
	state *state.State
}

func NewSplit(h *hypr.Client, s *state.State) *Split {
	return &Split{hypr: h, state: s}
}

// Execute applies "wide" or "narrow" from "default" and returns any other preset to "default".
//
// It also accepts "default" and "reapply"/"-r".
func (s *Split) Execute(flag string) (string, error) {
	win, err := s.hypr.ActiveWindow()
	if err != nil {
		return "", err
	}
	if win != nil && win.Floating {
		return "ignored: floating window", nil
	}
	ws, err := s.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}

	current, _ := s.ratio(ws)
	switch flag {
	case "wide", "narrow":
		if current != "default" {
			return s.Apply("default")
		}
		return s.Apply(flag)
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
		return "", fmt.Errorf("usage: split {wide|narrow|default|reapply}")
	}
}

// Apply sets the active workspace's preset and mfact, and seeds master.mfact to match.
func (s *Split) Apply(preset string) (string, error) {
	ws, err := s.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	share := s.state.GetScreenShare()
	name, mfact := s.state.GetConfig().Windows.Split.Ratio(preset, share)
	if err := s.hypr.LayoutMsg(fmt.Sprintf("mfact exact %s", mfact)); err != nil {
		return "", fmt.Errorf("set mfact: %w", err)
	}
	s.state.SetSplitMark(ws, state.SplitMark{Preset: name, Share: share})
	if err := s.seed(mfact); err != nil {
		return "", err
	}
	if err := windows.Refit(s.hypr, s.state, ws); err != nil {
		return "", fmt.Errorf("refit shadow: %w", err)
	}
	return fmt.Sprintf("split: %s (%s)", name, mfact), nil
}

// Reseed sets Hyprland's master.mfact to the active workspace's preset for the current share mode.
func (s *Split) Reseed() error {
	_, mfact := s.ratio(s.state.GetWorkspace())
	return s.seed(mfact)
}

// Refresh reapplies the active workspace's preset when its mark is stale.
//
// Monocle workspaces and floating or missing active windows are skipped and left stale.
func (s *Split) Refresh() error {
	ws := s.state.GetWorkspace()
	preset, _ := s.ratio(ws)
	want := state.SplitMark{Preset: preset, Share: s.state.GetScreenShare()}
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

// Preset returns the active workspace's preset name.
func (s *Split) Preset() string {
	name, _ := s.ratio(s.state.GetWorkspace())
	return name
}

// ratio resolves ws's preset, "default" when unmarked, and its mfact for the current share mode.
func (s *Split) ratio(ws int) (name, mfact string) {
	return s.state.GetConfig().Windows.Split.Ratio(s.state.GetSplitMark(ws).Preset, s.state.GetScreenShare())
}

func (s *Split) seed(mfact string) error {
	f, err := strconv.ParseFloat(mfact, 64)
	if err != nil {
		return fmt.Errorf("split ratio %q: %w", mfact, err)
	}
	if err := s.hypr.SetMasterFactor(f); err != nil {
		return fmt.Errorf("seed mfact: %w", err)
	}
	return nil
}
