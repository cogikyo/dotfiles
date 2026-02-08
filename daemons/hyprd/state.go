package main

import (
	"encoding/json"
	"sync"

	"dotfiles/daemons/hyprd/commands"
	"dotfiles/daemons/config"
)

// State tracks workspace information and window management state for commands.
// All access is thread-safe via RWMutex. Serializable to JSON for daemon status endpoint.
type State struct {
	mu sync.RWMutex

	Workspace          int   `json:"workspace"`           // Current active workspace ID
	OccupiedWorkspaces []int `json:"occupied_workspaces"` // Workspace IDs with windows

	Monocle          *commands.MonocleState          `json:"monocle,omitempty"`          // Active monocle mode state
	Hidden           map[string]*commands.HiddenState `json:"hidden,omitempty"`           // Windows in special workspace by address
	DisplacedMasters map[int]string                   `json:"displaced_masters,omitempty"` // Original master window per workspace
	SplitRatio       string                           `json:"split_ratio"`                 // Master/slave split identifier
	Geometry         *commands.MonitorGeometry        `json:"geometry,omitempty"`          // Cached monitor dimensions

	config *config.HyprConfig // Daemon configuration
}

// NewState creates a State with default values and the given configuration.
func NewState(cfg *config.HyprConfig) *State {
	return &State{
		Workspace:          1,
		OccupiedWorkspaces: []int{},
		Hidden:             make(map[string]*commands.HiddenState),
		DisplacedMasters:   make(map[int]string),
		SplitRatio:         "default",
		config:             cfg,
	}
}

// JSON serializes the State to JSON bytes under read lock.
func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s)
}

func (s *State) SetWorkspace(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Workspace = ws
}

func (s *State) GetWorkspace() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Workspace
}

func (s *State) SetOccupied(workspaces []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OccupiedWorkspaces = workspaces
}

// GetOccupied returns a copy of occupied workspace IDs.
func (s *State) GetOccupied() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]int, len(s.OccupiedWorkspaces))
	copy(result, s.OccupiedWorkspaces)
	return result
}

func (s *State) SetMonocle(m *commands.MonocleState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Monocle = m
}

// GetMonocle returns a copy of monocle state, or nil if inactive.
func (s *State) GetMonocle() *commands.MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Monocle == nil {
		return nil
	}
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

func (s *State) AddHidden(h *commands.HiddenState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hidden[h.Address] = h
}

// RemoveHidden deletes and returns the hidden state for a window.
func (s *State) RemoveHidden(addr string) *commands.HiddenState {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.Hidden[addr]
	delete(s.Hidden, addr)
	return h
}

func (s *State) IsHidden(addr string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.Hidden[addr]
	return ok
}

func (s *State) SetSplitRatio(ratio string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SplitRatio = ratio
}

func (s *State) GetSplitRatio() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SplitRatio
}

// SetDisplacedMaster records the original master window for a workspace.
// Pass empty string to clear.
func (s *State) SetDisplacedMaster(ws int, addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if addr == "" {
		delete(s.DisplacedMasters, ws)
	} else {
		s.DisplacedMasters[ws] = addr
	}
}

func (s *State) GetDisplacedMaster(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DisplacedMasters[ws]
}

// ClearWindowState removes all tracked state for a window address.
// Called when a window closes to prevent stale references.
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

func (s *State) SetGeometry(g *commands.MonitorGeometry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Geometry = g
}

// GetGeometry returns a copy of cached monitor geometry, or nil if unset.
func (s *State) GetGeometry() *commands.MonitorGeometry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Geometry == nil {
		return nil
	}
	g := *s.Geometry
	return &g
}

func (s *State) GetConfig() *config.HyprConfig {
	return s.config
}
