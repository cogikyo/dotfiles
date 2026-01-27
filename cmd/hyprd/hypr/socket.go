// Package hypr provides Hyprland IPC socket communication.
package hypr

// ================================================================================
// Hyprland IPC client for commands, queries, and event subscription
// ================================================================================

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Client connects to the Hyprland IPC socket.
type Client struct {
	socketPath string
}

// NewClient creates a Hyprland client from environment.
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

// Request sends a request to Hyprland and returns the response.
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

// Window represents a Hyprland window/client.
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

// WsRef is a workspace reference in window data.
type WsRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Clients queries all Hyprland windows.
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

// Dispatch sends a dispatcher command to Hyprland.
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
