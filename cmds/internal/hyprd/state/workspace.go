package state

// workspace.go stores displaced-master addresses used by swap and workspace normalization commands.

// SetDisplacedMaster records the former master for a workspace; empty addr clears it.
func (s *State) SetDisplacedMaster(ws int, addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if addr == "" {
		delete(s.DisplacedMasters, ws)
		return
	}
	s.DisplacedMasters[ws] = addr
}

// GetDisplacedMaster returns the former master address for ws, or "" if unset.
func (s *State) GetDisplacedMaster(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DisplacedMasters[ws]
}
