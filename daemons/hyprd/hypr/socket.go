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

// Client provides communication with the Hyprland compositor via IPC sockets.
type Client struct {
	socketPath string
}

// NewClient creates a Client using the HYPRLAND_INSTANCE_SIGNATURE environment
// variable to locate the socket path.
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

// SocketPath returns the path to the Hyprland command socket.
func (c *Client) SocketPath() string {
	return c.socketPath
}

// EventSocketPath returns the path to the Hyprland event socket.
func (c *Client) EventSocketPath() string {
	return filepath.Join(filepath.Dir(c.socketPath), ".socket2.sock")
}

// Request sends a command to Hyprland and returns the raw response bytes.
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

// Window represents a Hyprland window (client) with its position, size, and metadata.
type Window struct {
	Address   string `json:"address"`
	At        [2]int `json:"at"`
	Size      [2]int `json:"size"`
	Workspace WsRef  `json:"workspace"`
	Floating  bool   `json:"floating"`
	Pinned    bool   `json:"pinned"`
	Class     string `json:"class"`
	Title     string `json:"title"`
	Pid       int    `json:"pid"`
	Mapped    bool   `json:"mapped"`
}

// WsRef represents a workspace reference containing the workspace ID and name.
type WsRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Clients returns all windows currently managed by Hyprland.
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

// ActiveWindow returns the currently focused window.
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

// Dispatch executes a Hyprland dispatcher command and returns an error if
// the command fails.
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

// Monitor represents a Hyprland monitor with its dimensions and active workspace.
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

// Monitors returns all monitors known to Hyprland.
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

// FocusedMonitor returns the currently focused monitor.
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
