// Package commands provides window management commands for the hyprd daemon.
//
// The package implements a command pattern where each command type handles
// a specific window management operation. Commands receive a Hyprland client
// for IPC communication and a StateManager for persistent state tracking.
//
// # Command Types
//
// The following commands are available:
//
//   - Focus: Focuses windows by class or title, unhiding from special workspace if needed
//   - Hide: Toggles window visibility using special workspaces for slave window management
//   - Layout: Opens predefined session layouts with automatic window spawning and arrangement
//   - Monocle: Toggles fullscreen floating mode with origin workspace tracking
//   - Split: Cycles or sets master/slave split ratios (xs, default, lg)
//   - Swap: Swaps windows between master and slave positions with undo support
//   - WS: Switches workspaces with automatic master window focusing
//
// # Architecture
//
// Commands follow a consistent pattern:
//
//	type Command struct {
//	    hypr  *hypr.Client
//	    state StateManager
//	}
//
//	func NewCommand(h *hypr.Client, s StateManager) *Command
//	func (c *Command) Execute(args ...string) (string, error)
//
// The StateManager interface abstracts daemon state, allowing commands to
// track monocle windows, hidden windows, split ratios, and displaced masters
// without direct coupling to the daemon implementation.
//
// # Window Classification
//
// Commands use a master/slave layout model where the leftmost tiled window
// is considered the master. Utility functions in window.go provide:
//
//   - GetTiledWindows: Returns non-floating windows sorted by X position
//   - GetMaster: Returns the master (leftmost) window
//   - GetSlaves: Returns slave windows sorted by Y position
//   - IsMaster: Checks if a window is in master position
//
// # Configuration
//
// Commands read configuration through the StateManager's GetConfig method,
// which provides access to split ratios, monocle settings, style preferences,
// and other daemon configuration values.
package commands

import "dotfiles/cmd/hyprd/config"

// MonocleState tracks a window in monocle mode.
type MonocleState struct {
	Address  string `json:"address"`   // Window address (0x...)
	OriginWS int    `json:"origin_ws"` // Original workspace
	Position string `json:"position"`  // "master" | "0" | "1" | ...
}

// HiddenState tracks a window hidden to special workspace.
type HiddenState struct {
	Address    string `json:"address"`     // Window address (0x...)
	OriginWS   int    `json:"origin_ws"`   // Workspace it came from
	SlaveIndex int    `json:"slave_index"` // Position in slave stack
}

// MonitorGeometry holds computed monitor dimensions for window positioning.
type MonitorGeometry struct {
	Width        int `json:"width"`         // Full monitor width
	Height       int `json:"height"`        // Full monitor height
	ReservedTop  int `json:"reserved_top"`  // Top gap (e.g., for bar)
	ReservedBot  int `json:"reserved_bot"`  // Bottom gap
	ReservedLeft int `json:"reserved_left"` // Left gap
	UsableHeight int `json:"usable_height"` // Height minus reserved areas
	MonocleW     int `json:"monocle_w"`     // Monocle window width
	MonocleH     int `json:"monocle_h"`     // Monocle window height
}

// StateManager defines the interface for commands to interact with daemon state.
// It provides access to monocle state, hidden windows, split ratios, displaced
// masters, monitor geometry, and configuration. The daemon.State type implements
// this interface.
type StateManager interface {
	GetMonocle() *MonocleState
	SetMonocle(m *MonocleState)

	GetHidden() map[string]*HiddenState
	AddHidden(h *HiddenState)
	RemoveHidden(addr string) *HiddenState
	IsHidden(addr string) bool

	GetSplitRatio() string
	SetSplitRatio(ratio string)

	GetDisplacedMaster(ws int) string
	SetDisplacedMaster(ws int, addr string)

	GetGeometry() *MonitorGeometry

	GetConfig() *config.Config
}
