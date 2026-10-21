package wm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/hyprd/browser"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/kitty"
)

const tabUsage = "usage: tab <1..8> [group]"

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

	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 1 || n > 8 {
		return "", errors.New(tabUsage)
	}

	win, err := t.hypr.ActiveWindow()
	if err != nil {
		return "", err
	}
	if win == nil {
		return "no focused window", nil
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
