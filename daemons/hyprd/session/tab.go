package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

// Escape sequences sent into nvim via kitty send-text.
// \x1b exits insert mode, \r submits the :lua, \x0c (Ctrl-L) forces redraw.
const (
	nvimCloseTree = "\x1b:lua if vim.bo.filetype==\"NvimTree\" then require(\"nvim-tree.api\").tree.close() end\r\x0c"
	nvimFocusTree = "\x1b:lua local v=require(\"nvim-tree.view\"); if v.is_visible() then vim.fn.win_gotoid(v.get_winnr()) else require(\"nvim-tree.api\").tree.open() end\r\x0c"
)

// Tab focuses or toggles a named tab inside the workspace's editor kitty.
type Tab struct {
	hypr  *hypr.Client
	state *state.State
}

func NewTab(h *hypr.Client, s *state.State) *Tab {
	return &Tab{hypr: h, state: s}
}

// Execute focuses the tab matching tabName on the active workspace.
//
// tabName may be a bare name, a colon-separated alias, or a semantic action (nvim/git/build).
// Pulls the editor in from the shadow workspace when stashed there.
// Re-focusing the already-active tab toggles back to the previous window.
// "term" with no editor present spawns a fresh terminal in the project dir.
//
// Depends on KITTY_TAB_ID env tagging installed by Tabs.init; without it, only a raw editor-window focus is possible.
func (t *Tab) Execute(tabName string) (string, error) {
	if tabName == "" {
		return "", fmt.Errorf("usage: tab <name|alias>")
	}
	actionName := baseTabName(tabName)

	wsID, err := t.activeWorkspace()
	if err != nil {
		return "", err
	}

	editor, err := t.findEditor(wsID)
	if err != nil {
		return "", err
	}

	if editor == nil {
		if actionName == "term" {
			return t.spawnTerminal(wsID)
		}
		return "no editor on workspace", nil
	}

	kitty := NewKittyClient(editor.Pid)
	st, err := kitty.State()
	if err != nil {
		t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", editor.Address))
		return fmt.Sprintf("focused editor (no kitty socket): %s", editor.Address), nil
	}

	windows, err := kitty.FullState()
	if err != nil {
		return "", err
	}
	if len(windows) == 0 {
		return "", fmt.Errorf("no kitty windows")
	}

	cfg := t.state.GetConfig()
	profileName := detectTabProfile(cfg, windows[0])
	profile, ok := cfg.Tabs[profileName]
	if !ok {
		return "", fmt.Errorf("unknown profile: %s", profileName)
	}

	currentTab := activeProfileTabName(cfg, profileName, windows[0])
	t.rememberTabState(wsID, profileName, &profile, currentTab)

	targetTab := resolveTabAlias(cfg, tabName, profileName)
	if !strings.Contains(tabName, ":") && profileTab(cfg, profileName, targetTab) == nil {
		var rememberedTab, rememberedContext string
		if mem := t.state.GetTabMemory(wsID, profileName); mem != nil {
			rememberedTab = mem.ByAction[normalizeTabAction(tabName)]
			rememberedContext = mem.Context
		}
		if resolved := pickSemanticTab(&profile, normalizeTabAction(tabName), currentTab, rememberedTab, rememberedContext); resolved != "" {
			targetTab = resolved
		}
	}
	if actionName == "nvimtree" && targetTab == actionName {
		targetTab = "nvim"
	}
	if profileTab(cfg, profileName, targetTab) == nil {
		if actionName == "term" {
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

	activeAddr, _ := t.activeWindowAddress()
	if activeAddr == editor.Address && st.ActiveTabID == targetTabID {
		prevAddr, err := t.previousWindowAddress()
		if err == nil && prevAddr != "" && prevAddr != editor.Address {
			if err := t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", prevAddr)); err != nil {
				return "", err
			}
			return "toggled back", nil
		}
		return "already focused", nil
	}

	t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", editor.Address))
	if err := kitty.FocusTab(targetTabID); err != nil {
		return "", err
	}

	switch actionName {
	case "nvim":
		if err := kitty.SendText(targetTabID, nvimCloseTree); err != nil {
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

	cfg := t.state.GetConfig()
	for i := range clients {
		c := &clients[i]
		if strings.HasPrefix(c.Workspace.Name, cfg.Windows.ShadowWorkspace) &&
			c.Class == "kitty" && c.InitialTitle == "editor" {
			t.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, c.Address))
			return c, nil
		}
	}

	return nil, nil
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

func (t *Tab) previousWindowAddress() (string, error) {
	clients, err := t.hypr.Clients()
	if err != nil {
		return "", err
	}
	for _, c := range clients {
		if c.FocusHistoryID == 1 {
			return c.Address, nil
		}
	}
	return "", nil
}

func (t *Tab) spawnTerminal(wsID int) (string, error) {
	project := t.state.GetProjectPath(wsID)
	if project == "" {
		project = "$HOME"
	}
	t.hypr.Dispatch(fmt.Sprintf("exec kitty --title terminal --directory %s --session ~/.config/kitty/sessions/term.conf", project))
	return "spawned terminal", nil
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
