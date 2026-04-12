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
// track hidden windows, split ratios, and displaced masters
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
// which provides access to split ratios, style preferences,
// and other daemon configuration values.
package commands

import "dotfiles/daemons/config"

// HiddenState tracks a window moved to the special workspace for temporary hiding,
// storing its origin so it can be restored to the same workspace and slave position.
type HiddenState struct {
	Address    string `json:"address"`     // Window address (0x...)
	OriginWS   int    `json:"origin_ws"`   // Workspace to restore to
	SlaveIndex int    `json:"slave_index"` // Slave position to restore to
}

// ThreeBodyState tracks a three-window layout where only master + one slave are visible.
// The third window is hidden in a shadow workspace, swapped in on demand.
type ThreeBodyState struct {
	Master string `json:"master"` // Address of master window (always visible, left)
	Active string `json:"active"` // Address of visible slave (right, full height)
	Shadow string `json:"shadow"` // Address of hidden slave (in shadow workspace)
}

// MonocleWindow tracks a single window displaced during monocle mode.
type MonocleWindow struct {
	Address  string `json:"address"`
	OriginWS int    `json:"origin_ws"`
}

// MonocleState tracks all windows displaced from a workspace during monocle mode,
// plus the focused window that was floated and resized.
type MonocleState struct {
	Focused        string          `json:"focused"`                  // Address of the monocled (floated) window
	Master         string          `json:"master"`                   // Address of the original master window
	Windows        []MonocleWindow `json:"windows"`
	SavedThreeBody *ThreeBodyState `json:"saved_three_body,omitempty"` // Full three-body state to restore
}

// StateManager abstracts daemon state access for commands, providing
// hidden window management, split ratios, displaced master tracking,
// and configuration. Implemented by daemon.State.
type StateManager interface {
	GetHidden() map[string]*HiddenState // Returns all hidden windows by address
	AddHidden(h *HiddenState)
	RemoveHidden(addr string) *HiddenState // Returns the removed state
	IsHidden(addr string) bool

	GetSplitRatio() string // Returns "xs", "default", or "lg"
	SetSplitRatio(ratio string)

	GetDisplacedMaster(ws int) string    // Returns window address of displaced master on workspace
	SetDisplacedMaster(ws int, addr string)

	GetThreeBody(ws int) *ThreeBodyState          // Returns three-body state for workspace, or nil
	SetThreeBody(ws int, tb *ThreeBodyState)       // Sets three-body state for workspace
	ClearThreeBody(ws int)                         // Removes three-body state for workspace
	AllThreeBody() map[int]*ThreeBodyState         // Returns copy of all three-body states

	GetProjectPath(ws int) string                  // Returns project root for workspace, or ""
	SetProjectPath(ws int, path string)             // Sets project root for workspace

	GetMonocle(ws int) *MonocleState               // Returns monocle state for workspace, or nil
	SetMonocle(ws int, ms *MonocleState)            // Sets monocle state for workspace
	ClearMonocle(ws int)                            // Removes monocle state for workspace
	AllMonocle() map[int]*MonocleState              // Returns copy of all monocle states
	HasAnyMonocle() bool                            // Returns true if any workspace has monocle active

	GetConfig() *config.HyprConfig
}
