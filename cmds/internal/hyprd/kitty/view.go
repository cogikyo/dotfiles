package kitty

import (
	"errors"
	"sync"
	"time"

	"dotfiles/cmds/internal/hyprd/hypr"
)

type Seen struct {
	Shown  bool
	Hidden time.Time
}

type Tracker struct {
	hypr   *hypr.Client
	mu     sync.Mutex
	hidden map[PaneKey]time.Time
}

func NewTracker(h *hypr.Client) *Tracker {
	return &Tracker{hypr: h, hidden: make(map[PaneKey]time.Time)}
}

func (t *Tracker) View() ([]LocatedPane, map[PaneKey]Seen, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	panes, err := Inventory()
	if panes == nil {
		clear(t.hidden)
		return nil, nil, err
	}
	windows, windowsErr := t.hypr.Clients()
	monitors, monitorsErr := t.hypr.Monitors()
	if windowsErr != nil || monitorsErr != nil {
		clear(t.hidden)
		return panes, nil, errors.Join(err, windowsErr, monitorsErr)
	}

	now := time.Now()
	seen := make(map[PaneKey]Seen)
	hidden := make(map[PaneKey]time.Time)
	for key, shown := range InView(panes, windows, monitors) {
		if shown {
			seen[key] = Seen{Shown: true}
			continue
		}
		since, ok := t.hidden[key]
		if !ok {
			// Hidden duration is a lower bound from first observation after startup.
			since = now
		}
		hidden[key] = since
		seen[key] = Seen{Hidden: since}
	}
	t.hidden = hidden
	return panes, seen, err
}

func (t *Tracker) Shown(pane LocatedPane) (shown, known bool) {
	windows, windowsErr := t.hypr.Clients()
	monitors, monitorsErr := t.hypr.Monitors()
	if windowsErr != nil || monitorsErr != nil {
		return false, false
	}
	shown, known = InView([]LocatedPane{pane}, windows, monitors)[pane.Key()]
	if shown || !known {
		t.mu.Lock()
		delete(t.hidden, pane.Key())
		t.mu.Unlock()
	}
	return shown, known
}

func InView(panes []LocatedPane, windows []hypr.Window, monitors []hypr.Monitor) map[PaneKey]bool {
	shown := make(map[int]bool)
	for _, monitor := range monitors {
		for _, workspace := range []hypr.WsRef{monitor.ActiveWS, monitor.SpecialWS} {
			if workspace.ID != 0 {
				shown[workspace.ID] = true
			}
		}
	}
	count := make(map[int]int)
	visible := make(map[int]bool)
	for _, window := range windows {
		count[window.Pid]++
		visible[window.Pid] = shown[window.Workspace.ID]
	}
	view := make(map[PaneKey]bool, len(panes))
	for _, pane := range panes {
		// Hyprland's PID cannot distinguish several OS windows in one Kitty.
		if count[pane.KittyPID] == 1 {
			view[pane.Key()] = visible[pane.KittyPID] && pane.TabActive
		}
	}
	return view
}
