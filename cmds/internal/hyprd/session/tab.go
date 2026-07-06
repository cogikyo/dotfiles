package session

// tab.go resolves tab actions/aliases and focuses or toggles a target tab in the workspace editor kitty.

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Nvim escape sequences sent via kitty send-text: \x1b exits insert, \r submits :lua, \x0c redraws.
const (
	nvimCommandPrefix = "\x1b:lua "
	nvimCommandSuffix = "\r\x0c"
	nvimCloseTreeLua  = `if vim.bo.filetype=="NvimTree" then require("nvim-tree.api").tree.close() end`
	nvimCloseTree     = nvimCommandPrefix + nvimCloseTreeLua + nvimCommandSuffix
	nvimFocusTree     = "\x1b:lua local v=require(\"nvim-tree.view\"); if v.is_visible() then vim.fn.win_gotoid(v.get_winnr()) else require(\"nvim-tree.api\").tree.open() end\r\x0c"
)

// Tab focuses or toggles a named tab inside the workspace's editor kitty instance.
type Tab struct {
	hypr  *hypr.Client
	state *state.State
}

func NewTab(h *hypr.Client, s *state.State) *Tab {
	return &Tab{hypr: h, state: s}
}

// Execute focuses the named tab, resolving aliases and semantic actions (nvim/git/build).
//
// Re-focusing the active tab toggles back to the previous window.
// Pulls the editor from the shadow workspace only when it belongs to this workspace.
func (t *Tab) Execute(tabName, filePath string) (string, error) {
	if tabName == "" {
		return "", fmt.Errorf("usage: tab <name|alias>")
	}
	cfg := t.state.GetConfig()
	req := parseTabRequest(cfg, tabName)
	actionName := req.actionName
	if filePath != "" && actionName != "nvim" {
		return "", fmt.Errorf("path argument is only supported for nvim")
	}

	wsID, err := t.activeWorkspace()
	if err != nil {
		return "", err
	}

	editor, wsID, err := t.findTargetWindow(wsID, req)
	if err != nil {
		return "", err
	}

	if editor == nil {
		if !req.explicit && actionName == "term" {
			return t.spawnTerminal(wsID)
		}
		if req.explicit {
			return fmt.Sprintf("no %s window", req.profileName), nil
		}
		return "no editor on workspace", nil
	}

	kitty := NewKittyClient(editor.Pid)
	windows, err := kitty.FullState()
	if err != nil {
		t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", editor.Address))
		return fmt.Sprintf("focused editor (no kitty socket): %s", editor.Address), nil
	}
	if len(windows) == 0 {
		return "", fmt.Errorf("no kitty windows")
	}
	st := stateFromWindow(windows[0])

	profileName := detectTabProfile(cfg, windows[0])
	if req.explicit {
		profileName = req.profileName
	}
	profile, ok := cfg.Tabs[profileName]
	if !ok {
		return "", fmt.Errorf("unknown profile: %s", profileName)
	}

	currentTab := activeProfileTabName(cfg, profileName, windows[0])
	t.rememberTabState(wsID, profileName, &profile, currentTab)

	action := req.semanticAction
	targetTab := resolveTabAlias(cfg, tabName, profileName)
	if req.explicit {
		targetTab = req.tabName
	}
	if !req.explicit && !strings.Contains(tabName, ":") && profileTab(cfg, profileName, targetTab) == nil {
		var rememberedTab, rememberedContext string
		if mem := t.state.GetTabMemory(wsID, profileName); mem != nil {
			rememberedTab = mem.ByAction[action]
			rememberedContext = mem.Context
		}
		if resolved := pickSemanticTab(&profile, action, currentTab, rememberedTab, rememberedContext); resolved != "" {
			targetTab = resolved
		}
	}
	if actionName == "nvimtree" && targetTab == actionName {
		targetTab = "nvim"
	}
	if profileTab(cfg, profileName, targetTab) == nil {
		if !req.explicit && actionName == "term" {
			return t.spawnTerminal(wsID)
		}
		return "", fmt.Errorf("tab %q not in profile %s", targetTab, profileName)
	}

	prefix := tabProfilePrefix(cfg, profileName)
	if prefix == "" {
		prefix = "ed-"
	}
	targetTabID := runtimeTabID(windows[0], &profile, targetTab)
	if targetTabID == "" {
		targetTabID = fmt.Sprintf("%d-%s%s", st.WindowID, prefix, targetTab)
	}
	actionConfig, hasActionConfig := tabAction(&profile, targetTab, action)

	activeAddr, _ := t.activeWindowAddress()
	if activeAddr == editor.Address && st.ActiveTabID == targetTabID {
		if hasActionConfig && activePaneIndex(windows[0], targetTabID) != actionConfig.Pane {
			if err := kitty.FocusPane(targetTabID, actionConfig.Pane); err != nil {
				return "", err
			}
			if filePath == "" {
				t.rememberTabState(wsID, profileName, &profile, targetTab)
				return fmt.Sprintf("tab: %s", targetTab), nil
			}
		}
		if filePath != "" {
			if err := kitty.SendText(targetTabID, nvimOpenFile(filePath)); err != nil {
				return "", err
			}
			t.rememberTabState(wsID, profileName, &profile, targetTab)
			return fmt.Sprintf("tab: %s", targetTab), nil
		}
		if req.explicit {
			return "already focused", nil
		}
		prevAddr, err := t.previousWindowAddress(wsID, editor.Address)
		if err == nil && prevAddr != "" && prevAddr != editor.Address {
			if err := t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", prevAddr)); err != nil {
				return "", err
			}
			return "toggled back", nil
		}
		return "already focused", nil
	}

	t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", editor.Address))
	focusedByID, err := focusProfileTab(kitty, targetTabID, req.explicit, &profile, targetTab)
	if err != nil {
		return "", err
	}
	if hasActionConfig && focusedByID {
		if err := kitty.FocusPane(targetTabID, actionConfig.Pane); err != nil {
			return "", err
		}
	}

	switch actionName {
	case "nvim":
		text := nvimCloseTree
		if filePath != "" {
			text = nvimOpenFile(filePath)
		}
		if err := kitty.SendText(targetTabID, text); err != nil {
			return "", err
		}
	case "nvimtree":
		if err := kitty.SendText(targetTabID, nvimFocusTree); err != nil {
			return "", err
		}
	}

	t.rememberTabState(wsID, profileName, &profile, targetTab)

	return fmt.Sprintf("tab: %s", targetTab), nil
}

