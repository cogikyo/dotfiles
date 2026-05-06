package wm

// threebody.go manages three-body enrollment, focus rotation, and shadow swapping for configured body windows.

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ThreeBody implements a 3-window layout: master + active slave + hidden shadow.
//
// Invariant: when enrolled, exactly two windows are tiled and the shadow is parked on windows.ShadowWorkspace.
type ThreeBody struct {
	hypr      *hypr.Client
	state     *state.State
	hasNotify func() bool
	notifyAct func() bool
}

func NewThreeBody(h *hypr.Client, s *state.State) *ThreeBody {
	return &ThreeBody{hypr: h, state: s}
}

// SetNotifyHooks wires a notification bridge so "agents" can try a pending action before switching focus.
func (tb *ThreeBody) SetNotifyHooks(check func() bool, action func() bool) {
	tb.hasNotify = check
	tb.notifyAct = action
}

var threeBodyOrder = []string{"editor", "agents", "browser"}

// Execute dispatches a three-body command by body name ("shadow", or a configured body like "editor"/"agents"/"browser").
func (tb *ThreeBody) Execute(name string) (string, error) {
	if name == "shadow" {
		return tb.executeShadow()
	}

	spec, ok := config.ThreeBody[name]
	if !ok {
		return "", fmt.Errorf("unknown three-body window: %s", name)
	}
	if tb.ignoreOnCurrentWorkspace(name) {
		return fmt.Sprintf("ignored on workspace 1-2: %s", name), nil
	}
	if name == "agents" && tb.hasNotify != nil && tb.hasNotify() && tb.notifyAct != nil {
		if msg, err := tb.focusShadowedBody(name, spec.Class, spec.Title); err != nil {
			return "", err
		} else if msg != "" {
			return tb.notifyActionAfter(msg)
		}
		if tb.notifyAct() {
			return "notification: action", nil
		}
	}
	return tb.Focus(name, spec.Class, spec.Title, spec.Command)
}

func (tb *ThreeBody) ignoreOnCurrentWorkspace(name string) bool {
	if name != "editor" && name != "agents" {
		return false
	}
	wsID, err := tb.activeWorkspace()
	return err == nil && wsID >= 1 && wsID <= 2
}

func (tb *ThreeBody) notifyActionAfter(prefix string) (string, error) {
	if tb.notifyAct() {
		return prefix + "; notification: action", nil
	}
	return prefix, nil
}

func (tb *ThreeBody) focusShadowedBody(bodyName, class, title string) (string, error) {
	wsID, err := tb.activeWorkspace()
	if err != nil {
		return "", err
	}

	clients, err := tb.hypr.Clients()
	if err != nil {
		return "", err
	}
	if st := tb.state.GetThreeBody(wsID); st != nil && shadowMatches(clients, st, class, title) {
		return tb.Focus(bodyName, class, title, "")
	}

	for shadowWS, st := range tb.state.AllThreeBody() {
		if shadowWS == wsID || !shadowMatches(clients, st, class, title) {
			continue
		}
		if err := tb.hypr.Dispatch(fmt.Sprintf("workspace %d", shadowWS)); err != nil {
			return "", fmt.Errorf("focus three-body workspace: %w", err)
		}
		msg, err := tb.swap(st, shadowWS)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("workspace %d; %s", shadowWS, msg), nil
	}
	return "", nil
}

func shadowMatches(clients []hypr.Window, st *state.ThreeBodyState, class, title string) bool {
	for i := range clients {
		c := &clients[i]
		if c.Address == st.Shadow && c.Workspace.Name == windows.ShadowWorkspace && windows.MatchesTarget(c, class, title) {
			return true
		}
	}
	return false
}

// WindowSpec is a flat view of a ThreeBody config entry for fallback iteration.
type WindowSpec struct {
	Name      string
	Class     string
	Title     string
	LaunchCmd string
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ shadow + swap                                                                │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// executeShadow builds fallbacks from threeBodyOrder and delegates to Swap.
func (tb *ThreeBody) executeShadow() (string, error) {
	var fallbacks []WindowSpec
	for _, name := range threeBodyOrder {
		if w, ok := config.ThreeBody[name]; ok {
			fallbacks = append(fallbacks, WindowSpec{Name: name, Class: w.Class, Title: w.Title, LaunchCmd: w.Command})
		}
	}
	return tb.Swap(fallbacks)
}

// Swap rotates the hidden shadow into view, enrolling or launching a missing fallback as needed.
func (tb *ThreeBody) Swap(fallbacks []WindowSpec) (string, error) {
	wsID, err := tb.activeWorkspace()
	if err != nil {
		return "", err
	}

	tbState := tb.state.GetThreeBody(wsID)
	if tbState != nil {
		return tb.swap(tbState, wsID)
	}

	tiled, err := windows.GetTiledWindows(tb.hypr, wsID)
	if err != nil {
		return "", err
	}
	if len(tiled) == 3 {
		slaves := windows.GetSlaves(tiled)
		if len(slaves) == 2 {
			if err := tb.hideShadow(slaves[1].Address); err != nil {
				return "", fmt.Errorf("hide shadow: %w", err)
			}
			tb.setFadeRules(tiled[0], slaves[0], slaves[1])
			tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: tiled[0].Address, Active: slaves[0].Address, Shadow: slaves[1].Address})
			return fmt.Sprintf("enrolled: master=%s active=%s shadow=%s", tiled[0].Address, slaves[0].Address, slaves[1].Address), nil
		}
	}

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
			if c.Workspace.ID == wsID && windows.MatchesTarget(c, fb.Class, fb.Title) {
				found = true
				break
			}
		}
		if !found && fb.LaunchCmd != "" {
			cmd := tb.withSessionLaunchEnv(fb.LaunchCmd, wsID, fb.Name)
			if err := tb.hypr.Dispatch(fmt.Sprintf("exec %s", cmd)); err != nil {
				return "", fmt.Errorf("launch: %w", err)
			}
			return fmt.Sprintf("launched missing: %s %s", fb.Class, fb.Title), nil
		}
	}
	return "all fallback windows already present but not enough tiled", nil
}

