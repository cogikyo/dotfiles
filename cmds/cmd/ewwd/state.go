package main

// state.go provides a thread-safe topic-keyed store shared by ewwd providers.
import (
	"encoding/json"
	"maps"
	"sync"
)

// State is a thread-safe map of provider topic to untyped payload, rendered as JSON on demand.
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

// GetAll returns a shallow copy for lock-free iteration.
func (s *State) GetAll() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.data)
}

func (s *State) JSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.data)
}
