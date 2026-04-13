package state

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