// SwapMaster promotes the shadow into the master slot; the old master becomes the new shadow.
func (tb *ThreeBody) SwapMaster() (string, error) {
	wsID, err := tb.activeWorkspace()
	if err != nil {
		return "", err
	}
	tbState := tb.state.GetThreeBody(wsID)
	if tbState == nil {
		return "", nil
	}

	if err := tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, tbState.Shadow)); err != nil {
		return "", fmt.Errorf("restore shadow: %w", err)
	}
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", tbState.Shadow))
	tb.hypr.Dispatch("layoutmsg swapwithmaster master")
	if err := tb.hideShadow(tbState.Master); err != nil {
		return "", fmt.Errorf("hide old master: %w", err)
	}
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", tbState.Active))
	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: tbState.Shadow, Active: tbState.Active, Shadow: tbState.Master})
	return fmt.Sprintf("master swapped: master=%s shadow=%s", tbState.Shadow, tbState.Master), nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ focus                                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Focus focuses a named body by class/title, enrolling or launching as needed.
func (tb *ThreeBody) Focus(bodyName, class, title, launchCmd string) (string, error) {
	if class == "" {
		return "", fmt.Errorf("class required")
	}

	wsID, err := tb.activeWorkspace()
	if err != nil {
		return "", err
	}

	clients, err := tb.hypr.Clients()
	if err != nil {
		return "", err
	}

	tbState := tb.state.GetThreeBody(wsID)
	if tbState != nil {
		return tb.focusWithState(tbState, wsID, class, title, clients)
	}
	return tb.focusWithEnroll(wsID, bodyName, class, title, launchCmd, clients)
}

func (tb *ThreeBody) focusWithState(st *state.ThreeBodyState, wsID int, class, title string, clients []hypr.Window) (string, error) {
	target := tb.findByAddress(clients, st.Master, st.Active, st.Shadow, class, title)
	switch {
	case target == nil:
		return fmt.Sprintf("not found: %s %s", class, title), nil
	case target.Address == st.Master:
		tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", st.Master))
		return fmt.Sprintf("focused master: %s", st.Master), nil
	case target.Address == st.Active:
		tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", st.Active))
		return fmt.Sprintf("focused active: %s", st.Active), nil
	case target.Address == st.Shadow:
		return tb.swap(st, wsID)
	default:
		return fmt.Sprintf("not found: %s %s", class, title), nil
	}
}

func (tb *ThreeBody) findByAddress(clients []hypr.Window, master, active, shadow, class, title string) *hypr.Window {
	addresses := map[string]bool{master: true, active: true, shadow: true}
	for i := range clients {
		c := &clients[i]
		if addresses[c.Address] && windows.MatchesTarget(c, class, title) {
			return c
		}
	}
	return nil
}

// swap is the core rotation, batched so Hyprland applies all three dispatches atomically.
func (tb *ThreeBody) swap(st *state.ThreeBodyState, wsID int) (string, error) {
	tiled, err := windows.GetTiledWindows(tb.hypr, wsID)
	if err != nil {
		return "", fmt.Errorf("get tiled: %w", err)
	}
	if len(tiled) < 2 {
		return "", fmt.Errorf("expected 2 tiled windows, got %d", len(tiled))
	}

	actualMaster := tiled[0].Address
	slaves := windows.GetSlaves(tiled)
	if len(slaves) == 0 {
		return "", fmt.Errorf("no slave window found")
	}
	actualSlave := slaves[0].Address

	batch := fmt.Sprintf("dispatch movetoworkspacesilent %s,address:%s; dispatch movetoworkspacesilent %d,address:%s; dispatch focuswindow address:%s", windows.ShadowWorkspace, actualSlave, wsID, st.Shadow, st.Shadow)
	if _, err := tb.hypr.Request("[[BATCH]]" + batch); err != nil {
		return "", fmt.Errorf("swap batch: %w", err)
	}

	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: actualMaster, Active: st.Shadow, Shadow: actualSlave})
	return fmt.Sprintf("swapped: active=%s shadow=%s", st.Shadow, actualSlave), nil
}

