package state

// GetMonocle returns a deep copy of the workspace's MonocleState, or nil when not in monocle mode.
func (s *State) GetMonocle(ws int) *MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ms := s.Monocle[ws]
	if ms == nil {
		return nil
	}
	copied := *ms
	copied.Windows = make([]MonocleWindow, len(ms.Windows))
	copy(copied.Windows, ms.Windows)
	return &copied
}

func (s *State) SetMonocle(ws int, ms *MonocleState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Monocle[ws] = ms
}

func (s *State) ClearMonocle(ws int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Monocle, ws)
}

// AllMonocle returns a deep copy of every active monocle workspace keyed by workspace ID.
func (s *State) AllMonocle() map[int]*MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int]*MonocleState, len(s.Monocle))
	for k, v := range s.Monocle {
		copied := *v
		copied.Windows = make([]MonocleWindow, len(v.Windows))
		copy(copied.Windows, v.Windows)
		out[k] = &copied
	}
	return out
}

// ClearWindowState purges every trace of addr from the store on window-close.
//
// Clears Hidden, any DisplacedMasters mapping, drops the containing ThreeBody entry, and removes addr from any
// MonocleState (deleting the state entirely when it empties).
//
// Returns the removed ThreeBodyState so the caller can restore the surviving pair, or nil if none matched.
func (s *State) ClearWindowState(addr string) *ThreeBodyState {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Hidden, addr)
	for ws, a := range s.DisplacedMasters {
		if a == addr {
			delete(s.DisplacedMasters, ws)
		}
	}

	for ws, tb := range s.ThreeBody {
		if tb.Master == addr || tb.Active == addr || tb.Shadow == addr {
			removed := *tb
			delete(s.ThreeBody, ws)
			return &removed
		}
	}

	for ws, ms := range s.Monocle {
		for i, mw := range ms.Windows {
			if mw.Address == addr {
				ms.Windows = append(ms.Windows[:i], ms.Windows[i+1:]...)
				if len(ms.Windows) == 0 {
					delete(s.Monocle, ws)
				}
				return nil
			}
		}
	}

	return nil
}
