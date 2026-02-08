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

import "dotfiles/daemons/config"

// MonocleState tracks a fullscreen floating window, storing its origin workspace
// and position so it can be restored when toggled off.
type MonocleState struct {
	Address  string `json:"address"`   // Window address (0x...)
	OriginWS int    `json:"origin_ws"` // Workspace to restore to
	Position string `json:"position"`  // Position to restore: "master" or slave index "0", "1", etc.
}

// HiddenState tracks a window moved to the special workspace for temporary hiding,
// storing its origin so it can be restored to the same workspace and slave position.
type HiddenState struct {
	Address    string `json:"address"`     // Window address (0x...)
	OriginWS   int    `json:"origin_ws"`   // Workspace to restore to
	SlaveIndex int    `json:"slave_index"` // Slave position to restore to
}

// MonitorGeometry holds computed screen dimensions for window positioning and monocle sizing.
// Computed by ComputeGeometry from raw dimensions, reserved areas, and monocle ratios.
type MonitorGeometry struct {
	Width        int `json:"width"`         // Full monitor width in pixels
	Height       int `json:"height"`        // Full monitor height in pixels
	ReservedTop  int `json:"reserved_top"`  // Top reserved area (e.g., status bar)
	ReservedBot  int `json:"reserved_bot"`  // Bottom reserved area
	ReservedLeft int `json:"reserved_left"` // Left reserved area
	UsableHeight int `json:"usable_height"` // Height minus top and bottom reserved areas
	MonocleW     int `json:"monocle_w"`     // Monocle window width (monitor width * ratio)
	MonocleH     int `json:"monocle_h"`     // Monocle window height (usable height * ratio)
}

// StateManager abstracts daemon state access for commands, providing monocle tracking,
// hidden window management, split ratios, displaced master tracking, monitor geometry,
// and configuration. Implemented by daemon.State.
type StateManager interface {
	GetMonocle() *MonocleState
	SetMonocle(m *MonocleState)

	GetHidden() map[string]*HiddenState // Returns all hidden windows by address
	AddHidden(h *HiddenState)
	RemoveHidden(addr string) *HiddenState // Returns the removed state
	IsHidden(addr string) bool

	GetSplitRatio() string // Returns "xs", "default", or "lg"
	SetSplitRatio(ratio string)

	GetDisplacedMaster(ws int) string    // Returns window address of displaced master on workspace
	SetDisplacedMaster(ws int, addr string)

	GetGeometry() *MonitorGeometry

	GetConfig() *config.HyprConfig
}
