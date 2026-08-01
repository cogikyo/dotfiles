// Package kitty owns Kitty transport, tab profiles, refresh, selection, and host switching for hyprd.
package kitty

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Client talks to one Kitty instance through its per-PID Unix remote-control socket.
type Client struct {
	socketPath string
}

func NewClient(pid int) *Client {
	return &Client{socketPath: fmt.Sprintf("unix:/tmp/kitty-%d", pid)}
}

type OSWindow struct {
	ID        int   `json:"id"`
	IsFocused bool  `json:"is_focused"`
	IsActive  bool  `json:"is_active"`
	Tabs      []Tab `json:"tabs"`
}

type Tab struct {
	ID             int            `json:"id"`
	IsActive       bool           `json:"is_active"`
	IsFocused      bool           `json:"is_focused"`
	Title          string         `json:"title"`
	Layout         string         `json:"layout"`
	LayoutOpts     map[string]any `json:"layout_opts"`
	EnabledLayouts []string       `json:"enabled_layouts"`
	Windows        []Pane         `json:"windows"`
}

type Pane struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	IsActive  bool   `json:"is_active"`
	IsFocused bool   `json:"is_focused"`
	CWD       string `json:"cwd"`
	// Cmdline is the window's direct child argv from Kitty ls (0.48.1 schema).
	// For managed remote panes this is the stable local launch shape (kitten ssh …), not the tty foreground which often settles to plain ssh.
	Cmdline             []string          `json:"cmdline"`
	Env                 map[string]string `json:"env"`
	UserVars            map[string]string `json:"user_vars"`
	ForegroundProcesses []Process         `json:"foreground_processes"`
}

type Process struct {
	Cmdline []string `json:"cmdline"`
	CWD     string   `json:"cwd"`
	PID     int      `json:"pid"`
}

func (k *Client) FullState() ([]OSWindow, error) {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath, "ls").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kitty ls: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var windows []OSWindow
	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("parse kitty state: %w", err)
	}
	return windows, nil
}

func (k *Client) FocusTab(tabID string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-tab", "--match", "env:KITTY_TAB_ID="+tabID).Run()
}

func (k *Client) gotoTab(index int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"action", "goto_tab", fmt.Sprintf("%d", index)).Run()
}

func (k *Client) FocusWindow(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-window", "--match", fmt.Sprintf("id:%d", id)).Run()
}

func (k *Client) launch(args ...string) error {
	_, err := k.launchID(args...)
	return err
}

func (k *Client) launchID(args ...string) (int, error) {
	cmdArgs := append([]string{"@", "--to", k.socketPath, "launch"}, args...)
	out, err := exec.Command("kitty", cmdArgs...).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("kitty launch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	id, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse launched kitty window id %q: %w", strings.TrimSpace(string(out)), err)
	}
	return id, nil
}

func (k *Client) gotoLayout(tabID, layout string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"goto-layout", "--match", "env:KITTY_TAB_ID="+tabID, layout).Run()
}

func (k *Client) gotoLayoutByNumericID(id int, layout string) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"goto-layout", "--match", fmt.Sprintf("id:%d", id), layout).Run()
}

// closeTab closes the tab with the given KITTY_TAB_ID; a missing tab is a no-op.
func (k *Client) closeTab(tabID string) error {
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

func (k *Client) closeTabByNumericID(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"close-tab", "--match", fmt.Sprintf("id:%d", id)).Run()
}

func (k *Client) closeTabsByNumericIDs(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	terms := make([]string, len(ids))
	for i, id := range ids {
		terms[i] = fmt.Sprintf("id:%d", id)
	}
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"close-tab", "--match", strings.Join(terms, " or ")).Run()
}

func (k *Client) focusTabByNumericID(id int) error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"focus-tab", "--match", fmt.Sprintf("id:%d", id)).Run()
}

func (k *Client) notify(args ...string) error {
	cmdArgs := []string{"@", "--to", k.socketPath, "kitten", "--match", "state:focused", "notify"}
	cmdArgs = append(cmdArgs, args...)
	return exec.Command("kitty", cmdArgs...).Run()
}

func focusedOSWindow(windows []OSWindow) (OSWindow, error) {
	var focused *OSWindow
	for i := range windows {
		if !windows[i].IsFocused {
			continue
		}
		if focused != nil {
			return OSWindow{}, fmt.Errorf("multiple focused kitty OS windows")
		}
		focused = &windows[i]
	}
	if focused == nil {
		return OSWindow{}, fmt.Errorf("no focused kitty OS window")
	}
	return *focused, nil
}

func (k *Client) moveTabBackward() error {
	return exec.Command("kitty", "@", "--to", k.socketPath,
		"action", "move_tab_backward").Run()
}

// tabIndex returns the position of tabID in the first OS window, or -1 if absent.
func (k *Client) tabIndex(tabID string) (int, error) {
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

func tabHasID(tab Tab, tabID string) bool {
	for _, pane := range tab.Windows {
		if pane.Env != nil && pane.Env["KITTY_TAB_ID"] == tabID {
			return true
		}
	}
	return false
}

func tabSelected(tab Tab) bool {
	return tab.IsFocused || tab.IsActive
}

func paneSelected(pane Pane) bool {
	return pane.IsFocused || pane.IsActive
}
