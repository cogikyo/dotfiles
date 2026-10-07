package wm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/hyprd/browser"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/kitty"
	"dotfiles/cmds/internal/hyprd/windows"
)

const tabUsage = "usage: tab <left|right>:<1..8> [group]"

type Tab struct {
	hypr *hypr.Client
}

func NewTab(h *hypr.Client) *Tab {
	return &Tab{hypr: h}
}

func (t *Tab) Execute(args string) (string, error) {
	fields := strings.Fields(args)
	if len(fields) == 0 || len(fields) > 2 || len(fields) == 2 && fields[1] != "group" {
		return "", errors.New(tabUsage)
	}
	group := len(fields) == 2

	side, raw, _ := strings.Cut(fields[0], ":")
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 8 || (side != "left" && side != "right") {
		return "", errors.New(tabUsage)
	}

	win, err := t.window(side)
	if err != nil {
		return "", err
	}
	if win == nil {
		return fmt.Sprintf("no %s window", side), nil
	}
	if err := t.hypr.FocusWindow(win.Address); err != nil {
		return "", fmt.Errorf("focus %s: %w", side, err)
	}

	switch {
	case win.Class == "kitty" && group:
		return "no tab groups in kitty", nil
	case win.Class == "kitty":
		err = kitty.NewClient(win.Pid).GotoTab(n)
	case browser.IsFirefoxWindow(*win) && group:
		err = t.hypr.SendShortcut("ALT SHIFT", strconv.Itoa(n), win.Address)
	case browser.IsFirefoxWindow(*win):
		err = t.hypr.SendShortcut("ALT", strconv.Itoa(n), win.Address)
	default:
		return fmt.Sprintf("no tabs in %s", win.Class), nil
	}
	if err != nil {
		return "", fmt.Errorf("tab %s: %w", strings.Join(fields, " "), err)
	}
	return "tab: " + strings.Join(fields, " "), nil
}

func (t *Tab) window(side string) (*hypr.Window, error) {
	wsID, err := t.hypr.ActiveWorkspace()
	if err != nil {
		return nil, err
	}
	tiled, err := windows.GetTiledWindows(t.hypr, wsID)
	if err != nil {
		return nil, err
	}
	if len(tiled) == 0 {
		return t.hypr.ActiveWindow()
	}
	if side == "right" {
		if slaves := windows.GetSlaves(tiled); len(slaves) > 0 {
			return &slaves[0], nil
		}
	}
	return &tiled[0], nil
}
