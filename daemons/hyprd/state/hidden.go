package state

// HiddenState tracks a window moved to the special workspace for temporary hiding.
type HiddenState struct {
	Address    string `json:"address"`
	OriginWS   int    `json:"origin_ws"`
	SlaveIndex int    `json:"slave_index"`
}

// ThreeBodyState tracks a three-window layout where only master + one slave are visible.
type ThreeBodyState struct {
	Master string `json:"master"`
	Active string `json:"active"`
	Shadow string `json:"shadow"`
}

// MonocleWindow tracks a single window displaced during monocle mode.
type MonocleWindow struct {
	Address  string `json:"address"`
	OriginWS int    `json:"origin_ws"`
}

// MonocleState tracks windows displaced from a workspace during monocle mode.
type MonocleState struct {
	Focused        string          `json:"focused"`
	Master         string          `json:"master"`
	Windows        []MonocleWindow `json:"windows"`
	SavedThreeBody *ThreeBodyState `json:"saved_three_body,omitempty"`
}

func (s *State) GetHidden() map[string]*HiddenState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*HiddenState, len(s.Hidden))
	for k, v := range s.Hidden {
		copy := *v
		out[k] = &copy
	}
	return out
}

func (s *State) AddHidden(h *HiddenState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hidden[h.Address] = h
}

func (s *State) RemoveHidden(addr string) *HiddenState {
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
