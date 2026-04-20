package wm

// focus.go targets a window by class/title and restores it from hidden workspaces before focusing.

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
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

	wsData, err := f.hypr.Request("j/activeworkspace")
	if err != nil {
		return "", err
	}

	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		return "", fmt.Errorf("parse workspace: %w", err)
	}

	clients, err := f.hypr.Clients()
	if err != nil {
		return "", err
	}
	hiddenPrefix := f.state.GetConfig().Windows.HiddenWorkspace

	var target *hypr.Window
	var hiddenTarget *hypr.Window
	for i := range clients {
		c := &clients[i]
		if !windows.MatchesTarget(c, class, title) {
			continue
		}
		if c.Workspace.ID == ws.ID {
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
		if _, err := hide.UnhideByAddress(target.Address, ws.ID); err != nil {
			return "", fmt.Errorf("unhide: %w", err)
		}
	}

	f.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", target.Address))
	return fmt.Sprintf("focused: %s (%s)", target.Title, target.Address), nil
}
