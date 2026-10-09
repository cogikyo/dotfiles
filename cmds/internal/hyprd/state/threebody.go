package state

import "time"

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

func (s *State) ClaimThreeBodyLaunch(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pendingLaunches == nil {
		s.pendingLaunches = make(map[string]time.Time)
	}
	now := time.Now()
	if expires, ok := s.pendingLaunches[key]; ok && now.Before(expires) {
		return false
	}
	s.pendingLaunches[key] = now.Add(ttl)
	return true
}

func (s *State) ClearThreeBodyLaunch(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingLaunches, key)
}

// SetArranging marks ws while a session layout places its windows, so new windows there are not auto-parked.
func (s *State) SetArranging(ws int, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		if s.arranging == nil {
			s.arranging = make(map[int]bool)
		}
		s.arranging[ws] = true
		return
	}
	delete(s.arranging, ws)
}

// Arranging reports whether a session layout is placing windows on ws.
func (s *State) Arranging(ws int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.arranging[ws]
}
