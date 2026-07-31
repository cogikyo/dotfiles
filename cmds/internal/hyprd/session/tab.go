package session

// tab.go focuses an editor or agents Kitty window and selects a physical tab by index.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

const maxTabIndex = 4

type Tab struct {
	hypr  *hypr.Client
	state *state.State
}

func NewTab(h *hypr.Client, s *state.State) *Tab {
	return &Tab{hypr: h, state: s}
}

// Execute focuses the requested profile window and selects its zero-based physical tab index.
func (t *Tab) Execute(target string) (string, error) {
	profile, index, err := parseTabTarget(target)
	if err != nil {
		return "", err
	}

	wsID, err := t.activeWorkspace()
	if err != nil {
		return "", err
	}

	win, wsID, err := t.findTargetWindow(wsID, profile)
	if err != nil {
		return "", err
	}
	if win == nil {
		return fmt.Sprintf("no %s window on workspace %d", profile, wsID), nil
	}

	if err := t.hypr.FocusWindow(win.Address); err != nil {
		return "", fmt.Errorf("focus %s: %w", profile, err)
	}

	kittyIndex := index + 1
	if err := NewKittyClient(win.Pid).GotoTab(kittyIndex); err != nil {
		return "", fmt.Errorf("select %s tab %d: %w", profile, index, err)
	}
	return fmt.Sprintf("tab: %s:%d", profile, index), nil
}

func parseTabTarget(target string) (string, int, error) {
	profile, rawIndex, ok := strings.Cut(strings.TrimSpace(target), ":")
	if !ok || profile == "" || rawIndex == "" {
		return "", 0, fmt.Errorf("usage: tab <editor|agents>:<index 0..4>")
	}
	if profile != "editor" && profile != "agents" {
		return "", 0, fmt.Errorf("invalid tab profile %q: must be editor or agents", profile)
	}

	index, err := strconv.Atoi(rawIndex)
	if err != nil {
		return "", 0, fmt.Errorf("invalid tab index %q: must be an integer from 0 to 4", rawIndex)
	}
	if index < 0 || index > maxTabIndex {
		return "", 0, fmt.Errorf("invalid tab index %d: must be from 0 to 4", index)
	}
	return profile, index, nil
}

func (t *Tab) findTargetWindow(wsID int, profile string) (*hypr.Window, int, error) {
	if profile == "agents" {
		return t.findBodyWindow(wsID, profile)
	}

	win, err := t.findEditor(wsID)
	return win, wsID, err
}

func (t *Tab) findBodyWindow(wsID int, bodyName string) (*hypr.Window, int, error) {
	spec, ok := config.ThreeBody[bodyName]
	if !ok {
		return nil, wsID, fmt.Errorf("unknown three-body window: %s", bodyName)
	}
	clients, err := t.hypr.Clients()
	if err != nil {
		return nil, wsID, err
	}

	if win := findBodyOnWorkspace(clients, wsID, spec); win != nil {
		return t.focusBody(spec, wsID, win.Address)
	}
	if addr := matchingThreeBodyAddress(clients, t.state.GetThreeBody(wsID), spec); addr != "" {
		return t.focusBody(spec, wsID, addr)
	}

	return nil, wsID, nil
}

func (t *Tab) focusBody(spec config.ThreeBodyWindow, wsID int, addr string) (*hypr.Window, int, error) {
	currentWS, err := t.activeWorkspace()
	if err != nil {
		return nil, wsID, err
	}
	if currentWS != wsID {
		if err := t.hypr.FocusWorkspace(wsID); err != nil {
			return nil, wsID, fmt.Errorf("focus workspace: %w", err)
		}
	}
	if st := t.state.GetThreeBody(wsID); st != nil && addr == st.Shadow {
		if err := t.swapBodyShadow(st, wsID); err != nil {
			return nil, wsID, err
		}
	} else if err := t.hypr.FocusWindow(addr); err != nil {
		return nil, wsID, err
	}

	clients, err := t.hypr.Clients()
	if err != nil {
		return nil, wsID, err
	}
	if win := findWindowByAddress(clients, addr); win != nil {
		return win, wsID, nil
	}
	return findBodyOnWorkspace(clients, wsID, spec), wsID, nil
}

