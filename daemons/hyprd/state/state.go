// Package state stores hyprd runtime state behind a single mutex.
//
// Responsibilities:
// - Track workspace, layout, and window-placement runtime data.
// - Persist per-workspace session selection and tab memory.
// - Expose safe copy-on-read helpers for concurrent command handlers.
package state

// state.go defines the core State container plus config wiring and full-state serialization/restoration.

import (
	"encoding/json"
	"slices"
	"sync"

	"dotfiles/daemons/config"
)

// State holds all hyprd runtime fields, guarded by a single RWMutex.
//
// Exported fields are JSON-serialized for subscriber event streams; always use accessor methods.
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

// JSON returns the full state snapshot as JSON for subscriber event streams.
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

// Restore loads previously serialized state while preserving the current config.
func (s *State) Restore(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var snap State
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	s.Workspace = snap.Workspace
	s.OccupiedWorkspaces = snap.OccupiedWorkspaces
	s.SplitRatio = snap.SplitRatio

	if snap.Hidden != nil {
		s.Hidden = snap.Hidden
	}
	if snap.DisplacedMasters != nil {
		s.DisplacedMasters = snap.DisplacedMasters
	}
	if snap.ThreeBody != nil {
		s.ThreeBody = snap.ThreeBody
	}
	if snap.ProjectPaths != nil {
		s.ProjectPaths = snap.ProjectPaths
	}
	if snap.Monocle != nil {
		s.Monocle = snap.Monocle
	}
	if snap.ActiveSessions != nil {
		s.ActiveSessions = snap.ActiveSessions
	}
	if snap.TabMemory != nil {
		s.TabMemory = snap.TabMemory
	}

	return nil
}

// ReloadConfig swaps in a new HyprConfig during hot-reload.
func (s *State) ReloadConfig(cfg *config.HyprConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}
