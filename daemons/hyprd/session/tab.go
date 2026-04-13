package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

const (
	nvimCloseTree = "\x1b:lua if vim.bo.filetype==\"NvimTree\" then require(\"nvim-tree.api\").tree.close() end\r\x0c"
	nvimFocusTree = "\x1b:lua local v=require(\"nvim-tree.view\"); if v.is_visible() then vim.fn.win_gotoid(v.get_winnr()) else require(\"nvim-tree.api\").tree.open() end\r\x0c"
)

type Tab struct {
	hypr  *hypr.Client
	state *state.State
}

func NewTab(h *hypr.Client, s *state.State) *Tab {
	return &Tab{hypr: h, state: s}
}

func (t *Tab) Execute(tabName string) (string, error) {
	if tabName == "" {
		return "", fmt.Errorf("usage: tab {term|nvim|nvimtree|git|xplr}")
	}

	kittyTab := tabName
	if tabName == "nvimtree" {
		kittyTab = "nvim"
	}

	wsID, err := t.activeWorkspace()
	if err != nil {
		return "", err
	}

	editor, err := t.findEditor(wsID)
	if err != nil {
		return "", err
	}

	if editor == nil {
		if tabName == "term" {
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

	prefix := t.editorPrefix()
	targetTabID := fmt.Sprintf("%d-%s%s", st.WindowID, prefix, kittyTab)

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
	kitty.FocusTab(targetTabID)

	nvimTabID := fmt.Sprintf("%d-%snvim", st.WindowID, prefix)
	switch tabName {
	case "nvim":
		kitty.SendText(nvimTabID, nvimCloseTree)
	case "nvimtree":
		kitty.SendText(nvimTabID, nvimFocusTree)
	}

	return fmt.Sprintf("tab: %s", tabName), nil
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

func (t *Tab) editorPrefix() string {
	cfg := t.state.GetConfig()
	if cfg.Tabs != nil {
		if p, ok := cfg.Tabs["editor"]; ok {
			return p.Prefix
		}
	}
	return "ed-"
}

func (t *Tab) spawnTerminal(wsID int) (string, error) {
	project := t.state.GetProjectPath(wsID)
	if project == "" {
		project = "$HOME"
	}
	t.hypr.Dispatch(fmt.Sprintf("exec kitty --title terminal --directory %s --session ~/.config/kitty/sessions/term.conf", project))
	return "spawned terminal", nil
}
