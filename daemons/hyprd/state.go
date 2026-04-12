package main

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/commands"
	"encoding/json"
	"sync"
)

// State tracks workspace information and window management state for commands.
// All access is thread-safe via RWMutex. Serializable to JSON for daemon status endpoint.
type State struct {
	mu sync.RWMutex

	Workspace          int   `json:"workspace"`           // Current active workspace ID
	OccupiedWorkspaces []int `json:"occupied_workspaces"` // Workspace IDs with windows

	Hidden           map[string]*commands.HiddenState `json:"hidden,omitempty"`            // Windows in special workspace by address
	DisplacedMasters map[int]string                   `json:"displaced_masters,omitempty"` // Original master window per workspace
	ThreeBody        map[int]*commands.ThreeBodyState `json:"three_body,omitempty"`        // Three-body layout state per workspace
	ProjectPaths     map[int]string                   `json:"project_paths,omitempty"`     // Project root per workspace (resolved via zoxide)
	Monocle          map[int]*commands.MonocleState   `json:"monocle,omitempty"`           // Windows displaced per workspace during monocle
	SplitRatio       string                           `json:"split_ratio"`                 // Master/slave split identifier
	config *config.HyprConfig // Daemon configuration
}

// NewState creates a State with default values and the given configuration.
func NewState(cfg *config.HyprConfig) *State {
	return &State{
		Workspace:          1,
		OccupiedWorkspaces: []int{},
		Hidden:             make(map[string]*commands.HiddenState),
		DisplacedMasters:   make(map[int]string),
		ThreeBody:          make(map[int]*commands.ThreeBodyState),
		ProjectPaths:       make(map[int]string),
		Monocle:            make(map[int]*commands.MonocleState),
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

// GetThreeBody returns a copy of three-body state for a workspace, or nil if inactive.
func (s *State) GetThreeBody(ws int) *commands.ThreeBodyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tb := s.ThreeBody[ws]
	if tb == nil {
		return nil
	}
	copy := *tb
	return &copy
}

// SetThreeBody stores three-body state for a workspace.
func (s *State) SetThreeBody(ws int, tb *commands.ThreeBodyState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ThreeBody[ws] = tb
}

// ClearThreeBody removes three-body state for a workspace.
func (s *State) ClearThreeBody(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ThreeBody, ws)
}

// AllThreeBody returns a copy of all three-body states.
func (s *State) AllThreeBody() map[int]*commands.ThreeBodyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[int]*commands.ThreeBodyState, len(s.ThreeBody))
	for k, v := range s.ThreeBody {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetProjectPath returns the project root for a workspace, or empty if unset.
func (s *State) GetProjectPath(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ProjectPaths[ws]
}

// SetProjectPath stores the project root for a workspace.
func (s *State) SetProjectPath(ws int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == "" {
		delete(s.ProjectPaths, ws)
	} else {
		s.ProjectPaths[ws] = path
	}
}

// GetMonocle returns a copy of monocle state for a workspace, or nil if inactive.
func (s *State) GetMonocle(ws int) *commands.MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ms := s.Monocle[ws]
	if ms == nil {
		return nil
	}
	copied := *ms
	copied.Windows = make([]commands.MonocleWindow, len(ms.Windows))
	copy(copied.Windows, ms.Windows)
	return &copied
}

// SetMonocle stores monocle state for a workspace.
func (s *State) SetMonocle(ws int, ms *commands.MonocleState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Monocle[ws] = ms
}

// ClearMonocle removes monocle state for a workspace.
func (s *State) ClearMonocle(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Monocle, ws)
}

// AllMonocle returns a copy of all monocle states.
func (s *State) AllMonocle() map[int]*commands.MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[int]*commands.MonocleState, len(s.Monocle))
	for k, v := range s.Monocle {
		copied := *v
		copied.Windows = make([]commands.MonocleWindow, len(v.Windows))
		copy(copied.Windows, v.Windows)
		result[k] = &copied
	}
	return result
}

// HasAnyMonocle returns true if any workspace has monocle mode active.
func (s *State) HasAnyMonocle() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Monocle) > 0
}

// ClearWindowState removes all tracked state for a window address.
// Called when a window closes to prevent stale references.
// Returns the ThreeBodyState the window belonged to (if any) for caller cleanup.
func (s *State) ClearWindowState(addr string) *commands.ThreeBodyState {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Hidden, addr)
	for ws, a := range s.DisplacedMasters {
		if a == addr {
			delete(s.DisplacedMasters, ws)
		}
	}

	for ws, tb := range s.ThreeBody {
		if tb.Master == addr || tb.Active == addr || tb.Shadow == addr {
			removed := *tb
			delete(s.ThreeBody, ws)
			return &removed
		}
	}

	for ws, ms := range s.Monocle {
		for i, mw := range ms.Windows {
			if mw.Address == addr {
				ms.Windows = append(ms.Windows[:i], ms.Windows[i+1:]...)
				if len(ms.Windows) == 0 {
					delete(s.Monocle, ws)
				}
				return nil
			}
		}
	}

	return nil
}

func (s *State) GetConfig() *config.HyprConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// ReloadConfig swaps the config pointer under write lock, preserving all runtime state.
func (s *State) ReloadConfig(cfg *config.HyprConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}
