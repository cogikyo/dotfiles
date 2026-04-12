package commands

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ThreeBody manages a three-window layout where only master + one slave are visible.
// The third window is hidden in a shadow workspace, swapped in on focus.
type ThreeBody struct {
	hypr      *hypr.Client
	state     StateManager
	hasNotify func() bool // checks if dunst has notifications
	notifyAct func()      // runs dunstctl action
}

// NewThreeBody creates a ThreeBody command handler.
func NewThreeBody(h *hypr.Client, s StateManager) *ThreeBody {
	return &ThreeBody{hypr: h, state: s}
}

// SetNotifyHooks injects notification check/action functions for notify-or behavior.
func (tb *ThreeBody) SetNotifyHooks(check func() bool, action func()) {
	tb.hasNotify = check
	tb.notifyAct = action
}

// threeBodyOrder defines the deterministic iteration order for fallbacks.
var threeBodyOrder = []string{"editor", "agents", "browser"}

// Execute handles named three-body subcommands (editor, agents, browser, shadow).
func (tb *ThreeBody) Execute(name string) (string, error) {
	cfg := tb.state.GetConfig()

	if name == "shadow" {
		return tb.executeShadow(cfg)
	}

	spec, ok := cfg.ThreeBody[name]
	if !ok {
		return "", fmt.Errorf("unknown three-body window: %s", name)
	}

	if spec.NotifyOr && tb.hasNotify != nil && tb.hasNotify() {
		if tb.notifyAct != nil {
			tb.notifyAct()
		}
		return "notification: action", nil
	}

	return tb.Focus(spec.Class, spec.Title, spec.Command)
}

// executeShadow swaps active/shadow, launching missing windows from config fallbacks.
func (tb *ThreeBody) executeShadow(cfg *config.HyprConfig) (string, error) {
	var fallbacks []WindowSpec
	for _, name := range threeBodyOrder {
		if w, ok := cfg.ThreeBody[name]; ok {
			fallbacks = append(fallbacks, WindowSpec{
				Class:     w.Class,
				Title:     w.Title,
				LaunchCmd: w.Command,
			})
		}
	}
	return tb.Swap(fallbacks)
}

// WindowSpec describes an expected window for three-body enrollment fallbacks.
type WindowSpec struct {
	Class     string
	Title     string
	LaunchCmd string
}

// Swap toggles between active and shadow slaves.
// If three-body isn't enrolled yet, launches the first missing window from fallbacks.
func (tb *ThreeBody) Swap(fallbacks []WindowSpec) (string, error) {
	wsData, err := tb.hypr.Request("j/activeworkspace")
	if err != nil {
		return "", err
	}
	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		return "", fmt.Errorf("parse workspace: %w", err)
	}

	state := tb.state.GetThreeBody(ws.ID)
	if state != nil {
		return tb.swap(state, ws.ID)
	}

	// No three-body state — try to enroll or launch missing windows
	cfg := tb.state.GetConfig()
	tiled, err := GetTiledWindows(tb.hypr, ws.ID, cfg.Windows.IgnoredClasses)
	if err != nil {
		return "", err
	}

	if len(tiled) == 3 {
		// All 3 present — enroll with first slave as active, second as shadow
		slaves := GetSlaves(tiled)
		if len(slaves) == 2 {
			if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
				cfg.Windows.ShadowWorkspace, slaves[1].Address)); err != nil {
				return "", fmt.Errorf("hide shadow: %w", err)
			}
			tb.state.SetThreeBody(ws.ID, &ThreeBodyState{
				Master: tiled[0].Address,
				Active: slaves[0].Address,
				Shadow: slaves[1].Address,
			})
			return fmt.Sprintf("enrolled: master=%s active=%s shadow=%s",
				tiled[0].Address, slaves[0].Address, slaves[1].Address), nil
		}
	}

	// Fewer than 3 windows — launch the first missing one from fallbacks
	if len(fallbacks) == 0 {
		return "no three-body active and no fallbacks provided", nil
	}

	clients, err := tb.hypr.Clients()
	if err != nil {
		return "", err
	}

	for _, fb := range fallbacks {
		found := false
		for i := range clients {
			c := &clients[i]
			if c.Workspace.ID == ws.ID && matchesTarget(c, fb.Class, fb.Title) {
				found = true
				break
			}
		}
		if !found && fb.LaunchCmd != "" {
			cmd := tb.withProjectPath(fb.LaunchCmd, ws.ID)
			if err := tb.hypr.Dispatch(fmt.Sprintf("exec %s", cmd)); err != nil {
				return "", fmt.Errorf("launch: %w", err)
			}
			return fmt.Sprintf("launched missing: %s %s", fb.Class, fb.Title), nil
		}
	}

	return "all fallback windows already present but not enough tiled", nil
}

