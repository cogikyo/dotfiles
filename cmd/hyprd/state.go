package main

import (
	"encoding/json"
	"sync"

	"dotfiles/cmd/hyprd/commands"
	"dotfiles/cmd/hyprd/config"
)

// State holds all daemon state with thread-safe access via RWMutex.
type State struct {
	mu sync.RWMutex

	Workspace          int   `json:"workspace"`
	OccupiedWorkspaces []int `json:"occupied_workspaces"`

	Monocle          *commands.MonocleState          `json:"monocle,omitempty"`
	Hidden           map[string]*commands.HiddenState `json:"hidden,omitempty"`
	DisplacedMasters map[int]string                   `json:"displaced_masters,omitempty"`
	SplitRatio       string                           `json:"split_ratio"`
	Geometry         *commands.MonitorGeometry        `json:"geometry,omitempty"`

	config *config.Config
}

// NewState creates a State initialized with the given configuration.
func NewState(cfg *config.Config) *State {
	return &State{
		Workspace:          1,
		OccupiedWorkspaces: []int{},
		Hidden:             make(map[string]*commands.HiddenState),
		DisplacedMasters:   make(map[int]string),
		SplitRatio:         "default",
		config:             cfg,
	}
}

// JSON returns the State serialized as JSON bytes.
func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

// SetWorkspace updates the current workspace ID.
func (s *State) SetWorkspace(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Workspace = ws
}

// GetWorkspace returns the current workspace ID.
func (s *State) GetWorkspace() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Workspace
}

// SetOccupied updates the list of workspace IDs that contain windows.
func (s *State) SetOccupied(workspaces []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OccupiedWorkspaces = workspaces
}

// GetOccupied returns a copy of the occupied workspace IDs.
func (s *State) GetOccupied() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]int, len(s.OccupiedWorkspaces))
	copy(result, s.OccupiedWorkspaces)
	return result
}

// SetMonocle sets or clears the monocle mode state.
func (s *State) SetMonocle(m *commands.MonocleState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Monocle = m
}

// GetMonocle returns a copy of the current monocle state, or nil if inactive.
func (s *State) GetMonocle() *commands.MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Monocle == nil {
		return nil
	}
	m := *s.Monocle
	return &m
}

// GetHidden returns a copy of the hidden window states by address.
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

// AddHidden records a window as hidden in the special workspace.
func (s *State) AddHidden(h *commands.HiddenState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hidden[h.Address] = h
}

// RemoveHidden removes and returns the hidden state for a window address.
func (s *State) RemoveHidden(addr string) *commands.HiddenState {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.Hidden[addr]
	delete(s.Hidden, addr)
	return h
}

// IsHidden reports whether a window address is currently hidden.
func (s *State) IsHidden(addr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.Hidden[addr]
	return ok
}

// SetSplitRatio updates the master/slave split ratio identifier.
func (s *State) SetSplitRatio(ratio string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SplitRatio = ratio
}

// GetSplitRatio returns the current split ratio identifier.
func (s *State) GetSplitRatio() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SplitRatio
}

// SetDisplacedMaster records the original master window address for a workspace.
func (s *State) SetDisplacedMaster(ws int, addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if addr == "" {
		delete(s.DisplacedMasters, ws)
	} else {
		s.DisplacedMasters[ws] = addr
	}
}

// GetDisplacedMaster returns the displaced master address for a workspace.
func (s *State) GetDisplacedMaster(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DisplacedMasters[ws]
}

// ClearWindowState removes monocle, hidden, and displaced master state for
// a window address. This should be called when a window closes.
func (s *State) ClearWindowState(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Monocle != nil && s.Monocle.Address == addr {
		s.Monocle = nil
	}
	delete(s.Hidden, addr)
	for ws, a := range s.DisplacedMasters {
		if a == addr {
			delete(s.DisplacedMasters, ws)
		}
	}
}

// SetGeometry updates the cached monitor geometry.
func (s *State) SetGeometry(g *commands.MonitorGeometry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Geometry = g
}

// GetGeometry returns a copy of the current monitor geometry, or nil if unset.
func (s *State) GetGeometry() *commands.MonitorGeometry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Geometry == nil {
		return nil
	}
	g := *s.Geometry
	return &g
}

// GetConfig returns the daemon configuration loaded at startup.
func (s *State) GetConfig() *config.Config {
	return s.config
}
