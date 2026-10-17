package windows

import (
	"errors"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

func MonocleSize(h *hypr.Client, s *state.State) (w, ht int, err error) {
	m, err := h.FocusedMonitor()
	if err != nil {
		return 0, 0, err
	}
	if m == nil {
		return 0, 0, errors.New("no monitor")
	}
	width, height := m.Area()
	w, ht = s.Profile().Monocle.Size(width, height, s.OuterGaps())
	return w, ht, nil
}
