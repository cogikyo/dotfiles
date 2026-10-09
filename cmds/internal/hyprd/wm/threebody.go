package wm

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// ThreeBody implements a 3-window layout: master + active slave + hidden shadow.
//
// Invariant: when enrolled, exactly two windows are tiled and the shadow is parked floating on windows.ShadowWorkspace at the slave box.
type ThreeBody struct {
	hypr  *hypr.Client
	state *state.State
}

func NewThreeBody(h *hypr.Client, s *state.State) *ThreeBody {
	return &ThreeBody{hypr: h, state: s}
}

var chatBodies = map[string]config.ThreeBodyWindow{
	"editor": {Class: "slack", Command: "slack"},
	"agents": {Class: "grok-bot", Command: "gtk-launch grok-bot"},
}

const threeBodyLaunchTTL = 5 * time.Second

// Execute focuses a configured body like "editor"/"agents"/"browser" on the active workspace.
func (tb *ThreeBody) Execute(name string) (string, error) {
	wsID, err := tb.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}

	spec, ok := bodySpec(name, wsID)
	if !ok {
		return "", fmt.Errorf("unknown three-body window: %s", name)
	}
	if ignoreBodyOnWorkspace(name, wsID) {
		return fmt.Sprintf("ignored on the music workspace: %s", name), nil
	}
	return tb.Focus(wsID, name, spec.Class, spec.Title, spec.Command)
}

func bodySpec(name string, wsID int) (config.ThreeBodyWindow, bool) {
	if spec, ok := chatBodies[name]; ok && wsID == chatWorkspace {
		return spec, true
	}
	spec, ok := config.ThreeBody[name]
	return spec, ok
}

func ignoreBodyOnWorkspace(name string, wsID int) bool {
	return wsID == musicWorkspace && (name == "editor" || name == "agents")
}

// Show focuses a window without ever exposing the shadow workspace.
//
// A parked three-body member swaps into its own workspace's slave slot.
// A parked orphan moves to home, or to the active workspace when home is 0, where it tiles as a slave, or as master on an empty workspace.
func (tb *ThreeBody) Show(address string, home int) error {
	clients, err := tb.hypr.Clients()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(clients, func(c hypr.Window) bool { return c.Address == address })
	if i < 0 {
		return fmt.Errorf("window %s not found", address)
	}
	if clients[i].Workspace.Name != windows.ShadowWorkspace {
		return tb.hypr.FocusWindow(address)
	}

	wsID := home
	for id, st := range tb.state.AllThreeBody() {
		if st.Has(address) {
			wsID = id
			break
		}
	}
	if wsID <= 0 {
		if wsID, err = tb.hypr.ActiveWorkspace(); err != nil {
			return err
		}
	}
	if err := tb.hypr.FocusWorkspace(wsID); err != nil {
		return fmt.Errorf("focus workspace %d: %w", wsID, err)
	}
	_, err = tb.swapIn(wsID, address)
	return err
}

// Toggle swaps the active workspace's parked shadow with its visible slave.
func (tb *ThreeBody) Toggle() (string, error) {
	wsID, err := tb.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	return tb.Swap(wsID)
}

// Swap rotates the parked shadow into view, enrolling three tiled windows when no live three-body exists.
func (tb *ThreeBody) Swap(wsID int) (string, error) {
	if st := tb.state.GetThreeBody(wsID); st != nil {
		clients, err := tb.hypr.Clients()
		if err != nil {
			return "", err
		}
		if shadow := windows.Parked(st, clients); shadow != nil {
			return tb.swapIn(wsID, shadow.Address)
		}
		tb.state.ClearThreeBody(wsID)
	}

	tiled, err := windows.GetTiledWindows(tb.hypr, wsID)
	if err != nil {
		return "", err
	}
	slaves := windows.GetSlaves(tiled)
	if len(tiled) != 3 || len(slaves) != 2 {
		return "no three-body", nil
	}

	if err := windows.Park(tb.hypr, slaves[1], wsID); err != nil {
		return "", fmt.Errorf("park shadow: %w", err)
	}
	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: tiled[0].Address, Active: slaves[0].Address, Shadow: slaves[1].Address})
	return fmt.Sprintf("enrolled: master=%s active=%s shadow=%s", tiled[0].Address, slaves[0].Address, slaves[1].Address), nil
}

// SwapMaster promotes the parked shadow into the master slot; the old master becomes the new shadow.
func (tb *ThreeBody) SwapMaster() (string, error) {
	wsID, err := tb.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	st := tb.state.GetThreeBody(wsID)
	if st == nil {
		return "", nil
	}
	clients, err := tb.hypr.Clients()
	if err != nil {
		return "", err
	}
	tiled := windows.Tiled(clients, wsID)
	parked := windows.Parked(st, clients)
	slaves := windows.GetSlaves(tiled)
	if parked == nil || len(slaves) == 0 {
		return "", nil
	}
	shadow, master, active := parked.Address, tiled[0].Address, slaves[0].Address

	if err := windows.SwapSlot(tb.hypr, shadow, master, wsID); err != nil {
		return "", fmt.Errorf("swap shadow into master: %w", err)
	}
	if err := windows.Fit(tb.hypr, master, wsID); err != nil {
		return "", fmt.Errorf("fit old master: %w", err)
	}
	_ = tb.hypr.FocusWindow(active)
	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: shadow, Active: active, Shadow: master})
	return fmt.Sprintf("master swapped: master=%s shadow=%s", shadow, master), nil
}

