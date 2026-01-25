package commands

import (
	"encoding/json"
	"fmt"

	"hyprd/hypr"
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

// Execute focuses a window by class on the current workspace.
// Args: "class" or "class title"
func (f *Focus) Execute(class, title string) (string, error) {
	if class == "" {
		return "", fmt.Errorf("class required")
	}

	// Check if current window is in pseudo-master mode - restore it first
	win, err := f.hypr.ActiveWindow()
	if err == nil && win != nil {
		pseudo := f.state.GetPseudo()
		if pseudo != nil && pseudo.Address == win.Address {
			// Restore pseudo-master before focusing
			p := NewPseudo(f.hypr, f.state)
			p.Execute()
		}
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
	wsID := ws.ID

	// Find window by class (and title if specified)
	clients, err := f.hypr.Clients()
	if err != nil {
		return "", err
	}

	var target *hypr.Window
	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID != wsID || c.Class != class {
			continue
		}
		if title != "" && c.Title != title {
			continue
		}
		target = c
		break
	}

	if target == nil {
		return fmt.Sprintf("not found: %s", class), nil
	}

	// Focus the window
	f.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", target.Address))

	return fmt.Sprintf("focused: %s (%s)", class, target.Address), nil
}
