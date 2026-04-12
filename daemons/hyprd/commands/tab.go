package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"dotfiles/daemons/hyprd/hypr"
)

const (
	nvimCloseTree = "\x1b:lua if vim.bo.filetype==\"NvimTree\" then require(\"nvim-tree.api\").tree.close() end\r"
	nvimFocusTree = "\x1b:lua local v=require(\"nvim-tree.view\"); if v.is_visible() then vim.fn.win_gotoid(v.get_winnr()) else require(\"nvim-tree.api\").tree.open() end\r"
)

// Tab switches kitty tabs within the "editor" window on the current workspace.
type Tab struct {
	hypr  *hypr.Client
	state StateManager
}

// NewTab creates a Tab command handler.
func NewTab(h *hypr.Client, s StateManager) *Tab {
	return &Tab{hypr: h, state: s}
}

// Execute focuses the editor window and switches to the named kitty tab.
// Valid tab names: term, nvim, nvimtree, git, xplr.
func (t *Tab) Execute(tabName string) (string, error) {
	if tabName == "" {
		return "", fmt.Errorf("usage: tab {term|nvim|nvimtree|git|xplr}")
	}

	// Resolve the actual kitty tab ID (nvimtree uses the nvim tab)
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
	state, err := kitty.State()
	if err != nil {
		// Socket not available — just focus the hyprland window
		t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", editor.Address))
		return fmt.Sprintf("focused editor (no kitty socket): %s", editor.Address), nil
	}

	targetTabID := fmt.Sprintf("%d-%s", state.WindowID, kittyTab)

	// Already focused on target tab — do nothing
	activeAddr, _ := t.activeWindowAddress()
	if activeAddr == editor.Address && state.ActiveTabID == targetTabID {
		prevAddr, err := t.previousWindowAddress()
		if err == nil && prevAddr != "" && prevAddr != editor.Address {
			if err := t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", prevAddr)); err != nil {
				return "", err
			}
			return "toggled back", nil
		}
		return "already focused", nil
	}

	// Focus the hyprland window and switch kitty tab
	t.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", editor.Address))
	kitty.FocusTab(targetTabID)

	// Handle nvim special cases
	nvimTabID := fmt.Sprintf("%d-nvim", state.WindowID)
	switch tabName {
	case "nvim":
		kitty.SendText(nvimTabID, nvimCloseTree)
	case "nvimtree":
		kitty.SendText(nvimTabID, nvimFocusTree)
	}

	return fmt.Sprintf("tab: %s", tabName), nil
}

// activeWorkspace returns the current workspace ID.
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

// findEditor returns the kitty window with initialTitle "editor" on the given workspace.
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

	// Also check shadow workspace in case three-body hid it
	cfg := t.state.GetConfig()
	for i := range clients {
		c := &clients[i]
		if strings.HasPrefix(c.Workspace.Name, cfg.Windows.ShadowWorkspace) &&
			c.Class == "kitty" && c.InitialTitle == "editor" {
			// Restore from shadow before returning
			t.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, c.Address))
			return c, nil
		}
	}

	return nil, nil
}

// activeWindowAddress returns the address of the currently focused hyprland window.
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

// previousWindowAddress returns the previously focused Hyprland window.
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

// spawnTerminal launches a floating terminal when no editor exists.
func (t *Tab) spawnTerminal(wsID int) (string, error) {
	project := t.state.GetProjectPath(wsID)
	if project == "" {
		project = "$HOME"
	}
	t.hypr.Dispatch(fmt.Sprintf("exec kitty --title terminal --directory %s --session ~/.config/kitty/sessions/term.conf", project))
	return "spawned terminal", nil
}
