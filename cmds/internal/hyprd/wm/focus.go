package wm

import (
	"fmt"
	"strings"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Focus routes focus to a window matching class/title, preferring the active workspace over the configured hidden workspace.
type Focus struct {
	hypr  *hypr.Client
	state *state.State
}

func NewFocus(h *hypr.Client, s *state.State) *Focus {
	return &Focus{hypr: h, state: s}
}

// Execute focuses a window by class (required) and optional title, unhiding from the configured hidden workspace if needed.
func (f *Focus) Execute(class, title string) (string, error) {
	if class == "" {
		return "", fmt.Errorf("class required")
	}

	wsID, err := f.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}

	clients, err := f.hypr.Clients()
	if err != nil {
		return "", err
	}
	hiddenPrefix := windows.HiddenWorkspace

	var target *hypr.Window
	var hiddenTarget *hypr.Window
	for i := range clients {
		c := &clients[i]
		if !windows.MatchesTarget(c, class, title) {
			continue
		}
		if c.Workspace.ID == wsID {
			target = c
			break
		}
		if strings.HasPrefix(c.Workspace.Name, hiddenPrefix) {
			hiddenTarget = c
		}
	}

	if target == nil {
		target = hiddenTarget
	}
	if target == nil {
		return fmt.Sprintf("not found: %s %s", class, title), nil
	}

	if strings.HasPrefix(target.Workspace.Name, "special:") {
		hide := NewHide(f.hypr, f.state)
		if _, err := hide.UnhideByAddress(target.Address, wsID); err != nil {
			return "", fmt.Errorf("unhide: %w", err)
		}
	}

	_ = f.hypr.FocusWindow(target.Address)
	return fmt.Sprintf("focused: %s (%s)", target.Title, target.Address), nil
}
