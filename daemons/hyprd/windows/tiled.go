// Package windows provides reusable window-selection helpers over Hyprland client lists.
//
// Responsibilities:
// - Filter and order tiled windows for master/slave layouts.
// - Match windows by class/title for command targeting.
// - Expose geometry helpers reused by wm and session packages.
package windows

// tiled.go implements tiled-window ordering, master/slave extraction, and cursor-centering helpers.

import (
	"fmt"
	"slices"
	"sort"

	"dotfiles/daemons/hyprd/hypr"
)

// CenterCursor moves the cursor to the center of the active window.
func CenterCursor(h *hypr.Client) {
	win, err := h.ActiveWindow()
	if err != nil || win == nil {
		return
	}
	x := win.At[0] + win.Size[0]/2
	y := win.At[1] + win.Size[1]/2
	h.Dispatch(fmt.Sprintf("movecursor %d %d", x, y))
}

// GetTiledWindows returns non-floating windows on wsID sorted by X position.
func GetTiledWindows(h *hypr.Client, wsID int, ignoredClasses []string) ([]hypr.Window, error) {
	clients, err := h.Clients()
	if err != nil {
		return nil, err
	}

	var tiled []hypr.Window
	for _, c := range clients {
		if c.Workspace.ID == wsID && !c.Floating && !slices.Contains(ignoredClasses, c.Class) {
			tiled = append(tiled, c)
		}
	}

	sort.Slice(tiled, func(i, j int) bool {
		return tiled[i].At[0] < tiled[j].At[0]
	})

	return tiled, nil
}

// GetMaster returns the leftmost tiled window on wsID, or nil if none.
func GetMaster(h *hypr.Client, wsID int, ignoredClasses []string) (*hypr.Window, error) {
	tiled, err := GetTiledWindows(h, wsID, ignoredClasses)
	if err != nil {
		return nil, err
	}
	if len(tiled) == 0 {
		return nil, nil
	}
	return &tiled[0], nil
}

// GetSlaves returns tiled windows right of the master, sorted by Y position.
func GetSlaves(tiled []hypr.Window) []hypr.Window {
	if len(tiled) < 2 {
		return nil
	}
	masterX := tiled[0].At[0]
	var slaves []hypr.Window
	for _, w := range tiled[1:] {
		if w.At[0] > masterX {
			slaves = append(slaves, w)
		}
	}
	sort.Slice(slaves, func(i, j int) bool {
		return slaves[i].At[1] < slaves[j].At[1]
	})
	return slaves
}

// IsMaster reports whether addr is the leftmost tiled window.
func IsMaster(tiled []hypr.Window, addr string) bool {
	return len(tiled) > 0 && tiled[0].Address == addr
}

// SlaveIndex returns addr's index in slaves, or -1 if absent.
func SlaveIndex(slaves []hypr.Window, addr string) int {
	for i, s := range slaves {
		if s.Address == addr {
			return i
		}
	}
	return -1
}
