package state

// threebody.go provides copy-safe accessors for per-workspace three-body layout state.

// GetThreeBody returns a deep copy of the workspace's three-body state, or nil if inactive.
func (s *State) GetThreeBody(ws int) *ThreeBodyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tb := s.ThreeBody[ws]
	if tb == nil {
		return nil
	}
	copy := *tb
	return &copy
}

func (s *State) SetThreeBody(ws int, tb *ThreeBodyState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ThreeBody[ws] = tb
}

func (s *State) ClearThreeBody(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ThreeBody, ws)
}

// AllThreeBody returns a deep copy of every active three-body state keyed by workspace.
func (s *State) AllThreeBody() map[int]*ThreeBodyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int]*ThreeBodyState, len(s.ThreeBody))
	for k, v := range s.ThreeBody {
		copy := *v
		out[k] = &copy
	}
	return out
}
