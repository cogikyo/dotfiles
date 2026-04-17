package main

import (
	"encoding/json"
	"maps"
	"sync"
)

// State is a thread-safe map of provider topic -> any-typed payload.
// Providers stash their own structs without daemon-level typing; JSON is rendered on demand.
type State struct {
	mu   sync.RWMutex
	data map[string]any
}

func NewState() *State {
	return &State{
		data: make(map[string]any),
	}
}

func (s *State) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *State) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// GetAll returns a shallow copy so callers can iterate without holding the lock.
func (s *State) GetAll() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.data))
	maps.Copy(result, s.data)
	return result
}

func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.data)
}
