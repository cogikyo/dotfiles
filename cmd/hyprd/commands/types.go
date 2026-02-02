// Package commands implements window management commands for hyprd including monocle mode,
// hide/show mode, split ratio cycling, master/slave swapping, workspace switching,
// and session layout management.
package commands

// ================================================================================
// Shared type definitions for daemon state management
// ================================================================================

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

// StateManager interface for commands to interact with daemon state.
// Implemented by daemon.State.
type StateManager interface {
	// Monocle state
	GetMonocle() *MonocleState
	SetMonocle(m *MonocleState)

	// Hidden windows state
	GetHidden() map[string]*HiddenState
	AddHidden(h *HiddenState)
	RemoveHidden(addr string) *HiddenState
	IsHidden(addr string) bool

	// Split ratio
	GetSplitRatio() string
	SetSplitRatio(ratio string)

	// Displaced masters (for swap)
	GetDisplacedMaster(ws int) string
	SetDisplacedMaster(ws int, addr string)
}
