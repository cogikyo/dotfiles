// Package hypr provides Hyprland IPC socket communication.
//
// Sockets live at $XDG_RUNTIME_DIR/hypr/$HYPRLAND_INSTANCE_SIGNATURE/.
// `.socket.sock` accepts request/response text commands; `.socket2.sock` streams newline-delimited events.
// Request format: "j/command" for JSON output, "dispatch args" for dispatcher actions, bare command for plain text.
package hypr

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ client & transport                                                           │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Client communicates with Hyprland via its Unix sockets.
type Client struct {
	socketPath string // .socket.sock
}

// NewClient resolves the command socket from HYPRLAND_INSTANCE_SIGNATURE.
// Errors if the env var is unset (Hyprland not running) or the socket is missing.
// XDG_RUNTIME_DIR falls back to /run/user/$UID.
func NewClient() (*Client, error) {
	sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if sig == "" {
		return nil, fmt.Errorf("HYPRLAND_INSTANCE_SIGNATURE not set — is Hyprland running?")
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	socketPath := filepath.Join(runtimeDir, "hypr", sig, ".socket.sock")
	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("socket not found: %s", socketPath)
	}

	return &Client{socketPath: socketPath}, nil
}

func (c *Client) SocketPath() string {
	return c.socketPath
}

func (c *Client) EventSocketPath() string {
	return filepath.Join(filepath.Dir(c.socketPath), ".socket2.sock")
}

// Request sends a command and returns the raw response.
//
// Response is read in a single 64KiB Read — large outputs (many clients, etc.) may be truncated.
func (c *Client) Request(command string) ([]byte, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial hyprland: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(command)); err != nil {
		return nil, fmt.Errorf("write command: %w", err)
	}

	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return buf[:n], nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ windows                                                                      │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Window mirrors the JSON returned by `hyprctl -j clients`.
type Window struct {
	Address        string `json:"address"`
	At             [2]int `json:"at"`   // [x, y]
	Size           [2]int `json:"size"` // [w, h]
	Workspace      WsRef  `json:"workspace"`
	Floating       bool   `json:"floating"`
	Pinned         bool   `json:"pinned"`
	Class          string `json:"class"`
	Title          string `json:"title"`
	InitialTitle   string `json:"initialTitle"`
	Pid            int    `json:"pid"`
	Mapped         bool   `json:"mapped"`
	FocusHistoryID int    `json:"focusHistoryID"` // 0 = active, 1 = previous, ...
}

// WsRef is a workspace reference embedded in window and monitor queries.
type WsRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (c *Client) Clients() ([]Window, error) {
	data, err := c.Request("j/clients")
	if err != nil {
		return nil, err
	}

	var windows []Window
	if err := json.Unmarshal(data, &windows); err != nil {
		return nil, fmt.Errorf("parse clients: %w", err)
	}
	return windows, nil
}

// ActiveWindow returns the focused window, or nil if none has focus.
func (c *Client) ActiveWindow() (*Window, error) {
	data, err := c.Request("j/activewindow")
	if err != nil {
		return nil, err
	}

	var w Window
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parse activewindow: %w", err)
	}

	if w.Address == "" {
		// Hyprland returns `{}` (not an error) when no window is active.
		return nil, nil
	}
	return &w, nil
}

// Dispatch executes a Hyprland dispatcher command (e.g. "movefocus l", "workspace 1").
// Errors when the response isn't "ok".
func (c *Client) Dispatch(args string) error {
	resp, err := c.Request("dispatch " + args)
	if err != nil {
		return err
	}
	if string(resp) != "ok" {
		return fmt.Errorf("dispatch failed: %s", string(resp))
	}
	return nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ monitors                                                                     │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Monitor mirrors the JSON returned by `hyprctl -j monitors`.
//
// FIXME: ReservedTop/ReservedRight JSON tags look wrong — Hyprland returns `reserved` as a [L,T,R,B] array, not
// scalar fields named `reserved`/`reservedB`.
type Monitor struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	Focused       bool   `json:"focused"`
	ActiveWS      WsRef  `json:"activeWorkspace"`
	ReservedTop   int    `json:"reserved"`
	ReservedRight int    `json:"reservedB"`
}

func (c *Client) Monitors() ([]Monitor, error) {
	data, err := c.Request("j/monitors")
	if err != nil {
		return nil, err
	}

	var monitors []Monitor
	if err := json.Unmarshal(data, &monitors); err != nil {
		return nil, fmt.Errorf("parse monitors: %w", err)
	}
	return monitors, nil
}

// FocusedMonitor returns the focused monitor, falling back to monitors[0].
// Returns (nil, nil) only when no monitors exist.
func (c *Client) FocusedMonitor() (*Monitor, error) {
	monitors, err := c.Monitors()
	if err != nil {
		return nil, err
	}

	for _, m := range monitors {
		if m.Focused {
			return &m, nil
		}
	}

	if len(monitors) > 0 {
		return &monitors[0], nil
	}
	return nil, nil
}
