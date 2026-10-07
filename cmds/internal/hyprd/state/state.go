// Package state stores hyprd runtime state behind a single mutex.
package state

import (
	"encoding/json"
	"slices"
	"sync"
	"time"

	"dotfiles/cmds/internal/config"
)

// State holds all hyprd runtime fields, guarded by a single RWMutex.
//
// Exported fields are JSON-serialized for subscriber event streams; always use accessor methods.
type State struct {
	mu sync.RWMutex

	Workspace          int                     `json:"workspace"`
	OccupiedWorkspaces []int                   `json:"occupied_workspaces"`
	Hidden             map[string]*HiddenState `json:"hidden,omitempty"`
	DisplacedMasters   map[int]string          `json:"displaced_masters,omitempty"`
	ThreeBody          map[int]*ThreeBodyState `json:"three_body,omitempty"`
	ProjectPaths       map[int]string          `json:"project_paths,omitempty"`
	Monocle            map[int]*MonocleState   `json:"monocle,omitempty"`
	SplitMarks         map[int]SplitMark       `json:"split_marks,omitempty"`
	ActiveSessions     map[int]string          `json:"active_sessions,omitempty"`
	ScreenShare        bool                    `json:"screen_share"`
	BrowserQA          []BrowserQAWindow       `json:"browser_qa"`
	pendingLaunches    map[string]time.Time    `json:"-"`
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
		SplitMarks:         make(map[int]SplitMark),
		ActiveSessions:     make(map[int]string),
		BrowserQA:          []BrowserQAWindow{},
		pendingLaunches:    make(map[string]time.Time),
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

// SplitMark records the split preset and share mode last applied to a workspace.
type SplitMark struct {
	Preset string `json:"preset"`
	Share  bool   `json:"share"`
}

func (s *State) SetSplitMark(ws int, mark SplitMark) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SplitMarks[ws] = mark
}

func (s *State) GetSplitMark(ws int) SplitMark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SplitMarks[ws]
}

func (s *State) ClearSplitMark(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.SplitMarks, ws)
}

func (s *State) SetScreenShare(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ScreenShare = active
}

func (s *State) GetScreenShare() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ScreenShare
}

// BrowserQAWindow identifies one marked browser window and its dedicated workspace.
type BrowserQAWindow struct {
	Address   string `json:"address"`
	Title     string `json:"title"`
	Slot      int    `json:"slot"`
	Workspace string `json:"workspace"`
}

func (s *State) SetBrowserQA(windows []BrowserQAWindow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BrowserQA = append([]BrowserQAWindow{}, windows...)
}

func (s *State) GetBrowserQA() []BrowserQAWindow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]BrowserQAWindow{}, s.BrowserQA...)
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
	s.ScreenShare = snap.ScreenShare
	s.BrowserQA = append([]BrowserQAWindow{}, snap.BrowserQA...)

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
	if snap.SplitMarks != nil {
		s.SplitMarks = snap.SplitMarks
	}
	if snap.ActiveSessions != nil {
		s.ActiveSessions = snap.ActiveSessions
	}
	if s.pendingLaunches == nil {
		s.pendingLaunches = make(map[string]time.Time)
	}

	return nil
}

// ReloadConfig swaps in a new HyprConfig during hot-reload.
func (s *State) ReloadConfig(cfg *config.HyprConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}