// SwapMaster swaps the shadow window into the master position.
// Old master goes to shadow, active slave stays put.
func (tb *ThreeBody) SwapMaster() (string, error) {
	wsData, err := tb.hypr.Request("j/activeworkspace")
	if err != nil {
		return "", err
	}
	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		return "", fmt.Errorf("parse workspace: %w", err)
	}

	state := tb.state.GetThreeBody(ws.ID)
	if state == nil {
		return "", nil // No three-body — return empty so caller falls through to normal swap
	}

	cfg := tb.state.GetConfig()

	// Restore shadow to workspace (becomes a new slave)
	if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s",
		ws.ID, state.Shadow)); err != nil {
		return "", fmt.Errorf("restore shadow: %w", err)
	}

	// Focus the restored window and swap it into master position
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", state.Shadow))
	tb.hypr.Dispatch("layoutmsg swapwithmaster master")

	// Now hide old master (which is now a slave)
	if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		cfg.Windows.ShadowWorkspace, state.Master)); err != nil {
		return "", fmt.Errorf("hide old master: %w", err)
	}

	// Focus active slave to keep it selected
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", state.Active))

	// Update state: shadow becomes master, old master becomes shadow
	tb.state.SetThreeBody(ws.ID, &ThreeBodyState{
		Master: state.Shadow,
		Active: state.Active,
		Shadow: state.Master,
	})

	return fmt.Sprintf("master swapped: master=%s shadow=%s", state.Shadow, state.Master), nil
}

// Focus finds a window by class/title and ensures it's the visible slave.
// If the target is in the shadow, it swaps with the active slave.
// Auto-enrolls when 3 tiled windows exist and no state is tracked yet.
// Returns "not found" if the window doesn't exist (caller should launch it).
func (tb *ThreeBody) Focus(class, title, launchCmd string) (string, error) {
	if class == "" {
		return "", fmt.Errorf("class required")
	}

	wsData, err := tb.hypr.Request("j/activeworkspace")
	if err != nil {
		return "", err
	}
	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		return "", fmt.Errorf("parse workspace: %w", err)
	}

	clients, err := tb.hypr.Clients()
	if err != nil {
		return "", err
	}

	state := tb.state.GetThreeBody(ws.ID)

	if state != nil {
		return tb.focusWithState(state, ws.ID, class, title, clients)
	}
	return tb.focusWithEnroll(ws.ID, class, title, launchCmd, clients)
}

// focusWithState handles focus when three-body is already active on the workspace.
func (tb *ThreeBody) focusWithState(state *ThreeBodyState, wsID int, class, title string, clients []hypr.Window) (string, error) {
	target := tb.findByAddress(clients, state.Master, state.Active, state.Shadow, class, title)

	switch {
	case target == nil:
		return fmt.Sprintf("not found: %s %s", class, title), nil

	case target.Address == state.Master:
		tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", state.Master))
		return fmt.Sprintf("focused master: %s", state.Master), nil

	case target.Address == state.Active:
		tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", state.Active))
		return fmt.Sprintf("focused active: %s", state.Active), nil

	case target.Address == state.Shadow:
		return tb.swap(state, wsID)

	default:
		return fmt.Sprintf("not found: %s %s", class, title), nil
	}
}

// findByAddress looks up which of the three tracked addresses matches class/title.
func (tb *ThreeBody) findByAddress(clients []hypr.Window, master, active, shadow, class, title string) *hypr.Window {
	addresses := map[string]bool{master: true, active: true, shadow: true}
	for i := range clients {
		c := &clients[i]
		if addresses[c.Address] && matchesTarget(c, class, title) {
			return c
		}
	}
	return nil
}

// swap sends the active slave to shadow and pulls the shadow slave back.
func (tb *ThreeBody) swap(state *ThreeBodyState, wsID int) (string, error) {
	cfg := tb.state.GetConfig()
	shadowWS := cfg.Windows.ShadowWorkspace

	// Hide current active slave
	if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		shadowWS, state.Active)); err != nil {
		return "", fmt.Errorf("hide active: %w", err)
	}

	// Restore shadow slave to workspace
	if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s",
		wsID, state.Shadow)); err != nil {
		return "", fmt.Errorf("restore shadow: %w", err)
	}

	// Focus the newly visible slave
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", state.Shadow))

	// Update state: swap active and shadow
	tb.state.SetThreeBody(wsID, &ThreeBodyState{
		Master: state.Master,
		Active: state.Shadow,
		Shadow: state.Active,
	})

	return fmt.Sprintf("swapped: active=%s shadow=%s", state.Shadow, state.Active), nil
}

