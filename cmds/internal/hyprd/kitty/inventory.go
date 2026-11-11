package kitty

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// LocatedPane identifies a pane within one Kitty instance and OS window.
type LocatedPane struct {
	KittyPID   int
	OSWindowID int
	TabID      int
	TabActive  bool
	Pane       Pane
}

type PaneKey struct {
	KittyPID int `json:"kitty_pid"`
	PaneID   int `json:"kitty_pane_id"`
}

func (p LocatedPane) Key() PaneKey {
	return PaneKey{KittyPID: p.KittyPID, PaneID: p.Pane.ID}
}

// Inventory returns the panes exposed by all live per-PID Kitty sockets.
// A non-nil slice with a joined error means at least one socket answered.
// A nil slice with an error means zero sockets answered.
func Inventory() ([]LocatedPane, error) {
	pids, err := socketPIDs()
	if err != nil {
		return nil, err
	}

	panes := make([]LocatedPane, 0)
	var errs []error
	answered := 0
	for _, pid := range pids {
		windows, err := NewClient(pid).FullState(context.Background())
		if err != nil {
			errs = append(errs, fmt.Errorf("kitty %d: %w", pid, err))
			continue
		}
		answered++
		panes = append(panes, locate(pid, windows)...)
	}
	err = errors.Join(errs...)
	if answered == 0 {
		return nil, err
	}
	return panes, err
}

func locate(pid int, windows []OSWindow) []LocatedPane {
	var panes []LocatedPane
	for _, window := range windows {
		for _, tab := range window.Tabs {
			for _, pane := range tab.Windows {
				panes = append(panes, LocatedPane{
					KittyPID:   pid,
					OSWindowID: window.ID,
					TabID:      tab.ID,
					TabActive:  tab.IsActive,
					Pane:       pane,
				})
			}
		}
	}
	return panes
}

func socketPIDs() ([]int, error) {
	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return nil, fmt.Errorf("read /tmp: %w", err)
	}

	var pids []int
	for _, entry := range entries {
		raw, ok := strings.CutPrefix(entry.Name(), "kitty-")
		if !ok || raw == "" {
			continue
		}
		pid, err := strconv.Atoi(raw)
		if err != nil || pid <= 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSocket == 0 {
			continue
		}
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids, nil
}
