package daemon

// ================================================================================
// Thread-safe generic state storage with RWMutex protection
// ================================================================================

import (
	"encoding/json"
	"maps"
	"sync"
)

// State holds all daemon state with thread-safe access.
// Uses a generic map to avoid boilerplate getter/setter methods per field.
type State struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewState creates initialized state.
func NewState() *State {
	return &State{
		data: make(map[string]any),
	}
}

// Set updates a state value by key.
func (s *State) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get returns a state value by key.
func (s *State) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// GetAll returns a copy of all state data.
func (s *State) GetAll() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.data))
	maps.Copy(result, s.data)
	return result
}

// JSON returns state as JSON bytes.
func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.data)
}
