package state

import "dotfiles/daemons/config"

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
	return s.config.ActiveSessions[ws].Session
}

func (s *State) SetActiveSession(ws int, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveSessions[ws] = name
}

func (s *State) ActiveSession(ws int) (config.Session, bool) {
	name := s.GetActiveSession(ws)
	if name == "" {
		return config.Session{}, false
	}

	cfg := s.GetConfig()
	session, ok := cfg.Sessions[name]
	return session, ok
}

func (s *State) SessionTabProfile(ws int, body string) string {
	session, ok := s.ActiveSession(ws)
	if !ok || session.Tabs == nil {
		return ""
	}
	return session.Tabs[body]
}
