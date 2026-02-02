package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/daemons/hyprd/hypr"
)

// Focus searches all workspaces for windows matching class/title criteria and brings them into view, automatically unhiding from special workspaces when needed.
type Focus struct {
	hypr  *hypr.Client  // Hyprland IPC client
	state StateManager  // Window state tracker
}

// NewFocus creates a Focus command handler.
func NewFocus(h *hypr.Client, s StateManager) *Focus {
	return &Focus{hypr: h, state: s}
}

// Execute finds and focuses a window by class (required) and title (optional).
// Prefers windows on the current workspace, falls back to hidden windows, and
// automatically unhides from special workspaces before focusing.
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
	currentWS := ws.ID

	clients, err := f.hypr.Clients()
	if err != nil {
		return "", err
	}

	var target *hypr.Window
	var hiddenTarget *hypr.Window

	for i := range clients {
		c := &clients[i]
		if !matchesTarget(c, class, title) {
			continue
		}

		if c.Workspace.ID == currentWS {
			target = c
			break
		}

		if strings.HasPrefix(c.Workspace.Name, "special:hiddenSlaves") {
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
		_, err := hide.UnhideByAddress(target.Address, currentWS)
		if err != nil {
			return "", fmt.Errorf("unhide: %w", err)
		}
	}

	f.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", target.Address))

	return fmt.Sprintf("focused: %s (%s)", target.Title, target.Address), nil
}

// matchesTarget returns true if window matches the search criteria, preferring exact title match over case-insensitive class match.
func matchesTarget(w *hypr.Window, class, title string) bool {
	if title != "" && w.Title == title {
		return true
	}
	if class != "" && strings.EqualFold(w.Class, class) {
		return true
	}
	return false
}