type tabRequest struct {
	profileName    string
	tabName        string
	actionName     string
	semanticAction string
	explicit       bool
}

func parseTabRequest(cfg *config.HyprConfig, name string) tabRequest {
	req := tabRequest{tabName: name, actionName: baseTabName(name), semanticAction: normalizeTabAction(name)}
	profileName, tabName, ok := strings.Cut(name, ":")
	if !ok || cfg == nil || cfg.Tabs == nil {
		return req
	}
	profile, ok := cfg.Tabs[profileName]
	if !ok {
		return req
	}
	if tabName == "" {
		tabName = profile.Focus
	}
	req.profileName = profileName
	req.tabName = tabName
	req.actionName = baseTabName(tabName)
	req.semanticAction = normalizeTabAction(tabName)
	req.explicit = true
	return req
}

func (t *Tab) findTargetWindow(wsID int, req tabRequest) (*hypr.Window, int, error) {
	if req.explicit && req.profileName == "agents" {
		win, targetWS, err := t.findBodyWindow(wsID, "agents")
		return win, targetWS, err
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
		return t.focusBody(bodyName, spec, wsID, win.Address)
	}
	if ownerWS, addr := findBodyInThreeBody(clients, t.state.AllThreeBody(), spec, wsID); addr != "" {
		return t.focusBody(bodyName, spec, ownerWS, addr)
	}
	for i := range clients {
		c := &clients[i]
		if c.Workspace.Name == windows.ShadowWorkspace || !windows.MatchesTarget(c, spec.Class, spec.Title) {
			continue
		}
		return t.focusBody(bodyName, spec, c.Workspace.ID, c.Address)
	}
	if ownerWS, addr := findBodyInThreeBody(clients, t.state.AllThreeBody(), spec, 0); addr != "" {
		return t.focusBody(bodyName, spec, ownerWS, addr)
	}

	return nil, wsID, nil
}

