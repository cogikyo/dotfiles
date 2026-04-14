package session

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

type KittyState struct {
	WindowID    int
	ActiveTabID string
}

type KittyOSWindow struct {
	ID   int        `json:"id"`
	Tabs []KittyTab `json:"tabs"`
}

type KittyTab struct {
	ID        int         `json:"id"`
	IsFocused bool        `json:"is_focused"`
	Title     string      `json:"title"`
	Windows   []KittyPane `json:"windows"`
}

type KittyPane struct {
	ID                  int               `json:"id"`
	Title               string            `json:"title"`
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
			if w.IsFocused && w.Env != nil {
				state.ActiveTabID = w.Env["KITTY_TAB_ID"]
			}
		}
	}
	return state, nil
}

func (k *KittyClient) FocusTab(tabID string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-tab", "--match", "env:KITTY_TAB_ID="+tabID).Run()
}

func (k *KittyClient) FocusWindow(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-window", "--match", fmt.Sprintf("id:%d", id)).Run()
}

func (k *KittyClient) SendText(tabID, text string) error {
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
