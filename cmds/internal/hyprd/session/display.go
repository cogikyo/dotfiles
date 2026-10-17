package session

import (
	"errors"
	"fmt"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/kitty"
	"dotfiles/cmds/internal/hyprd/state"
)

// ResolveDisplay picks the widest unmirrored monitor and the first profile matching it.
func ResolveDisplay(monitors []hypr.Monitor, profiles config.DisplayProfiles) (state.Display, bool) {
	var widest *hypr.Monitor
	var widestWidth float64
	for i, m := range monitors {
		if m.MirrorOf != "none" {
			continue
		}
		if width, _ := m.LogicalSize(); widest == nil || width > widestWidth {
			widest, widestWidth = &monitors[i], width
		}
	}
	if widest == nil {
		return state.Display{}, false
	}
	ppi := widest.PPI()
	return state.Display{
		Monitor:     widest.Name,
		Description: widest.Description,
		PPI:         ppi,
		Profile:     profiles.Resolve(widest.Description, widest.Width, ppi),
	}, true
}

// SyncDisplay resolves the display, applies its gaps, and converges GLava and the Kitty font size when they changed.
func (s *Share) SyncDisplay() (prev, next state.Display, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	monitors, err := s.hypr.Monitors()
	if err != nil {
		return prev, next, err
	}
	prev = s.state.GetDisplay()
	next = prev
	if display, ok := ResolveDisplay(monitors, s.state.GetConfig().Displays); ok {
		next = display
		s.state.SetDisplay(next)
	}
	errs := []error{setGaps(s.hypr, s.state.OuterGaps())}
	if prev != next {
		SyncGLava(s.state, s.lock.Locked())
	}
	if size := next.Profile.FontSize; size > 0 && size != prev.Profile.FontSize {
		if err := kitty.SetFontSize(size); err != nil {
			errs = append(errs, fmt.Errorf("kitty font: %w", err))
		}
	}
	return prev, next, errors.Join(errs...)
}

// Relayout follows a monitor or config change: it retunes splits and restarts eww when the window set or monitor changed.
func (s *Share) Relayout() error {
	prev, next, err := s.SyncDisplay()
	if prev == next {
		return err
	}
	errs := []error{err, s.retuneSplit()}
	if prev.Monitor != next.Monitor || prev.Profile.Eww != next.Profile.Eww {
		startDetached("ewwd", "relayout")
	}
	return errors.Join(errs...)
}

func setGaps(h *hypr.Client, gaps config.OuterGaps) error {
	if err := h.SetOuterGaps(gaps.Sides()); err != nil {
		return fmt.Errorf("set gaps_out: %w", err)
	}
	return nil
}