func (t *Tab) focusBody(_ string, spec config.ThreeBodyWindow, wsID int, addr string) (*hypr.Window, int, error) {
	currentWS, err := t.activeWorkspace()
	if err != nil {
		return nil, wsID, err
	}
	if currentWS != wsID {
		if err := t.hypr.Dispatch(fmt.Sprintf("workspace %d", wsID)); err != nil {
			return nil, wsID, fmt.Errorf("focus workspace: %w", err)
		}
	}
	if st := t.state.GetThreeBody(wsID); st != nil && addr == st.Shadow {
		if err := t.swapBodyShadow(st, wsID); err != nil {
			return nil, wsID, err
		}
	} else if err := t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", addr)); err != nil {
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
	batch := fmt.Sprintf("dispatch movetoworkspacesilent %s,address:%s; dispatch movetoworkspacesilent %d,address:%s; dispatch focuswindow address:%s", windows.ShadowWorkspace, actualSlave, wsID, st.Shadow, st.Shadow)
	if _, err := t.hypr.Request("[[BATCH]]" + batch); err != nil {
		return fmt.Errorf("swap batch: %w", err)
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

func findBodyInThreeBody(clients []hypr.Window, states map[int]*state.ThreeBodyState, spec config.ThreeBodyWindow, preferredWS int) (int, string) {
	if preferredWS != 0 {
		if addr := matchingThreeBodyAddress(clients, states[preferredWS], spec); addr != "" {
			return preferredWS, addr
		}
	}
	for wsID, st := range states {
		if wsID == preferredWS {
			continue
		}
		if addr := matchingThreeBodyAddress(clients, st, spec); addr != "" {
			return wsID, addr
		}
	}
	return 0, ""
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

func focusProfileTab(kitty *KittyClient, tabID string, explicit bool, profile *config.TabProfile, tabName string) (bool, error) {
	if err := kitty.FocusTab(tabID); err == nil {
		return true, nil
	} else if !explicit {
		return false, err
	}

	idx := profileTabIndex(profile, tabName)
	if idx < 0 {
		return false, fmt.Errorf("tab %q has no index fallback", tabName)
	}
	if err := kitty.GotoTab(idx + 1); err != nil {
		return false, err
	}
	return false, nil
}

func profileTabIndex(profile *config.TabProfile, tabName string) int {
	if profile == nil {
		return -1
	}
	for i := range profile.Tabs {
		if profile.Tabs[i].Name == tabName {
			return i
		}
	}
	return -1
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
		t.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, shadow.Address))
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

func (t *Tab) activeWindowAddress() (string, error) {
	data, err := t.hypr.Request("j/activewindow")
	if err != nil {
		return "", err
	}
	var win struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(data, &win); err != nil {
		return "", err
	}
	return win.Address, nil
}

func (t *Tab) previousWindowAddress(wsID int, editorAddress string) (string, error) {
	clients, err := t.hypr.Clients()
	if err != nil {
		return "", err
	}
	bestFocusID := 0
	bestAddress := ""
	for _, c := range clients {
		if c.Workspace.ID != wsID || c.Address == editorAddress || c.FocusHistoryID <= 0 {
			continue
		}
		if bestAddress == "" || c.FocusHistoryID < bestFocusID {
			bestFocusID = c.FocusHistoryID
			bestAddress = c.Address
		}
	}
	return bestAddress, nil
}

func (t *Tab) spawnTerminal(wsID int) (string, error) {
	project := t.state.GetProjectPath(wsID)
	if project == "" {
		project = "$HOME"
	}
	t.hypr.Dispatch(fmt.Sprintf("exec kitty --title terminal --directory %s --session ~/.config/kitty/sessions/term.conf", project))
	return "spawned terminal", nil
}

func nvimOpenFile(filePath string) string {
	return nvimCommandPrefix + "local p=" + luaQuote(filePath) + "; " + nvimCloseTreeLua + `; vim.cmd("edit "..vim.fn.fnameescape(p))` + nvimCommandSuffix
}

func luaQuote(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(value) {
		switch c := value[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, `\%03d`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (t *Tab) rememberTabState(wsID int, profileName string, profile *config.TabProfile, tabName string) {
	context := tabContext(tabName)
	for _, action := range actionKeysForTab(profile, tabName) {
		t.state.RememberTab(wsID, profileName, action, tabName, context)
	}
	if context != "" {
		t.state.RememberTab(wsID, profileName, "", "", context)
	}
}
