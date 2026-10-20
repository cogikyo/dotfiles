package state

import "slices"

// HiddenState records a window stashed on the special workspace, with enough context to restore its layout position.
type HiddenState struct {
	Address    string `json:"address"`
	OriginWS   int    `json:"origin_ws"`
	SlaveIndex int    `json:"slave_index"`
}

// ThreeBodyState is a three-window layout: Master is always visible, exactly one of Active/Shadow is rendered.
type ThreeBodyState struct {
	Master string `json:"master"`
	Active string `json:"active"`
	Shadow string `json:"shadow"`
}

// Has reports whether address is one of the three bodies; a nil state has none.
func (t *ThreeBodyState) Has(address string) bool {
	return t != nil && address != "" && (t.Master == address || t.Active == address || t.Shadow == address)
}

// MonocleState tracks entry order and mutable roles for restore on exit.
type MonocleState struct {
	Order          []string        `json:"order"` // Entry-time cycle order.
	Focused        string          `json:"focused"`
	Master         string          `json:"master"`
	SavedThreeBody *ThreeBodyState `json:"saved_three_body,omitempty"`
}

// Valid reports whether the zoomed window is in the cycle order.
func (ms MonocleState) Valid() bool {
	return slices.Contains(ms.Order, ms.Focused)
}

// Parked checks recorded membership, not the window's live workspace.
func (ms MonocleState) Parked(address string) bool {
	return address != ms.Focused && slices.Contains(ms.Order, address)
}

// Drop returns state without address, advancing focus in cycle order if needed.
// Losing a three-body member discards the saved three-body roles.
func (ms MonocleState) Drop(address string) MonocleState {
	i := slices.Index(ms.Order, address)
	if i < 0 {
		return ms
	}
	ms.Order = slices.Delete(slices.Clone(ms.Order), i, i+1)
	if len(ms.Order) == 0 {
		return MonocleState{}
	}
	if ms.Focused == address {
		ms.Focused = ms.Order[i%len(ms.Order)]
	}
	if ms.Master == address {
		ms.Master = ms.Order[0]
	}
	if ms.SavedThreeBody.Has(address) {
		ms.SavedThreeBody = nil
	}
	return ms
}

// GetHidden returns a deep copy of the hidden-window map.
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