// focusWithEnroll tries to enroll, focus a visible match, pull from another workspace's shadow, or spawn.
func (tb *ThreeBody) focusWithEnroll(wsID int, bodyName, class, title, launchCmd string, clients []hypr.Window) (string, error) {
	tiled, err := windows.GetTiledWindows(tb.hypr, wsID)
	if err != nil {
		return "", err
	}

	if len(tiled) == 3 {
		return tb.enroll(tiled, wsID, class, title)
	}

	for i := range clients {
		c := &clients[i]
		if c.Workspace.ID == wsID && windows.MatchesTarget(c, class, title) {
			tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
			return fmt.Sprintf("focused (no three-body): %s", c.Address), nil
		}
	}

	if bodyName == "agents" && wsID >= 1 && wsID <= 2 {
		return fmt.Sprintf("not found: %s %s", class, title), nil
	}

	if launchCmd != "" {
		cmd := tb.withSessionLaunchEnv(launchCmd, wsID, bodyName)
		if err := tb.hypr.Dispatch(fmt.Sprintf("exec %s", cmd)); err != nil {
			return "", fmt.Errorf("launch: %w", err)
		}
		return fmt.Sprintf("launched: %s", cmd), nil
	}
	return fmt.Sprintf("not found: %s %s", class, title), nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ launch env + helpers                                                         │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// zoxideRecent returns the most-recent zoxide entry as a last-resort project path.
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

func (tb *ThreeBody) resolveProjectPath(wsID int) string {
	if p := tb.state.GetProjectPath(wsID); p != "" {
		return p
	}
	return zoxideRecent()
}

// withSessionLaunchEnv prepends PROJECT_PATH and HYPRD_TAB_PROFILE to kitty --session launches.
func (tb *ThreeBody) withSessionLaunchEnv(cmd string, wsID int, bodyName string) string {
	if !strings.Contains(cmd, "kitty") || !strings.Contains(cmd, "--session") {
		return cmd
	}

	var env []string
	if project := tb.resolveProjectPath(wsID); project != "" {
		env = append(env, "PROJECT_PATH="+project)
	}
	if profile := tb.state.SessionTabProfile(wsID, bodyName); profile != "" {
		env = append(env, "HYPRD_TAB_PROFILE="+profile)
	}
	if len(env) == 0 {
		return cmd
	}

	return fmt.Sprintf("env %s %s", strings.Join(env, " "), cmd)
}

func (tb *ThreeBody) hideShadow(addr string) error {
	return tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", windows.ShadowWorkspace, addr))
}

// setFadeRules installs fade animation rules so slide transitions don't expose the shadow workspace.
func (tb *ThreeBody) setFadeRules(windows ...hypr.Window) {
	for _, w := range windows {
		rule := fmt.Sprintf("match:class %s", w.Class)
		if w.InitialTitle != "" {
			rule += fmt.Sprintf(" match:initialTitle %s", w.InitialTitle)
		}
		tb.hypr.Request(fmt.Sprintf("keyword windowrule %s, animation fade", rule))
	}
}

// enroll turns 3 tiled windows into a three-body: the matching slave becomes active, the other becomes shadow.
//
// If only the master matches, slaves are assigned arbitrarily.
func (tb *ThreeBody) enroll(tiled []hypr.Window, wsID int, class, title string) (string, error) {
	master := tiled[0]
	slaves := windows.GetSlaves(tiled)
	if len(slaves) != 2 {
		return "", fmt.Errorf("expected 2 slaves, got %d", len(slaves))
	}

	var active, shadow *hypr.Window
	for i := range slaves {
		if windows.MatchesTarget(&slaves[i], class, title) {
			active = &slaves[i]
		} else {
			shadow = &slaves[i]
		}
	}

	if active == nil && windows.MatchesTarget(&master, class, title) {
		tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", master.Address))
		active = &slaves[0]
		shadow = &slaves[1]
		if err := tb.hideShadow(shadow.Address); err != nil {
			return "", fmt.Errorf("hide shadow: %w", err)
		}
		tb.setFadeRules(master, *active, *shadow)
		tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: master.Address, Active: active.Address, Shadow: shadow.Address})
		return fmt.Sprintf("enrolled (master focused): master=%s active=%s shadow=%s", master.Address, active.Address, shadow.Address), nil
	}

	if active == nil || shadow == nil {
		return fmt.Sprintf("not found in slaves: %s %s", class, title), nil
	}

	if err := tb.hideShadow(shadow.Address); err != nil {
		return "", fmt.Errorf("hide shadow: %w", err)
	}
	tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", active.Address))
	tb.setFadeRules(master, *active, *shadow)
	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: master.Address, Active: active.Address, Shadow: shadow.Address})
	return fmt.Sprintf("enrolled: master=%s active=%s shadow=%s", master.Address, active.Address, shadow.Address), nil
}

func (tb *ThreeBody) activeWorkspace() (int, error) {
	data, err := tb.hypr.Request("j/activeworkspace")
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
