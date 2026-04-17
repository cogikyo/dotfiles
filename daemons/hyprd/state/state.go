// Package state is the thread-safe store for hyprd runtime state.
//
// Tracks workspace occupancy, hidden windows, displaced masters, three-body and monocle layouts, split ratios, active
// sessions, and per-workspace tab memory. Access is guarded by a single RWMutex; state serializes to JSON for
// subscriber event streams.
package state

import (
	"encoding/json"
	"slices"
	"sync"

	"dotfiles/daemons/config"
)

// State holds all hyprd runtime fields plus the mutex guarding them.
//
// Exported fields are JSON-serialized for subscribers; always go through the accessor methods so mu is held.
type State struct {
	mu sync.RWMutex

	Workspace          int                           `json:"workspace"`
	OccupiedWorkspaces []int                         `json:"occupied_workspaces"`
	Hidden             map[string]*HiddenState       `json:"hidden,omitempty"`
	DisplacedMasters   map[int]string                `json:"displaced_masters,omitempty"`
	ThreeBody          map[int]*ThreeBodyState       `json:"three_body,omitempty"`
	ProjectPaths       map[int]string                `json:"project_paths,omitempty"`
	Monocle            map[int]*MonocleState         `json:"monocle,omitempty"`
	SplitRatio         string                        `json:"split_ratio"`
	ActiveSessions     map[int]string                `json:"active_sessions,omitempty"`
	TabMemory          map[int]map[string]*TabMemory `json:"tab_memory,omitempty"`
	config             *config.HyprConfig
}

func NewState(cfg *config.HyprConfig) *State {
	return &State{
		Workspace:          1,
		OccupiedWorkspaces: []int{},
		Hidden:             make(map[string]*HiddenState),
		DisplacedMasters:   make(map[int]string),
		ThreeBody:          make(map[int]*ThreeBodyState),
		ProjectPaths:       make(map[int]string),
		Monocle:            make(map[int]*MonocleState),
		ActiveSessions:     make(map[int]string),
		TabMemory:          make(map[int]map[string]*TabMemory),
		SplitRatio:         "default",
		config:             cfg,
	}
}

// JSON marshals the current State snapshot for subscriber event streams.
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

func (s *State) GetOccupied() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.OccupiedWorkspaces)
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

func (s *State) GetConfig() *config.HyprConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// ReloadConfig swaps in a new HyprConfig from the hot-reload path.
func (s *State) ReloadConfig(cfg *config.HyprConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}
