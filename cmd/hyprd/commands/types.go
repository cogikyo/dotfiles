// Package commands implements window management commands for hyprd including monocle mode,
// pseudo-master mode, split ratio cycling, master/slave swapping, workspace switching,
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

// PseudoState tracks a window in pseudo-master mode.
type PseudoState struct {
	Address    string `json:"address"`
	SlaveIndex int    `json:"slave_index"`
}

// StateManager interface for commands to interact with daemon state.
// Implemented by daemon.State.
type StateManager interface {
	// Monocle state
	GetMonocle() *MonocleState
	SetMonocle(m *MonocleState)

	// Pseudo-master state
	GetPseudo() *PseudoState
	SetPseudo(p *PseudoState)

	// Split ratio
	GetSplitRatio() string
	SetSplitRatio(ratio string)

	// Displaced masters (for swap)
	GetDisplacedMaster(ws int) string
	SetDisplacedMaster(ws int, addr string)
}
