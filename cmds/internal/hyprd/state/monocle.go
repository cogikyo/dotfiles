package state

import "slices"

// GetMonocle copies the workspace's monocle state, or returns nil if inactive.
// Order is cloned; SavedThreeBody remains shared.
func (s *State) GetMonocle(ws int) *MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ms := s.Monocle[ws]
	if ms == nil {
		return nil
	}
	copied := *ms
	copied.Order = slices.Clone(ms.Order)
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

// AllMonocle copies every active monocle state, keyed by workspace.
// Each Order is cloned; SavedThreeBody pointers remain shared.
func (s *State) AllMonocle() map[int]*MonocleState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int]*MonocleState, len(s.Monocle))
	for k, v := range s.Monocle {
		copied := *v
		copied.Order = slices.Clone(v.Order)
		out[k] = &copied
	}
	return out
}

// ClearWindowState purges all traces of addr from Hidden, DisplacedMasters, ThreeBody, and Monocle on window-close.
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
		if !slices.Contains(ms.Order, addr) {
			continue
		}
		if next := ms.Drop(addr); next.Valid() {
			s.Monocle[ws] = &next
		} else {
			delete(s.Monocle, ws)
		}
		return nil
	}

	return nil
}
