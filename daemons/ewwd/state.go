package main

import (
	"encoding/json"
	"maps"
	"sync"
)

// State provides thread-safe storage for provider data using a generic key-value map.
// This avoids needing typed getters/setters for each provider's data structure.
type State struct {
	mu   sync.RWMutex      // Protects concurrent access
	data map[string]any    // Provider data keyed by topic name
}

// NewState creates an initialized state store.
func NewState() *State {
	return &State{
		data: make(map[string]any),
	}
}

// Set atomically updates a topic's data.
func (s *State) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get returns the current data for a topic, or nil if not set.
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

// JSON marshals all state data to JSON for client consumption.
func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.data)
}
