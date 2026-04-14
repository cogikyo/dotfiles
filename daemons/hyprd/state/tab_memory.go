package state

type TabMemory struct {
	ByAction map[string]string `json:"by_action,omitempty"`
	Context  string            `json:"context,omitempty"`
}

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
