package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// KittyClient communicates with a kitty instance via remote control.
type KittyClient struct {
	socketPath string
}

// NewKittyClient creates a client for the kitty instance with the given PID.
func NewKittyClient(pid int) *KittyClient {
	return &KittyClient{socketPath: fmt.Sprintf("unix:/tmp/kitty-%d", pid)}
}

// KittyState holds the subset of kitty state needed for tab operations.
type KittyState struct {
	WindowID    int    // kitty OS window ID (used as KITTY_TAB_ID prefix)
	ActiveTabID string // KITTY_TAB_ID of the currently focused tab
}

// State queries the kitty instance and returns its window/tab state.
func (k *KittyClient) State() (*KittyState, error) {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath, "ls").Output()
	if err != nil {
		return nil, fmt.Errorf("kitty ls: %w", err)
	}

	var windows []struct {
		ID   int `json:"id"`
		Tabs []struct {
			IsFocused bool `json:"is_focused"`
			Windows   []struct {
				IsFocused bool              `json:"is_focused"`
				Env       map[string]string `json:"env"`
			} `json:"windows"`
		} `json:"tabs"`
	}

	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("parse kitty state: %w", err)
	}

	if len(windows) == 0 {
		return nil, fmt.Errorf("no kitty windows")
	}

	state := &KittyState{WindowID: windows[0].ID}
	for _, tab := range windows[0].Tabs {
		if !tab.IsFocused {
			continue
		}
		for _, w := range tab.Windows {
			if w.IsFocused {
				state.ActiveTabID = w.Env["KITTY_TAB_ID"]
			}
		}
	}

	return state, nil
}

// FocusTab switches to the tab with the given KITTY_TAB_ID.
func (k *KittyClient) FocusTab(tabID string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-tab", "--match", "env:KITTY_TAB_ID="+tabID).Run()
}

// SendText sends keystrokes to the window matched by KITTY_TAB_ID.
func (k *KittyClient) SendText(tabID, text string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"send-text", "--match", "env:KITTY_TAB_ID="+tabID, text).Run()
}
