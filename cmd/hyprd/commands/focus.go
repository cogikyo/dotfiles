package commands

// ================================================================================
// Focus window by class/title, unhiding from special workspace if needed
// ================================================================================

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/cmd/hyprd/hypr"
)

// Focus handles the focus-active command.
type Focus struct {
	hypr  *hypr.Client
	state StateManager
}

// NewFocus creates a focus command handler.
func NewFocus(h *hypr.Client, s StateManager) *Focus {
	return &Focus{hypr: h, state: s}
}

// Execute focuses a window by class/title, unhiding from special workspace if needed.
// Args: "class" or "class title"
// Searches all workspaces including special:hiddenSlaves.
func (f *Focus) Execute(class, title string) (string, error) {
	if class == "" {
		return "", fmt.Errorf("class required")
	}

	// Get current workspace
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

	// Find window by class (and title if specified)
	// Search ALL clients including special workspaces
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

		// Prefer window on current workspace
		if c.Workspace.ID == currentWS {
			target = c
			break
		}

		// Track hidden window as fallback
		if strings.HasPrefix(c.Workspace.Name, "special:hiddenSlaves") {
			hiddenTarget = c
		}
	}

	// Use hidden target if no visible target found
	if target == nil {
		target = hiddenTarget
	}

	if target == nil {
		return fmt.Sprintf("not found: %s %s", class, title), nil
	}

	// If window is on special workspace, unhide it first
	if strings.HasPrefix(target.Workspace.Name, "special:") {
		hide := NewHide(f.hypr, f.state)
		_, err := hide.UnhideByAddress(target.Address, currentWS)
		if err != nil {
			return "", fmt.Errorf("unhide: %w", err)
		}
	}

	// Focus the window
	f.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", target.Address))

	return fmt.Sprintf("focused: %s (%s)", target.Title, target.Address), nil
}

// matchesTarget checks if window matches by title (preferred) or class.
func matchesTarget(w *hypr.Window, class, title string) bool {
	// Title match takes precedence (exact match)
	if title != "" && w.Title == title {
		return true
	}
	// Class match (case-insensitive)
	if class != "" && strings.EqualFold(w.Class, class) {
		return true
	}
	return false
}
