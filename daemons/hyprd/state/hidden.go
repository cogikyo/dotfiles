package state

// HiddenState records a window stashed on the special workspace.
//
// OriginWS and SlaveIndex are kept so the window can be restored to its prior layout position.
type HiddenState struct {
	Address    string `json:"address"`
	OriginWS   int    `json:"origin_ws"`
	SlaveIndex int    `json:"slave_index"`
}

// ThreeBodyState is a three-window layout where Master is always visible.
//
// Exactly one of Active or Shadow is rendered; the other lives on the special workspace until toggled.
type ThreeBodyState struct {
	Master string `json:"master"`
	Active string `json:"active"`
	Shadow string `json:"shadow"`
}

type MonocleWindow struct {
	Address  string `json:"address"`
	OriginWS int    `json:"origin_ws"`
}

// MonocleState holds the per-workspace monocle snapshot.
//
// SavedThreeBody is set only when monocle was entered over a three-body layout, so the pair can be restored on exit.
type MonocleState struct {
	Focused        string          `json:"focused"`
	Master         string          `json:"master"`
	Windows        []MonocleWindow `json:"windows"`
	SavedThreeBody *ThreeBodyState `json:"saved_three_body,omitempty"`
	SavedSplitRatio string         `json:"saved_split_ratio,omitempty"`
}

// GetHidden returns a deep copy of the hidden-window map; safe to mutate.
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
