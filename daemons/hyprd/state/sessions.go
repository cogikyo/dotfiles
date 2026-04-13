package state

func (s *State) GetProjectPath(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ProjectPaths[ws]
}

func (s *State) SetProjectPath(ws int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == "" {
		delete(s.ProjectPaths, ws)
		return
	}
	s.ProjectPaths[ws] = path
}

func (s *State) GetActiveSession(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if name, ok := s.ActiveSessions[ws]; ok {
		return name
	}
	return s.config.ActiveSessions[ws]
}

func (s *State) SetActiveSession(ws int, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveSessions[ws] = name
}
