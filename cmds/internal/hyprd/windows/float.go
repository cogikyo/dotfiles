package windows

import (
	"math"
	"slices"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

// Recenter centers off-screen unmanaged floats at their current size.
func Recenter(h *hypr.Client, s *state.State) error {
	clients, err := h.Clients()
	if err != nil {
		return err
	}
	monitors, err := h.Monitors()
	if err != nil {
		return err
	}
	var managed []string
	for _, ms := range s.AllMonocle() {
		managed = append(managed, ms.Order...)
	}
	for _, tb := range s.AllThreeBody() {
		if shadow := Parked(tb, clients); shadow != nil {
			managed = append(managed, shadow.Address)
		}
	}
	var stray []string
	for _, c := range clients {
		if !c.Floating || !c.Mapped || IsIgnored(c.Class) || slices.Contains(managed, c.Address) {
			continue
		}
		i := slices.IndexFunc(monitors, func(m hypr.Monitor) bool { return m.ID == c.Monitor })
		if i < 0 || inside(c, monitors[i]) {
			continue
		}
		stray = append(stray, c.Address)
	}
	return h.CenterWindows(stray...)
}

func inside(w hypr.Window, m hypr.Monitor) bool {
	width, height := m.LogicalSize()
	width, height = math.Round(width), math.Round(height)
	x, y := float64(w.At[0]), float64(w.At[1])
	return x >= float64(m.X) && y >= float64(m.Y) &&
		x+float64(w.Size[0]) <= float64(m.X)+width &&
		y+float64(w.Size[1]) <= float64(m.Y)+height
}
