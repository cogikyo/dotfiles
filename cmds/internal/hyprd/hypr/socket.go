// Package hypr wraps Hyprland IPC socket communication for hyprd commands.
//
// It exposes a thin typed client over Hyprland's command and event sockets.
//
// Responsibilities:
// - Resolve command and event socket paths from runtime environment variables.
// - Send requests and dispatcher commands over Unix sockets.
// - Decode typed window and monitor payloads from Hyprland JSON endpoints.
package hypr

// socket.go defines the IPC client transport plus typed helpers for clients, active window, and monitors.

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
	socketPath string
}

// NewClient resolves the command socket from HYPRLAND_INSTANCE_SIGNATURE.
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

// SocketPath returns the path to the command socket.
func (c *Client) SocketPath() string {
	return c.socketPath
}

// EventSocketPath returns the path to the event-streaming socket.
func (c *Client) EventSocketPath() string {
	return filepath.Join(filepath.Dir(c.socketPath), ".socket2.sock")
}

// Request sends a command and returns the raw response.
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

// Window mirrors the JSON from `hyprctl -j clients`.
type Window struct {
	Address        string `json:"address"`
	At             [2]int `json:"at"`
	Size           [2]int `json:"size"`
	Workspace      WsRef  `json:"workspace"`
	Floating       bool   `json:"floating"`
	Pinned         bool   `json:"pinned"`
	Class          string `json:"class"`
	Title          string `json:"title"`
	InitialTitle   string `json:"initialTitle"`
	Pid            int    `json:"pid"`
	Mapped         bool   `json:"mapped"`
	FocusHistoryID int    `json:"focusHistoryID"`
}

// WsRef is a workspace id/name pair embedded in other Hyprland types.
type WsRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Clients returns all windows from `hyprctl -j clients`.
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

// ActiveWindow returns the focused window, or nil when nothing has focus.
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
		return nil, nil
	}
	return &w, nil
}

// Dispatch executes a Hyprland dispatcher command.
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

// Monitor mirrors the JSON from `hyprctl -j monitors`.
//
// TODO: reserved tags are wrong; Hyprland returns `reserved` as a [L,T,R,B] array.
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

// Monitors returns all monitors from `hyprctl -j monitors`.
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
