package session

// kitty.go wraps kitty remote-control commands and typed state decoding for tab/session automation.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// KittyClient talks to a kitty instance over its per-PID unix remote-control socket.
type KittyClient struct {
	socketPath string
}

func NewKittyClient(pid int) *KittyClient {
	return &KittyClient{socketPath: fmt.Sprintf("unix:/tmp/kitty-%d", pid)}
}

// KittyState is a snapshot of the first OS window.
//
// ActiveTabID is empty when the focused pane lacks a KITTY_TAB_ID env (e.g. the launcher tab).
type KittyState struct {
	WindowID    int
	ActiveTabID string
}

type KittyOSWindow struct {
	ID        int        `json:"id"`
	IsFocused bool       `json:"is_focused"`
	IsActive  bool       `json:"is_active"`
	Tabs      []KittyTab `json:"tabs"`
}

type KittyTab struct {
	ID        int         `json:"id"`
	IsActive  bool        `json:"is_active"`
	IsFocused bool        `json:"is_focused"`
	Title     string      `json:"title"`
	Windows   []KittyPane `json:"windows"`
}

type KittyPane struct {
	ID                  int               `json:"id"`
	Title               string            `json:"title"`
	IsActive            bool              `json:"is_active"`
	IsFocused           bool              `json:"is_focused"`
	CWD                 string            `json:"cwd"`
	Env                 map[string]string `json:"env"`
	ForegroundProcesses []KittyProcess    `json:"foreground_processes"`
}

type KittyProcess struct {
	Cmdline []string `json:"cmdline"`
	CWD     string   `json:"cwd"`
	PID     int      `json:"pid"`
}

func (k *KittyClient) FullState() ([]KittyOSWindow, error) {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath, "ls").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kitty ls: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var windows []KittyOSWindow
	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("parse kitty state: %w", err)
	}
	return windows, nil
}

// State returns the KittyState of the first OS window.
func (k *KittyClient) State() (*KittyState, error) {
	windows, err := k.FullState()
	if err != nil {
		return nil, err
	}
	if len(windows) == 0 {
		return nil, fmt.Errorf("no kitty windows")
	}
	return stateFromWindow(windows[0]), nil
}

func stateFromWindow(win KittyOSWindow) *KittyState {
	state := &KittyState{WindowID: win.ID}
	for _, tab := range win.Tabs {
		if !tabSelected(tab) {
			continue
		}
		for _, w := range tab.Windows {
			if paneSelected(w) && w.Env != nil {
				state.ActiveTabID = w.Env["KITTY_TAB_ID"]
			}
		}
	}
	return state
}

func (k *KittyClient) FocusTab(tabID string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-tab", "--match", "env:KITTY_TAB_ID="+tabID).Run()
}

func (k *KittyClient) FocusWindow(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-window", "--match", fmt.Sprintf("id:%d", id)).Run()
}

func (k *KittyClient) FocusPane(tabID string, paneIndex int) error {
	if paneIndex < 0 {
		return nil
	}
	windows, err := k.FullState()
	if err != nil {
		return err
	}
	for _, osWindow := range windows {
		for _, tab := range osWindow.Tabs {
			if !tabHasID(tab, tabID) {
				continue
			}
			if paneIndex >= len(tab.Windows) {
				return fmt.Errorf("pane %d not in tab %s", paneIndex, tabID)
			}
			return k.FocusWindow(tab.Windows[paneIndex].ID)
		}
	}
	return nil
}

func (k *KittyClient) SendText(tabID, text string) error {
	if paneID, err := k.activePaneID(tabID); err != nil {
		return err
	} else if paneID != 0 {
		return exec.Command("kitty", "@", "--to", k.socketPath,
			"send-text", "--match", fmt.Sprintf("id:%d", paneID), text).Run()
	}
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"send-text", "--match", "env:KITTY_TAB_ID="+tabID, text).Run()
}

func (k *KittyClient) Launch(args ...string) error {
	cmdArgs := append([]string{"@", "--to", k.socketPath, "launch"}, args...)
	return exec.Command("kitty", cmdArgs...).Run()
}

func (k *KittyClient) GotoLayout(tabID, layout string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"goto-layout", "--match", "env:KITTY_TAB_ID="+tabID, layout).Run()
}

// CloseTab closes the tab with the given KITTY_TAB_ID; a missing tab is a no-op.
func (k *KittyClient) CloseTab(tabID string) error {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath,
		"close-tab", "--match", "env:KITTY_TAB_ID="+tabID).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "No matching") {
			return nil
		}
		return err
	}
	return nil
}

func (k *KittyClient) CloseTabByNumericID(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"close-tab", "--match", fmt.Sprintf("id:%d", id)).Run()
}

func (k *KittyClient) MoveTabBackward() error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"action", "move_tab_backward").Run()
}

// TabIndex returns the position of tabID in the first OS window, or -1 if absent.
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
			if pane.Env != nil && pane.Env["KITTY_TAB_ID"] == tabID {
				return i, nil
			}
		}
	}
	return -1, nil
}

func activePaneIndex(win KittyOSWindow, tabID string) int {
	for _, tab := range win.Tabs {
		if !tabSelected(tab) || !tabHasID(tab, tabID) {
			continue
		}
		for i, pane := range tab.Windows {
			if paneSelected(pane) {
				return i
			}
		}
	}
	return -1
}

func (k *KittyClient) activePaneID(tabID string) (int, error) {
	windows, err := k.FullState()
	if err != nil {
		return 0, err
	}
	for _, win := range windows {
		for _, tab := range win.Tabs {
			if !tabHasID(tab, tabID) {
				continue
			}
			for _, pane := range tab.Windows {
				if paneSelected(pane) {
					return pane.ID, nil
				}
			}
			if len(tab.Windows) > 0 {
				return tab.Windows[0].ID, nil
			}
		}
	}
	return 0, nil
}

func tabHasID(tab KittyTab, tabID string) bool {
	for _, pane := range tab.Windows {
		if pane.Env != nil && pane.Env["KITTY_TAB_ID"] == tabID {
			return true
		}
	}
	return false
}

func tabSelected(tab KittyTab) bool {
	return tab.IsFocused || tab.IsActive
}

func paneSelected(pane KittyPane) bool {
	return pane.IsFocused || pane.IsActive
}
