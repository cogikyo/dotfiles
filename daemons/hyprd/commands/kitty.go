package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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

// KittyOSWindow represents a kitty OS-level window containing tabs.
type KittyOSWindow struct {
	ID   int        `json:"id"`
	Tabs []KittyTab `json:"tabs"`
}

// KittyTab represents a single tab within a kitty window.
type KittyTab struct {
	ID        int         `json:"id"`
	IsFocused bool        `json:"is_focused"`
	Title     string      `json:"title"`
	Windows   []KittyPane `json:"windows"`
}

// KittyPane represents a pane (window) within a kitty tab.
type KittyPane struct {
	IsFocused bool              `json:"is_focused"`
	CWD       string            `json:"cwd"`
	Env       map[string]string `json:"env"`
}

// FullState queries the complete kitty state including all tabs and their env vars.
func (k *KittyClient) FullState() ([]KittyOSWindow, error) {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath, "ls").Output()
	if err != nil {
		return nil, fmt.Errorf("kitty ls: %w", err)
	}

	var windows []KittyOSWindow
	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("parse kitty state: %w", err)
	}

	return windows, nil
}

// State queries the kitty instance and returns its window/tab state.
func (k *KittyClient) State() (*KittyState, error) {
	windows, err := k.FullState()
	if err != nil {
		return nil, err
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

// Launch opens a new tab in the kitty instance with the given arguments.
func (k *KittyClient) Launch(args ...string) error {
	cmdArgs := append([]string{"@", "--to", k.socketPath, "launch"}, args...)
	return exec.Command("kitty", cmdArgs...).Run()
}

// CloseTab closes the tab matched by KITTY_TAB_ID env var.
// Returns nil if no matching tab exists.
func (k *KittyClient) CloseTab(tabID string) error {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath,
		"close-tab", "--match", "env:KITTY_TAB_ID="+tabID).CombinedOutput()
	if err != nil {
		// "No matching" means tab doesn't exist — not an error
		if strings.Contains(string(out), "No matching") {
			return nil
		}
		return err
	}
	return nil
}

// CloseTabByNumericID closes a tab by its kitty internal tab ID.
func (k *KittyClient) CloseTabByNumericID(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"close-tab", "--match", fmt.Sprintf("id:%d", id)).Run()
}

// MoveTabBackward moves the currently focused tab one position to the left.
func (k *KittyClient) MoveTabBackward() error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"action", "move_tab_backward").Run()
}

// TabIndex returns the index of the tab with the given KITTY_TAB_ID, or -1 if not found.
func (k *KittyClient) TabIndex(tabID string) (int, error) {
	windows, err := k.FullState()
	if err != nil {
		return -1, err
	}
	if len(windows) == 0 {
		return -1, nil
	}
	for i, tab := range windows[0].Tabs {
		for _, pane := range tab.Windows {
			if pane.Env["KITTY_TAB_ID"] == tabID {
				return i, nil
			}
		}
	}
	return -1, nil
}
