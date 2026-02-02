package main

// Thread-safe daemon state with RWMutex protection

import (
	"encoding/json"
	"sync"

	"dotfiles/cmd/hyprd/commands"
)

// State holds all daemon state with thread-safe access.
type State struct {
	mu sync.RWMutex

	// Workspace tracking
	Workspace          int   `json:"workspace"`
	OccupiedWorkspaces []int `json:"occupied_workspaces"`

	// Monocle mode: window floated to WS6 for focus
	Monocle *commands.MonocleState `json:"monocle,omitempty"`

	// Hidden windows: slaves sent to special:hiddenSlaves workspace
	Hidden map[string]*commands.HiddenState `json:"hidden,omitempty"`

	// Displaced masters: original master saved when slave swapped to master
	DisplacedMasters map[int]string `json:"displaced_masters,omitempty"`

	// Split ratio state
	SplitRatio string `json:"split_ratio"` // "xs" | "default" | "lg"
}

// NewState creates initialized state.
func NewState() *State {
	return &State{
		Workspace:          1,
		OccupiedWorkspaces: []int{},
		Hidden:             make(map[string]*commands.HiddenState),
		DisplacedMasters:   make(map[int]string),
		SplitRatio:         "default",
	}
}

// JSON returns state as JSON bytes.
func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

// SetWorkspace updates current workspace.
func (s *State) SetWorkspace(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Workspace = ws
}

// GetWorkspace returns current workspace.
func (s *State) GetWorkspace() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Workspace
}

// SetOccupied updates the list of occupied workspaces.
func (s *State) SetOccupied(workspaces []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OccupiedWorkspaces = workspaces
}

// GetOccupied returns a copy of occupied workspaces (thread-safe).
func (s *State) GetOccupied() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]int, len(s.OccupiedWorkspaces))
	copy(result, s.OccupiedWorkspaces)
	return result
}

// SetMonocle sets or clears monocle state.
func (s *State) SetMonocle(m *commands.MonocleState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Monocle = m
}

// GetMonocle returns current monocle state.
func (s *State) GetMonocle() *commands.MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Monocle == nil {
		return nil
	}
	// Return copy to avoid data races
	m := *s.Monocle
	return &m
}

// GetHidden returns a copy of all hidden window states.
func (s *State) GetHidden() map[string]*commands.HiddenState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*commands.HiddenState)
	for k, v := range s.Hidden {
		copy := *v
		result[k] = &copy
	}
	return result
}

// AddHidden adds a window to the hidden state.
func (s *State) AddHidden(h *commands.HiddenState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hidden[h.Address] = h
}

// RemoveHidden removes and returns hidden state for an address.
func (s *State) RemoveHidden(addr string) *commands.HiddenState {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.Hidden[addr]
	delete(s.Hidden, addr)
	return h
}

// IsHidden checks if a window address is hidden.
func (s *State) IsHidden(addr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.Hidden[addr]
	return ok
}

// SetSplitRatio updates split ratio.
func (s *State) SetSplitRatio(ratio string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SplitRatio = ratio
}

// GetSplitRatio returns current split ratio.
func (s *State) GetSplitRatio() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SplitRatio
}

// SetDisplacedMaster records original master for a workspace.
func (s *State) SetDisplacedMaster(ws int, addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if addr == "" {
		delete(s.DisplacedMasters, ws)
	} else {
		s.DisplacedMasters[ws] = addr
	}
}

// GetDisplacedMaster returns displaced master for workspace, if any.
func (s *State) GetDisplacedMaster(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DisplacedMasters[ws]
}

// ClearWindowState removes any monocle/hidden state for a window address.
// Called when a window closes.
func (s *State) ClearWindowState(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Monocle != nil && s.Monocle.Address == addr {
		s.Monocle = nil
	}
	delete(s.Hidden, addr)
	// Clean displaced masters referencing this address
	for ws, a := range s.DisplacedMasters {
		if a == addr {
			delete(s.DisplacedMasters, ws)
		}
	}
}