// focusWithEnroll handles focus when no three-body state exists yet.
// Auto-enrolls if exactly 3 tiled windows are present, otherwise searches broadly.
func (tb *ThreeBody) focusWithEnroll(wsID int, class, title, launchCmd string, clients []hypr.Window) (string, error) {
	cfg := tb.state.GetConfig()

	tiled, err := GetTiledWindows(tb.hypr, wsID, cfg.Windows.IgnoredClasses)
	if err != nil {
		return "", err
	}

	if len(tiled) == 3 {
		return tb.enroll(tiled, wsID, class, title)
	}

	// Not exactly 3 tiled windows — check if target exists on this workspace already
	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID == wsID && matchesTarget(c, class, title) {
			tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
			return fmt.Sprintf("focused (no three-body): %s", c.Address), nil
		}
	}

	// Not found — check shadow workspace (might belong to another workspace's three-body)
	for i := range clients {
		c := &clients[i]
		if strings.HasPrefix(c.Workspace.Name, cfg.Windows.ShadowWorkspace) && matchesTarget(c, class, title) {
			tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
			return fmt.Sprintf("focused (shadow): %s", c.Address), nil
		}
	}

	// Window doesn't exist — launch it if we have a command
	if launchCmd != "" {
		cmd := tb.withProjectPath(launchCmd, wsID)
		if err := tb.hypr.Dispatch(fmt.Sprintf("exec %s", cmd)); err != nil {
			return "", fmt.Errorf("launch: %w", err)
		}
		return fmt.Sprintf("launched: %s", cmd), nil
	}

	return fmt.Sprintf("not found: %s %s", class, title), nil
}

// zoxideRecent returns the most recent directory from zoxide history.
func zoxideRecent() string {
	out, err := exec.Command("zoxide", "query", "-l").Output()
	if err != nil {
		return ""
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 || lines[0] == "" {
		return ""
	}
	return lines[0]
}

// resolveProjectPath returns the project path for the current workspace.
// Priority: state (explicitly set via layout/project command) > zoxide most recent.
// Zoxide is always queried fresh — it's fast and avoids stale cache.
func (tb *ThreeBody) resolveProjectPath(wsID int) string {
	if p := tb.state.GetProjectPath(wsID); p != "" {
		return p
	}
	return zoxideRecent()
}

// withProjectPath prepends PROJECT_PATH to kitty session commands.
func (tb *ThreeBody) withProjectPath(cmd string, wsID int) string {
	if !strings.Contains(cmd, "kitty") || !strings.Contains(cmd, "--session") {
		return cmd
	}
	project := tb.resolveProjectPath(wsID)
	if project == "" {
		return cmd
	}
	return fmt.Sprintf("env PROJECT_PATH=%s %s", project, cmd)
}

// enroll sets up three-body state from exactly 3 tiled windows.
// Master is the leftmost. Target becomes the active slave, the other becomes shadow.
func (tb *ThreeBody) enroll(tiled []hypr.Window, wsID int, class, title string) (string, error) {
	cfg := tb.state.GetConfig()
	master := tiled[0]
	slaves := GetSlaves(tiled)

	if len(slaves) != 2 {
		return "", fmt.Errorf("expected 2 slaves, got %d", len(slaves))
	}

	// Determine which slave is the target
	var active, shadow *hypr.Window
	for i := range slaves {
		if matchesTarget(&slaves[i], class, title) {
			active = &slaves[i]
		} else {
			shadow = &slaves[i]
		}
	}

	// If target is the master, keep the first slave as active
	if active == nil && matchesTarget(&master, class, title) {
		tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", master.Address))
		active = &slaves[0]
		shadow = &slaves[1]

		// Hide the shadow
		if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
			cfg.Windows.ShadowWorkspace, shadow.Address)); err != nil {
			return "", fmt.Errorf("hide shadow: %w", err)
		}

		tb.state.SetThreeBody(wsID, &ThreeBodyState{
			Master: master.Address,
			Active: active.Address,
			Shadow: shadow.Address,
		})

		return fmt.Sprintf("enrolled (master focused): master=%s active=%s shadow=%s",
			master.Address, active.Address, shadow.Address), nil
	}

	if active == nil || shadow == nil {
		return fmt.Sprintf("not found in slaves: %s %s", class, title), nil
	}

	// Hide the shadow slave
	if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		cfg.Windows.ShadowWorkspace, shadow.Address)); err != nil {
		return "", fmt.Errorf("hide shadow: %w", err)
	}

	// Focus the target
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", active.Address))

	tb.state.SetThreeBody(wsID, &ThreeBodyState{
		Master: master.Address,
		Active: active.Address,
		Shadow: shadow.Address,
	})

	return fmt.Sprintf("enrolled: master=%s active=%s shadow=%s",
		master.Address, active.Address, shadow.Address), nil
}
