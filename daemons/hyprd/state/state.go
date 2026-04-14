package state

import (
	"encoding/json"
	"sync"

	"dotfiles/daemons/config"
)

// State tracks workspace information and window management state.
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

// NewState creates a State with default values and the given configuration.
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

func (s *State) GetOccupied() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int, len(s.OccupiedWorkspaces))
	copy(out, s.OccupiedWorkspaces)
	return out
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

func (s *State) ReloadConfig(cfg *config.HyprConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}
