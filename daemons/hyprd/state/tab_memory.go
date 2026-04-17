package state

// TabMemory records the last kitty tab used for each action in a profile.
//
// Context (usually the project path) disambiguates memory across project switches within the same workspace+profile.
type TabMemory struct {
	ByAction map[string]string `json:"by_action,omitempty"`
	Context  string            `json:"context,omitempty"`
}

// GetTabMemory returns a deep copy of the memory for (ws, profile), or nil when nothing is remembered.
func (s *State) GetTabMemory(ws int, profile string) *TabMemory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byProfile := s.TabMemory[ws]
	if byProfile == nil {
		return nil
	}
	mem := byProfile[profile]
	if mem == nil {
		return nil
	}

	copy := &TabMemory{Context: mem.Context}
	if len(mem.ByAction) > 0 {
		copy.ByAction = make(map[string]string, len(mem.ByAction))
		for k, v := range mem.ByAction {
			copy.ByAction[k] = v
		}
	}
	return copy
}

// RememberTab updates the tab memory for (ws, profile), creating nested maps as needed.
//
// Empty action or tabName skips the ByAction update; empty context leaves Context untouched.
func (s *State) RememberTab(ws int, profile, action, tabName, context string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.TabMemory[ws] == nil {
		s.TabMemory[ws] = make(map[string]*TabMemory)
	}
	mem := s.TabMemory[ws][profile]
	if mem == nil {
		mem = &TabMemory{ByAction: make(map[string]string)}
		s.TabMemory[ws][profile] = mem
	}

	if action != "" && tabName != "" {
		mem.ByAction[action] = tabName
	}
	if context != "" {
		mem.Context = context
	}
}
