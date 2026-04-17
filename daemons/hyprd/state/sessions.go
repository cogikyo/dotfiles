package state

import "dotfiles/daemons/config"

func (s *State) GetProjectPath(ws int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ProjectPaths[ws]
}

// SetProjectPath stores the project directory for a workspace; empty path clears the entry.
func (s *State) SetProjectPath(ws int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == "" {
		delete(s.ProjectPaths, ws)
		return
	}
	s.ProjectPaths[ws] = path
}

// GetActiveSession returns the runtime-overridden session for a workspace, falling back to the configured default.
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

// ActiveSession resolves the workspace's current session name to its config entry.
//
// Returns (zero, false) when no session is set or the name is not in config.
func (s *State) ActiveSession(ws int) (config.Session, bool) {
	name := s.GetActiveSession(ws)
	if name == "" {
		return config.Session{}, false
	}

	cfg := s.GetConfig()
	session, ok := cfg.Sessions[name]
	return session, ok
}

// SessionTabProfile returns the kitty tab profile mapped to a three-body body name in the workspace's active session.
//
// Returns "" when no session is active or the body has no mapping.
func (s *State) SessionTabProfile(ws int, body string) string {
	session, ok := s.ActiveSession(ws)
	if !ok || session.Tabs == nil {
		return ""
	}
	return session.Tabs[body]
}
