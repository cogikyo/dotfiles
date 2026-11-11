package opencode

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/kitty"
)

type Occupant int

const (
	Missing Occupant = iota
	Shell
	Home
	Starting
	Busy
	Shared
)

type Tab struct {
	Name     string
	Pane     kitty.Pane
	Occupant Occupant
}

type Navigate struct {
	kitty.PaneKey
	SessionID string `json:"session_id"`
}

const (
	settleTimeout = 10 * time.Second
	switchTimeout = 10 * time.Second
	launchTimeout = 30 * time.Second
)

func Showing(views *kitty.Tracker, id string, skip kitty.PaneKey) (kitty.PaneKey, bool) {
	panes, _, _ := views.View()
	for _, located := range panes {
		if located.Key() == skip {
			continue
		}
		if tui, ok := foregroundTUI(located.Pane); ok {
			if report, _ := readReport(tui.PID); sessionID(tui, report) == id {
				return located.Key(), true
			}
		}
	}
	return kitty.PaneKey{}, false
}

func Choose(ctx context.Context, term *kitty.Client, profile config.TabProfile, ensure func(tab string) error) (Tab, error) {
	ensured := 0
	deadline := time.Now().Add(settleTimeout)
	for {
		if err := context.Cause(ctx); err != nil {
			return Tab{}, err
		}
		windows, err := term.WaitInit(ctx)
		if err != nil {
			return Tab{}, err
		}
		panes := windows[0].ProfilePanes(profile)
		tabs := make([]Tab, len(profile.Tabs))
		for i, def := range profile.Tabs {
			tabs[i] = Tab{Name: def.Name}
			if pane, ok := panes[def.Name]; ok {
				tabs[i].Pane, tabs[i].Occupant = pane.Pane, occupy(pane)
			}
		}
		tab, settled := Pick(tabs)
		switch {
		case settled && tab.Occupant == Missing && ensured < len(profile.Tabs):
			if err := ensure(tab.Name); err != nil {
				return Tab{}, fmt.Errorf("ensure %s tab: %w", tab.Name, err)
			}
			ensured++
			deadline = time.Now().Add(settleTimeout)
			continue
		case settled, !time.Now().Before(deadline):
			return tab, nil
		}
		if err := sleep(ctx, 250*time.Millisecond); err != nil {
			return Tab{}, err
		}
	}
}

func Pick(tabs []Tab) (Tab, bool) {
	if len(tabs) == 0 {
		return Tab{}, true
	}
	order := append(slices.Clone(tabs[1:]), tabs[0])
	for _, tab := range order {
		switch tab.Occupant {
		case Missing, Shell, Home:
			return tab, true
		case Starting:
			return tab, false
		}
	}
	return order[0], true
}

func occupy(pane kitty.ProfilePane) Occupant {
	if pane.Windows > 1 {
		return Shared
	}
	if tui, ok := tuiProcess(pane.Pane); ok {
		report, err := readReport(tui.PID)
		switch {
		case err != nil:
			return Busy
		case report == nil, report.Session == nil && report.Draft == nil:
			return Starting
		case report.Session == nil && !*report.Draft:
			return Home
		}
		return Busy
	}
	switch {
	case loginZshReady(pane.Pane) && pane.AtPrompt:
		return Shell
	case loginZshCommand(pane.Cmdline) && pane.LastCommand == "" && time.Since(pane.Created()) < settleTimeout:
		return Starting
	}
	return Busy
}

func Open(ctx context.Context, pid int, tab Tab, id string, publish func(Navigate)) (confirm func() error, err error) {
	term := kitty.NewClient(pid)
	paneID := tab.Pane.ID
	_, tui := tuiProcess(tab.Pane)
	switch {
	case tab.Occupant == Shell:
		if err := term.SendText(ctx, paneID, launchCommand(id)); err != nil {
			return nil, err
		}
		return func() error { return await(ctx, term, paneID, id, launchTimeout, nil) }, nil
	case tab.Occupant == Shared:
		return nil, fmt.Errorf("tab %s has more than one window", tab.Name)
	case tui:
		if err := context.Cause(ctx); err != nil {
			return nil, err
		}
		event := Navigate{kitty.PaneKey{KittyPID: pid, PaneID: paneID}, id}
		nudge := func() { publish(event) }
		nudge()
		return func() error { return await(ctx, term, paneID, id, switchTimeout, nudge) }, nil
	case tab.Occupant == Starting:
		return nil, fmt.Errorf("tab %s has not reached a prompt", tab.Name)
	case tab.Occupant == Missing:
		return nil, fmt.Errorf("tab %s is missing", tab.Name)
	}
	return nil, fmt.Errorf("tab %s runs %s", tab.Name, foreground(tab.Pane))
}

func launch(ctx context.Context, term *kitty.Client, paneID int, id string) error {
	if err := term.SendText(ctx, paneID, launchCommand(id)); err != nil {
		return err
	}
	return await(ctx, term, paneID, id, launchTimeout, nil)
}

func await(ctx context.Context, term *kitty.Client, paneID int, id string, timeout time.Duration, nudge func()) error {
	want := "OpenCode"
	if ValidSessionID(id) {
		want = id
	}
	ctx, cancel := context.WithTimeoutCause(ctx, timeout, fmt.Errorf("pane %d did not show %s within %s", paneID, want, timeout))
	defer cancel()
	for {
		pane, found, err := term.Pane(ctx, paneID)
		if err == nil && found {
			if tui, ok := tuiProcess(pane); ok {
				if report, _ := readReport(tui.PID); report != nil {
					if !ValidSessionID(id) || report.Session != nil && *report.Session == id {
						return nil
					}
					if err := context.Cause(ctx); err != nil {
						return err
					}
					if nudge != nil {
						nudge()
					}
				}
			}
		}
		if err := sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-time.After(d):
		return nil
	}
}

func foreground(pane kitty.Pane) string {
	for _, process := range pane.ForegroundProcesses {
		if process.PID != pane.PID && len(process.Cmdline) > 0 {
			return filepath.Base(process.Cmdline[0])
		}
	}
	return "a non-OpenCode process"
}

func launchCommand(id string) string {
	if ValidSessionID(id) {
		return " c --session " + id + "\n"
	}
	return " c\n"
}
