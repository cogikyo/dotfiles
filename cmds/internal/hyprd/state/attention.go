package state

import (
	"maps"
	"slices"
	"strings"
)

// ArmAttention marks a window class as having an unseen notification and reports whether the set changed.
//
// Classes compare case-insensitively, as Hyprland window matching does.
func (s *State) ArmAttention(class string) bool {
	class = strings.ToLower(class)
	s.mu.Lock()
	defer s.mu.Unlock()
	if class == "" || s.attention[class] {
		return false
	}
	s.attention[class] = true
	return true
}

// ClearAttention drops a window class from the attention set and reports whether the set changed.
func (s *State) ClearAttention(class string) bool {
	class = strings.ToLower(class)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.attention[class] {
		return false
	}
	delete(s.attention, class)
	return true
}

// GetAttention returns the lowercased classes with unseen notifications, sorted.
func (s *State) GetAttention() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := slices.AppendSeq(make([]string, 0, len(s.attention)), maps.Keys(s.attention))
	slices.Sort(out)
	return out
}
