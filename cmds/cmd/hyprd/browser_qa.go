package main

import (
	"bytes"
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

const browserQAMarker = "--opencode-browser-qa"

func (e *EventLoop) syncBrowserQA(clients []hypr.Window) {
	matches := make(map[int]bool)
	windows := make(map[string]hypr.Window)

	for _, window := range clients {
		match, checked := matches[window.Pid]
		if !checked {
			match = processHasArg(window.Pid, browserQAMarker)
			matches[window.Pid] = match
		}
		if !match {
			continue
		}
		windows[window.Address] = window
	}

	previous := e.state.GetBrowserQA()
	assignments := allocateBrowserQA(windows, previous)
	e.state.SetBrowserQA(assignments)
	for address := range e.browserQAPlaced {
		if _, live := windows[address]; !live {
			delete(e.browserQAPlaced, address)
		}
	}
	for _, assignment := range assignments {
		window := windows[assignment.Address]
		if err := e.placeBrowserQA(window, assignment.Workspace, !e.browserQAPlaced[assignment.Address]); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd browser-qa: %v\n", err)
			continue
		}
		e.browserQAPlaced[assignment.Address] = true
	}
}

func allocateBrowserQA(windows map[string]hypr.Window, previous []state.BrowserQAWindow) []state.BrowserQAWindow {
	addresses := slices.Sorted(maps.Keys(windows))
	previous = slices.Clone(previous)
	slices.SortFunc(previous, func(a, b state.BrowserQAWindow) int {
		if order := cmp.Compare(a.Slot, b.Slot); order != 0 {
			return order
		}
		return cmp.Compare(a.Address, b.Address)
	})

	used := make(map[int]bool, len(previous))
	slots := make(map[string]int, len(windows))
	for _, assignment := range previous {
		if _, live := windows[assignment.Address]; !live || assignment.Slot < 1 || used[assignment.Slot] {
			continue
		}
		slots[assignment.Address] = assignment.Slot
		used[assignment.Slot] = true
	}

	next := 1
	assignments := make([]state.BrowserQAWindow, 0, len(addresses))
	for _, address := range addresses {
		slot, assigned := slots[address]
		if !assigned {
			for used[next] {
				next++
			}
			slot = next
			used[slot] = true
		}

		window := windows[address]
		title := window.Title
		if title == "" {
			title = window.InitialTitle
		}
		assignments = append(assignments, state.BrowserQAWindow{
			Address:   address,
			Title:     title,
			Slot:      slot,
			Workspace: browserQAWorkspace(slot),
		})
	}
	slices.SortFunc(assignments, func(a, b state.BrowserQAWindow) int {
		return cmp.Compare(a.Slot, b.Slot)
	})
	return assignments
}

func browserQAWorkspace(slot int) string {
	return "browser-qa-" + strconv.Itoa(slot)
}

func (e *EventLoop) placeBrowserQA(window hypr.Window, workspace string, newAssignment bool) error {
	changed := false
	if window.Workspace.Name != workspace {
		if err := e.hypr.MoveWindowToWorkspace(window.Address, "name:"+workspace, false); err != nil {
			return fmt.Errorf("move %s: %w", window.Address, err)
		}
		changed = true
	}
	if !window.Floating {
		if err := e.hypr.SetWindowFloating(window.Address); err != nil {
			return fmt.Errorf("float %s: %w", window.Address, err)
		}
		changed = true
	}
	w, h := e.state.GetConfig().MonocleSize()
	if window.Size != [2]int{w, h} {
		if err := e.hypr.ResizeWindowExact(window.Address, w, h); err != nil {
			return fmt.Errorf("resize %s: %w", window.Address, err)
		}
		changed = true
	}
	if changed || newAssignment {
		if err := e.hypr.CenterWindow(window.Address); err != nil {
			return fmt.Errorf("center %s: %w", window.Address, err)
		}
	}
	return nil
}

func processHasArg(pid int, arg string) bool {
	if pid <= 0 {
		return false
	}

	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return false
	}
	for processArg := range bytes.FieldsFuncSeq(cmdline, func(r rune) bool {
		return r == 0 || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		if string(processArg) == arg {
			return true
		}
	}
	return false
}
