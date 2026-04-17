package wm

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

// ThreeBody implements a 3-window layout (master + active slave + hidden shadow) used to cycle editor / agents /
// browser on a single workspace.
//
// Invariant: when state.GetThreeBody(ws) is non-nil, exactly two windows are tiled on ws and the third (shadow) is
// parked on cfg.Windows.ShadowWorkspace.
// Execute/Focus rotate which of the three holds the shadow role.
type ThreeBody struct {
	hypr      *hypr.Client
	state     *state.State
	hasNotify func() bool
	notifyAct func()
}

func NewThreeBody(h *hypr.Client, s *state.State) *ThreeBody {
	return &ThreeBody{hypr: h, state: s}
}

// SetNotifyHooks wires a notification-daemon bridge so the "agents" body can absorb a pending notification action
// instead of switching focus.
func (tb *ThreeBody) SetNotifyHooks(check func() bool, action func()) {
	tb.hasNotify = check
	tb.notifyAct = action
}

// threeBodyOrder is the shadow-swap fallback order when "shadow" is invoked without three-body enrolled on the ws.
var threeBodyOrder = []string{"editor", "agents", "browser"}

// Execute dispatches a three-body command by body name.
//
// "shadow" swaps the hidden body into view (or launches a missing body).
// Any configured body name focuses that body, enrolling three-body if exactly 3 tiled windows are present.
// "agents" additionally forwards to the notification action when one is pending.
func (tb *ThreeBody) Execute(name string) (string, error) {
	cfg := tb.state.GetConfig()
	if name == "shadow" {
		return tb.executeShadow(cfg)
	}

	spec, ok := cfg.ThreeBody[name]
	if !ok {
		return "", fmt.Errorf("unknown three-body window: %s", name)
	}
	if name == "agents" && tb.hasNotify != nil && tb.hasNotify() {
		if tb.notifyAct != nil {
			tb.notifyAct()
		}
		return "notification: action", nil
	}
	return tb.Focus(name, spec.Class, spec.Title, spec.Command)
}

// WindowSpec is a flat view of a ThreeBody config entry used when iterating fallbacks.
type WindowSpec struct {
	Name      string
	Class     string
	Title     string
	LaunchCmd string
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ shadow + swap                                                                │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// executeShadow passes configured bodies (in threeBodyOrder) to Swap as fallbacks for the unenrolled case.
func (tb *ThreeBody) executeShadow(cfg *config.HyprConfig) (string, error) {
	var fallbacks []WindowSpec
	for _, name := range threeBodyOrder {
		if w, ok := cfg.ThreeBody[name]; ok {
			fallbacks = append(fallbacks, WindowSpec{Name: name, Class: w.Class, Title: w.Title, LaunchCmd: w.Command})
		}
	}
	return tb.Swap(fallbacks)
}

// Swap rotates the hidden shadow into view on the active workspace.
//
// Enrolled: swap shadow with the current slave.
// Not enrolled: enroll when 3 tiled windows are present, otherwise launch the first missing fallback.
func (tb *ThreeBody) Swap(fallbacks []WindowSpec) (string, error) {
	wsID, err := tb.activeWorkspace()
	if err != nil {
		return "", err
	}

	tbState := tb.state.GetThreeBody(wsID)
	if tbState != nil {
		return tb.swap(tbState, wsID)
	}

	cfg := tb.state.GetConfig()
	tiled, err := windows.GetTiledWindows(tb.hypr, wsID, cfg.Windows.IgnoredClasses)
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

// SwapMaster promotes the shadow into the master slot; the old master becomes the new shadow, active slave stays.
// No-op when three-body isn't enrolled.
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
//
// Enrolled: requesting the shadow triggers a swap; master/active are focused directly.
// Not enrolled: 3 tiled windows → enroll; otherwise focus a visible match or spawn launchCmd.
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

// swap is the core rotation: pull shadow back, park current slave as new shadow, focus the promoted shadow.
//
// Batched so Hyprland applies all three dispatches atomically.
// Intermediate states (two shadows, zero slaves) would confuse the tiled layout.
func (tb *ThreeBody) swap(st *state.ThreeBodyState, wsID int) (string, error) {
	cfg := tb.state.GetConfig()
	tiled, err := windows.GetTiledWindows(tb.hypr, wsID, cfg.Windows.IgnoredClasses)
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

	batch := fmt.Sprintf("dispatch movetoworkspacesilent %d,address:%s; dispatch movetoworkspacesilent %s,address:%s; dispatch focuswindow address:%s", wsID, st.Shadow, cfg.Windows.ShadowWorkspace, actualSlave, st.Shadow)
	if _, err := tb.hypr.Request("[[BATCH]]" + batch); err != nil {
		return "", fmt.Errorf("swap batch: %w", err)
	}

	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: actualMaster, Active: st.Shadow, Shadow: actualSlave})
	return fmt.Sprintf("swapped: active=%s shadow=%s", st.Shadow, actualSlave), nil
}

// focusWithEnroll runs Focus when no three-body state exists.
//
// Order: enroll if 3 tiled windows; else focus any visible match; else pull a matching shadow from another workspace;
// else spawn launchCmd.
func (tb *ThreeBody) focusWithEnroll(wsID int, bodyName, class, title, launchCmd string, clients []hypr.Window) (string, error) {
	cfg := tb.state.GetConfig()
	tiled, err := windows.GetTiledWindows(tb.hypr, wsID, cfg.Windows.IgnoredClasses)
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

	for _, otherState := range tb.state.AllThreeBody() {
		for i := range clients {
			c := &clients[i]
			if c.Address == otherState.Shadow && windows.MatchesTarget(c, class, title) {
				tb.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", c.Address))
				return fmt.Sprintf("focused (shadow): %s", c.Address), nil
			}
		}
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

// zoxideRecent returns the most-recent zoxide entry — last-resort project path.
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

// withSessionLaunchEnv prepends PROJECT_PATH and HYPRD_TAB_PROFILE to kitty `--session` launches.
// Other commands pass through unchanged.
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
	cfg := tb.state.GetConfig()
	return tb.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", cfg.Windows.ShadowWorkspace, addr))
}

// setFadeRules installs `windowrule animation fade` for each body.
// Slide animations would expose the shadow-workspace transition.
func (tb *ThreeBody) setFadeRules(windows ...hypr.Window) {
	for _, w := range windows {
		rule := fmt.Sprintf("match:class %s", w.Class)
		if w.InitialTitle != "" {
			rule += fmt.Sprintf(" match:initialTitle %s", w.InitialTitle)
		}
		tb.hypr.Request(fmt.Sprintf("keyword windowrule %s, animation fade", rule))
	}
}

// enroll turns 3 tiled windows into a three-body: master stays, one slave is promoted to active, the other becomes
// the shadow.
//
// If the master matches but neither slave does, master is focused and slaves are assigned arbitrarily — the user can
// reshuffle via subsequent Focus calls.
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
