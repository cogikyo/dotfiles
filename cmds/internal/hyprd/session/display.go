package session

import (
	"fmt"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

func ResolveDisplay(monitors []hypr.Monitor, compactBelow int) (state.Display, bool) {
	var widest string
	var widestWidth float64
	for _, m := range monitors {
		if m.MirrorOf != "none" {
			continue
		}
		if width, _ := m.LogicalSize(); widest == "" || width > widestWidth {
			widest, widestWidth = m.Name, width
		}
	}
	if widest == "" {
		return state.Display{}, false
	}
	layout := config.LayoutFull
	if widestWidth < float64(compactBelow) {
		layout = config.LayoutCompact
	}
	return state.Display{Layout: layout, Monitor: widest}, true
}

func (s *Share) SyncDisplay() (changed bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	monitors, err := s.hypr.Monitors()
	if err != nil {
		return false, err
	}
	if display, ok := ResolveDisplay(monitors, s.state.GetConfig().Windows.CompactBelow); ok {
		changed = s.state.SetDisplay(display)
	}
	return changed, setGaps(s.hypr, s.state.OuterGaps())
}

func (s *Share) Relayout() error {
	changed, err := s.SyncDisplay()
	if changed {
		startDetached("ewwd", "relayout")
	}
	return err
}

func setGaps(h *hypr.Client, gaps config.OuterGaps) error {
	if err := h.SetOuterGaps(gaps.Sides()); err != nil {
		return fmt.Errorf("set gaps_out: %w", err)
	}
	return nil
}