// Focus focuses a named body by class/title, enrolling or launching as needed.
func (tb *ThreeBody) Focus(wsID int, bodyName, class, title, launchCmd string) (string, error) {
	if class == "" {
		return "", fmt.Errorf("class required")
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
	i := slices.IndexFunc(clients, func(c hypr.Window) bool {
		return st.Has(c.Address) && windows.MatchesTarget(&c, class, title)
	})
	if i < 0 {
		return fmt.Sprintf("not found: %s %s", class, title), nil
	}
	target := clients[i]
	if target.Workspace.Name == windows.ShadowWorkspace {
		return tb.swapIn(wsID, target.Address)
	}
	_ = tb.hypr.FocusWindow(target.Address)
	return fmt.Sprintf("focused: %s", target.Address), nil
}

// swapIn tiles a parked window on wsID and focuses it.
//
// A member of the workspace's three-body trades places with the visible slave.
// Anything else joins the layout, and a three-body too small to have a slave dissolves.
func (tb *ThreeBody) swapIn(wsID int, incoming string) (string, error) {
	tiled, err := windows.GetTiledWindows(tb.hypr, wsID)
	if err != nil {
		return "", fmt.Errorf("get tiled: %w", err)
	}
	st := tb.state.GetThreeBody(wsID)
	slaves := windows.GetSlaves(tiled)

	if !st.Has(incoming) || len(slaves) == 0 {
		if st.Has(incoming) {
			tb.state.ClearThreeBody(wsID)
		}
		if err := windows.Unpark(tb.hypr, incoming, wsID); err != nil {
			return "", fmt.Errorf("tile %s: %w", incoming, err)
		}
		if err := tb.hypr.FocusWindow(incoming); err != nil {
			return "", fmt.Errorf("focus %s: %w", incoming, err)
		}
		return fmt.Sprintf("tiled: %s", incoming), nil
	}

	outgoing := slaves[0].Address
	if err := windows.SwapSlot(tb.hypr, incoming, outgoing, wsID); err != nil {
		return "", fmt.Errorf("swap shadow: %w", err)
	}
	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: tiled[0].Address, Active: incoming, Shadow: outgoing})
	return fmt.Sprintf("swapped: active=%s shadow=%s", incoming, outgoing), nil
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
			tb.clearLaunch(wsID, bodyName)
			_ = tb.hypr.FocusWindow(c.Address)
			return fmt.Sprintf("focused (no three-body): %s", c.Address), nil
		}
	}

	if launchCmd != "" {
		return tb.launch(wsID, bodyName, launchCmd, "")
	}
	return fmt.Sprintf("not found: %s %s", class, title), nil
}

func (tb *ThreeBody) launch(wsID int, bodyName, launchCmd, msg string) (string, error) {
	if !tb.state.ClaimThreeBodyLaunch(tb.launchKey(wsID, bodyName), threeBodyLaunchTTL) {
		return fmt.Sprintf("launch pending: %s", bodyName), nil
	}

	cmd := tb.withSessionLaunchEnv(launchCmd, wsID, bodyName)
	if err := tb.hypr.Exec(cmd); err != nil {
		tb.clearLaunch(wsID, bodyName)
		return "", fmt.Errorf("launch: %w", err)
	}
	if msg != "" {
		return msg, nil
	}
	return fmt.Sprintf("launched: %s", cmd), nil
}

func (tb *ThreeBody) clearLaunch(wsID int, bodyName string) {
	tb.state.ClearThreeBodyLaunch(tb.launchKey(wsID, bodyName))
}

func (tb *ThreeBody) launchKey(wsID int, bodyName string) string {
	return fmt.Sprintf("%d:%s", wsID, bodyName)
}

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
		_ = tb.hypr.FocusWindow(master.Address)
		active = &slaves[0]
		shadow = &slaves[1]
		if err := windows.Park(tb.hypr, *shadow, wsID); err != nil {
			return "", fmt.Errorf("park shadow: %w", err)
		}
		tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: master.Address, Active: active.Address, Shadow: shadow.Address})
		return fmt.Sprintf("enrolled (master focused): master=%s active=%s shadow=%s", master.Address, active.Address, shadow.Address), nil
	}

	if active == nil || shadow == nil {
		return fmt.Sprintf("not found in slaves: %s %s", class, title), nil
	}

	if err := windows.Park(tb.hypr, *shadow, wsID); err != nil {
		return "", fmt.Errorf("park shadow: %w", err)
	}
	_ = tb.hypr.FocusWindow(active.Address)
	tb.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: master.Address, Active: active.Address, Shadow: shadow.Address})
	return fmt.Sprintf("enrolled: master=%s active=%s shadow=%s", master.Address, active.Address, shadow.Address), nil
}
