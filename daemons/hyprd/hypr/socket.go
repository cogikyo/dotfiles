// Package hypr provides Hyprland IPC socket communication.
//
// It connects to the Hyprland compositor via Unix domain sockets located at
// $XDG_RUNTIME_DIR/hypr/$HYPRLAND_INSTANCE_SIGNATURE/. The command socket
// (.socket.sock) accepts text commands and returns JSON or plain text responses.
// The event socket (.socket2.sock) streams newline-delimited events.
//
// Commands use the format "j/command" for JSON output or "command args" for
// dispatcher actions.
package hypr

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Client communicates with Hyprland via Unix domain sockets for commands and events.
type Client struct {
	socketPath string // path to .socket.sock
}

// NewClient creates a Client by resolving the socket path from HYPRLAND_INSTANCE_SIGNATURE.
// Returns an error if the environment variable is unset or the socket doesn't exist.
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

// SocketPath returns the command socket path (.socket.sock).
func (c *Client) SocketPath() string {
	return c.socketPath
}

// EventSocketPath returns the event socket path (.socket2.sock).
func (c *Client) EventSocketPath() string {
	return filepath.Join(filepath.Dir(c.socketPath), ".socket2.sock")
}

// Request sends a command to Hyprland and returns the raw response.
// The command should be formatted as "j/command" for JSON or "dispatch args" for actions.
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

// Window represents a Hyprland window with geometry, workspace assignment, and metadata.
type Window struct {
	Address   string `json:"address"` // unique window handle
	At        [2]int `json:"at"`      // [x, y] position
	Size      [2]int `json:"size"`    // [width, height]
	Workspace WsRef  `json:"workspace"`
	Floating  bool   `json:"floating"`
	Pinned    bool   `json:"pinned"`
	Class     string `json:"class"`
	Title     string `json:"title"`
	Pid       int    `json:"pid"`
	Mapped    bool   `json:"mapped"` // whether window is visible
}

// WsRef is a workspace reference used in window and monitor queries.
type WsRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"` // e.g., "1", "2", or custom name
}

// Clients queries all windows currently managed by Hyprland.
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

// ActiveWindow returns the currently focused window, or nil if no window has focus.
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
		return nil, nil // No active window
	}
	return &w, nil
}

// Dispatch executes a Hyprland dispatcher command (e.g., "movefocus l", "workspace 1").
// Returns an error if Hyprland responds with anything other than "ok".
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

// Monitor represents a physical or virtual display with geometry and workspace info.
type Monitor struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`          // e.g., "DP-1", "HDMI-A-1"
	Width         int    `json:"width"`         // resolution width
	Height        int    `json:"height"`        // resolution height
	X             int    `json:"x"`             // position in compositor space
	Y             int    `json:"y"`             // position in compositor space
	Focused       bool   `json:"focused"`       // currently active monitor
	ActiveWS      WsRef  `json:"activeWorkspace"`
	ReservedTop   int    `json:"reserved"`      // reserved pixels at top (bars, etc.)
	ReservedRight int    `json:"reservedB"`     // reserved pixels at right
}

// Monitors queries all displays currently known to Hyprland.
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

// FocusedMonitor returns the currently focused monitor, falling back to the first
// monitor if none are marked as focused. Returns nil only if no monitors exist.
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