func (t *Tab) swapBodyShadow(st *state.ThreeBodyState, wsID int) error {
	tiled, err := windows.GetTiledWindows(t.hypr, wsID)
	if err != nil {
		return fmt.Errorf("get tiled: %w", err)
	}
	if len(tiled) < 2 {
		return fmt.Errorf("expected 2 tiled windows, got %d", len(tiled))
	}
	slaves := windows.GetSlaves(tiled)
	if len(slaves) == 0 {
		return fmt.Errorf("no slave window found")
	}

	actualMaster := tiled[0].Address
	actualSlave := slaves[0].Address
	if err := t.hypr.MoveWindowToWorkspace(actualSlave, windows.ShadowWorkspace, false); err != nil {
		return fmt.Errorf("swap body shadow: move active slave: %w", err)
	}
	if err := t.hypr.MoveWindowToWorkspace(st.Shadow, strconv.Itoa(wsID), false); err != nil {
		return fmt.Errorf("swap body shadow: move shadow: %w", err)
	}
	if err := t.hypr.FocusWindow(st.Shadow); err != nil {
		return fmt.Errorf("swap body shadow: focus shadow: %w", err)
	}
	t.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: actualMaster, Active: st.Shadow, Shadow: actualSlave})
	return nil
}

func findBodyOnWorkspace(clients []hypr.Window, wsID int, spec config.ThreeBodyWindow) *hypr.Window {
	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID == wsID && windows.MatchesTarget(c, spec.Class, spec.Title) {
			return c
		}
	}
	return nil
}

func matchingThreeBodyAddress(clients []hypr.Window, st *state.ThreeBodyState, spec config.ThreeBodyWindow) string {
	if st == nil {
		return ""
	}
	for _, addr := range []string{st.Master, st.Active, st.Shadow} {
		if win := findWindowByAddress(clients, addr); win != nil && windows.MatchesTarget(win, spec.Class, spec.Title) {
			return addr
		}
	}
	return ""
}

func findWindowByAddress(clients []hypr.Window, addr string) *hypr.Window {
	for i := range clients {
		if clients[i].Address == addr {
			return &clients[i]
		}
	}
	return nil
}

func (t *Tab) activeWorkspace() (int, error) {
	data, err := t.hypr.Request("j/activeworkspace")
	if err != nil {
		return 0, err
	}
	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &ws); err != nil {
		return 0, fmt.Errorf("parse workspace: %w", err)
	}
	return ws.ID, nil
}

func (t *Tab) findEditor(wsID int) (*hypr.Window, error) {
	clients, err := t.hypr.Clients()
	if err != nil {
		return nil, err
	}

	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID == wsID && c.Class == "kitty" && c.InitialTitle == "editor" {
			return c, nil
		}
	}

	if shadow := shadowEditorForWorkspace(clients, t.state.GetThreeBody(wsID)); shadow != nil {
		if err := t.hypr.MoveWindowToWorkspace(shadow.Address, strconv.Itoa(wsID), false); err != nil {
			return nil, fmt.Errorf("move editor to workspace %d: %w", wsID, err)
		}
		return shadow, nil
	}

	return nil, nil
}

func shadowEditorForWorkspace(clients []hypr.Window, tb *state.ThreeBodyState) *hypr.Window {
	if tb == nil || tb.Shadow == "" {
		return nil
	}

	for i := range clients {
		c := &clients[i]
		if c.Address == tb.Shadow && strings.HasPrefix(c.Workspace.Name, windows.ShadowWorkspace) &&
			c.Class == "kitty" && c.InitialTitle == "editor" {
			return c
		}
	}

	return nil
}
