package wm

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Monocle parks all but the zoomed window as floats on special:mono<n>.
// All share the monocle box; role changes are kept for exit.
// An empty result with no error permits non-monocle command handling.
type Monocle struct {
	hypr  *hypr.Client
	state *state.State
}

func NewMonocle(h *hypr.Client, s *state.State) *Monocle {
	return &Monocle{hypr: h, state: s}
}

// Execute toggles monocle on the current workspace.
//
// Restore wins when a receipt exists or special:mono{n} still holds windows.
func (m *Monocle) Execute() (string, error) {
	wsID, err := m.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	if m.state.GetMonocle(wsID) != nil {
		return m.deactivate(wsID)
	}
	orphans, err := m.parked(wsID)
	if err != nil {
		return "", err
	}
	if len(orphans) > 0 {
		return m.deactivate(wsID)
	}
	return m.activate(wsID)
}

// DeactivateIfActive deactivates monocle on the current workspace without toggling it on.
func (m *Monocle) DeactivateIfActive() (string, error) {
	wsID, err := m.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	if m.state.GetMonocle(wsID) == nil {
		return "", nil
	}
	return m.deactivate(wsID)
}

// Swap updates the saved master role and zooms the swap partner.
func (m *Monocle) Swap() (string, error) {
	wsID, ms, err := m.current()
	if err != nil || ms == nil {
		return "", err
	}
	incoming, next, ok := swapRoles(*ms, m.state.GetDisplacedMaster(wsID))
	if !ok {
		return "monocle swap: no parked partner", nil
	}
	return m.rezoom(wsID, *ms, next, incoming)
}

// Shadow exchanges the saved slave roles and zooms the former shadow.
func (m *Monocle) Shadow() (string, error) {
	wsID, ms, err := m.current()
	if err != nil || ms == nil {
		return "", err
	}
	incoming, next, ok := shadowRoles(*ms)
	if !ok {
		return "monocle shadow: no parked shadow", nil
	}
	return m.rezoom(wsID, *ms, next, incoming)
}

// Cycle wraps through entry order without changing the saved roles.
func (m *Monocle) Cycle(step int) (string, error) {
	wsID, ms, err := m.current()
	if err != nil || ms == nil {
		return "", err
	}
	incoming, next, ok := zoom(*ms, neighbor(*ms, step))
	if !ok || incoming == ms.Focused {
		return fmt.Sprintf("monocle: ws%d, nothing to cycle", wsID), nil
	}
	return m.rezoom(wsID, *ms, next, incoming)
}

// Focus zooms a named body already in monocle without launching or enrolling it.
func (m *Monocle) Focus(name string) (string, error) {
	wsID, ms, err := m.current()
	if err != nil || ms == nil {
		return "", err
	}
	spec, ok := bodySpec(name, wsID)
	if !ok {
		return "", fmt.Errorf("unknown three-body window: %s", name)
	}
	if ignoreBodyOnWorkspace(name, wsID) {
		return fmt.Sprintf("ignored on the music workspace: %s", name), nil
	}
	clients, err := m.hypr.Clients()
	if err != nil {
		return "", err
	}
	target := matchBody(*ms, clients, spec.Class, spec.Title)
	if target == "" {
		return "", nil
	}
	return m.Show(target)
}

// Show zooms a recorded member on its own workspace, keeping monocle active.
// An empty result means the address was not handled.
func (m *Monocle) Show(address string) (string, error) {
	wsID, ms := 0, (*state.MonocleState)(nil)
	for id, st := range m.state.AllMonocle() {
		if slices.Contains(st.Order, address) {
			wsID, ms = id, st
			break
		}
	}
	if ms == nil {
		return "", nil
	}
	if ms.Parked(address) {
		parked, err := m.isParked(wsID, address)
		if err != nil || !parked {
			return "", err
		}
	}
	active, err := m.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}
	if active != wsID {
		if err := m.hypr.FocusWorkspace(wsID); err != nil {
			return "", fmt.Errorf("focus workspace %d: %w", wsID, err)
		}
	}
	if address == ms.Focused {
		if err := m.hypr.FocusWindow(address); err != nil {
			return "", fmt.Errorf("focus %s: %w", address, err)
		}
		return fmt.Sprintf("monocle: already showing %s", address), nil
	}
	incoming, next, _ := zoom(*ms, address)
	return m.place(wsID, *ms, next, incoming)
}

// Closed presents the next window before ClearWindowState removes address.
func (m *Monocle) Closed(address string) error {
	for wsID, ms := range m.state.AllMonocle() {
		if ms.Focused != address {
			continue
		}
		next := ms.Drop(address)
		if !next.Valid() {
			return nil
		}
		if err := m.hypr.Present(next.Focused, strconv.Itoa(wsID)); err != nil {
			return fmt.Errorf("monocle present %s: %w", next.Focused, err)
		}
		if active, err := m.hypr.ActiveWorkspace(); err == nil && active == wsID {
			_ = m.hypr.FocusWindow(next.Focused)
			windows.CenterCursor(m.hypr)
		}
		return nil
	}
	return nil
}

// Refit updates monocle geometry without changing state.
func (m *Monocle) Refit() error {
	all := m.state.AllMonocle()
	if len(all) == 0 {
		return nil
	}
	clients, err := m.hypr.Clients()
	if err != nil {
		return err
	}
	w, h, err := windows.MonocleSize(m.hypr, m.state)
	if err != nil {
		return fmt.Errorf("monocle size: %w", err)
	}
	offset := m.state.Profile().Monocle
	var members []string
	for wsID, ms := range all {
		for _, addr := range ms.Order {
			c := windows.Find(clients, addr)
			if c == nil || !c.Floating {
				continue
			}
			if addr == ms.Focused && c.Workspace.ID == wsID || addr != ms.Focused && c.Workspace.Name == monoWorkspace(wsID) {
				members = append(members, addr)
			}
		}
	}
	if err := m.hypr.Rebox(members, [2]int{w, h}, [2]int{offset.OffsetX, offset.OffsetY}); err != nil {
		return fmt.Errorf("monocle refit: %w", err)
	}
	return nil
}

func (m *Monocle) current() (int, *state.MonocleState, error) {
	wsID, err := m.hypr.ActiveWorkspace()
	if err != nil {
		return 0, nil, err
	}
	return wsID, m.state.GetMonocle(wsID), nil
}

func swapRoles(ms state.MonocleState, displaced string) (string, state.MonocleState, bool) {
	if tb := ms.SavedThreeBody; tb != nil {
		ms.Master = tb.Shadow
		ms.SavedThreeBody = &state.ThreeBodyState{Master: tb.Shadow, Active: tb.Active, Shadow: tb.Master}
		return zoom(ms, tb.Shadow)
	}
	if ms.Focused != ms.Master {
		incoming := ms.Master
		ms.Master = ms.Focused
		return zoom(ms, incoming)
	}
	incoming := displaced
	if !ms.Parked(displaced) {
		i := slices.IndexFunc(ms.Order, ms.Parked)
		if i < 0 {
			return "", ms, false
		}
		incoming = ms.Order[i]
	}
	ms.Master = incoming
	return zoom(ms, incoming)
}

func shadowRoles(ms state.MonocleState) (string, state.MonocleState, bool) {
	tb := ms.SavedThreeBody
	if tb == nil {
		return "", ms, false
	}
	ms.SavedThreeBody = &state.ThreeBodyState{Master: tb.Master, Active: tb.Shadow, Shadow: tb.Active}
	return zoom(ms, tb.Shadow)
}

func neighbor(ms state.MonocleState, step int) string {
	n := len(ms.Order)
	i := slices.Index(ms.Order, ms.Focused)
	if n == 0 || i < 0 {
		return ""
	}
	return ms.Order[((i+step)%n+n)%n]
}

func zoom(ms state.MonocleState, incoming string) (string, state.MonocleState, bool) {
	if !slices.Contains(ms.Order, incoming) {
		return "", ms, false
	}
	ms.Focused = incoming
	return incoming, ms, true
}

func matchBody(ms state.MonocleState, clients []hypr.Window, class, title string) string {
	candidates := []string{ms.Focused}
	for _, addr := range ms.Order {
		if addr != ms.Focused {
			candidates = append(candidates, addr)
		}
	}
	for _, members := range []bool{true, false} {
		for _, addr := range candidates {
			if ms.SavedThreeBody.Has(addr) != members {
				continue
			}
			if w := windows.Find(clients, addr); w != nil && windows.MatchesTarget(w, class, title) {
				return addr
			}
		}
	}
	return ""
}

func (m *Monocle) rezoom(wsID int, ms, next state.MonocleState, incoming string) (string, error) {
	if incoming != ms.Focused {
		parked, err := m.isParked(wsID, incoming)
		if err != nil {
			return "", err
		}
		if !parked {
			return fmt.Sprintf("monocle: %s not parked on %s", incoming, monoWorkspace(wsID)), nil
		}
	}
	return m.place(wsID, ms, next, incoming)
}

func (m *Monocle) place(wsID int, ms, next state.MonocleState, incoming string) (string, error) {
	if incoming == ms.Focused {
		m.commit(wsID, ms, next)
		return fmt.Sprintf("monocle: ws%d, roles updated, showing %s", wsID, incoming), nil
	}
	if err := m.hypr.Rezoom(incoming, ms.Focused); err != nil {
		if clients, cerr := m.hypr.Clients(); cerr == nil && swapped(clients, wsID, incoming, ms.Focused) {
			m.commit(wsID, ms, next)
		}
		return "", fmt.Errorf("monocle rezoom %s: %w", incoming, err)
	}
	m.commit(wsID, ms, next)
	windows.CenterCursor(m.hypr)
	return fmt.Sprintf("monocle: ws%d, showing %s", wsID, incoming), nil
}

func swapped(clients []hypr.Window, wsID int, incoming, outgoing string) bool {
	in := windows.Find(clients, incoming)
	out := windows.Find(clients, outgoing)
	return in != nil && in.Workspace.ID == wsID && (out == nil || out.Workspace.Name == monoWorkspace(wsID))
}

func (m *Monocle) commit(wsID int, ms, next state.MonocleState) {
	if next.SavedThreeBody == nil && next.Master != ms.Master {
		m.state.SetDisplacedMaster(wsID, ms.Master)
	}
	m.state.SetMonocle(wsID, &next)
}

func (m *Monocle) isParked(wsID int, address string) (bool, error) {
	clients, err := m.hypr.Clients()
	if err != nil {
		return false, err
	}
	w := windows.Find(clients, address)
	return w != nil && w.Workspace.Name == monoWorkspace(wsID), nil
}

// activate records cycle order and exit roles, then gives all members one box.
func (m *Monocle) activate(wsID int) (string, error) {
	clients, err := m.hypr.Clients()
	if err != nil {
		return "", err
	}
	active, err := m.hypr.ActiveWindow()
	if err != nil {
		return "", err
	}
	if active == nil {
		return "monocle: no active window", nil
	}
	tiled := windows.Tiled(clients, wsID)
	if len(tiled) == 0 {
		return "monocle: no tiled windows", nil
	}
	if !slices.ContainsFunc(tiled, func(w hypr.Window) bool { return w.Address == active.Address }) {
		return "monocle: active window is not tiled", nil
	}

	slaves := windows.GetSlaves(tiled)
	ordered := append([]hypr.Window{tiled[0]}, slaves...)
	for _, w := range tiled[1:] {
		if !slices.ContainsFunc(slaves, func(s hypr.Window) bool { return s.Address == w.Address }) {
			ordered = append(ordered, w)
		}
	}
	var saved *state.ThreeBodyState
	tb := m.state.GetThreeBody(wsID)
	if shadow := windows.Parked(tb, clients); shadow != nil {
		ordered = append(ordered, *shadow)
		if len(slaves) > 0 {
			saved = &state.ThreeBodyState{Master: tiled[0].Address, Active: slaves[0].Address, Shadow: shadow.Address}
		}
	}

	order := make([]string, 0, len(ordered))
	var parked []hypr.Window
	for _, w := range ordered {
		order = append(order, w.Address)
		if w.Address != active.Address {
			parked = append(parked, w)
		}
	}

	w, h, err := windows.MonocleSize(m.hypr, m.state)
	if err != nil {
		return "", fmt.Errorf("monocle size: %w", err)
	}
	offset := m.state.Profile().Monocle
	if err := m.hypr.Zoom(active.Address, parked, monoWorkspace(wsID), [2]int{w, h}, [2]int{offset.OffsetX, offset.OffsetY}); err != nil {
		return "", fmt.Errorf("monocle zoom: %w", err)
	}
	windows.CenterCursor(m.hypr)

	if tb != nil {
		m.state.ClearThreeBody(wsID)
	}
	m.state.SetMonocle(wsID, &state.MonocleState{
		Order:          order,
		Focused:        active.Address,
		Master:         tiled[0].Address,
		SavedThreeBody: saved,
	})
	return fmt.Sprintf("monocle: ws%d, %d windows hidden", wsID, len(parked)), nil
}

// deactivate restores tiling and saved roles, leaving any shadow parked.
// Without saved state, it tiles windows found on special:mono<n>.
func (m *Monocle) deactivate(wsID int) (string, error) {
	ms := m.state.GetMonocle(wsID)
	orphans, err := m.parked(wsID)
	if err != nil {
		return "", err
	}
	ws := strconv.Itoa(wsID)
	if ms != nil {
		clients, err := m.hypr.Clients()
		if err != nil {
			return "", err
		}
		for _, addr := range ms.Order {
			if windows.Find(clients, addr) == nil {
				*ms = ms.Drop(addr)
			}
		}
		if !ms.Valid() {
			m.state.ClearMonocle(wsID)
			ms = nil
		}
	}
	if ms == nil {
		var errs []error
		for _, addr := range orphans {
			if err := m.hypr.Unpark(addr, ws); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", addr, err))
			}
		}
		if len(orphans) == 0 {
			return "", nil
		}
		if len(errs) > 0 {
			return "", fmt.Errorf("monocle restore ws%d: %w", wsID, errors.Join(errs...))
		}
		return fmt.Sprintf("monocle off: ws%d, %d orphans restored", wsID, len(orphans)), nil
	}

	tb := ms.SavedThreeBody
	if tb != nil && ms.Focused == tb.Shadow {
		tb = &state.ThreeBodyState{Master: tb.Master, Active: tb.Shadow, Shadow: tb.Active}
	}
	var errs []error
	if tb != nil {
		if err := m.hypr.MoveWindowToWorkspace(tb.Shadow, windows.ShadowWorkspace, false); err != nil {
			errs = append(errs, fmt.Errorf("park %s: %w", tb.Shadow, err))
			tb = nil
		}
	}
	tile := []string{ms.Master}
	for _, addr := range slices.Concat(ms.Order, orphans) {
		if slices.Contains(tile, addr) || tb != nil && addr == tb.Shadow {
			continue
		}
		tile = append(tile, addr)
	}
	if err := m.hypr.Unzoom(ws, ms.Focused, tile); err != nil {
		return "", fmt.Errorf("monocle restore ws%d: %w", wsID, errors.Join(append(errs, err)...))
	}
	if m.ensureMaster(wsID, ms.Master) {
		_ = m.hypr.FocusWindow(ms.Focused)
	}
	if tb != nil {
		if err := windows.Fit(m.hypr, tb.Shadow, wsID); err != nil {
			errs = append(errs, fmt.Errorf("fit %s: %w", tb.Shadow, err))
		}
		m.state.SetThreeBody(wsID, tb)
	}
	m.state.ClearMonocle(wsID)

	if len(errs) > 0 {
		return "", fmt.Errorf("monocle restore ws%d: %w", wsID, errors.Join(errs...))
	}
	return fmt.Sprintf("monocle off: ws%d, %d windows restored", wsID, len(tile)-1), nil
}

func monoWorkspace(wsID int) string {
	return fmt.Sprintf("special:mono%d", wsID)
}

// IsMonoWorkspace recognizes the special:mono<n> parking workspace names.
func IsMonoWorkspace(name string) bool {
	id, ok := strings.CutPrefix(name, "special:mono")
	_, err := strconv.Atoi(id)
	return ok && err == nil
}

// parked returns the address of every client on special:mono{n}.
func (m *Monocle) parked(wsID int) ([]string, error) {
	clients, err := m.hypr.Clients()
	if err != nil {
		return nil, err
	}
	name := monoWorkspace(wsID)
	var out []string
	for _, c := range clients {
		if c.Workspace.Name == name && c.Address != "" {
			out = append(out, c.Address)
		}
	}
	return out, nil
}

// ensureMaster attempts to restore the master slot and reports whether it tried.
func (m *Monocle) ensureMaster(wsID int, masterAddr string) bool {
	if masterAddr == "" {
		return false
	}
	tiled, err := windows.GetTiledWindows(m.hypr, wsID)
	if err != nil || len(tiled) == 0 || tiled[0].Address == masterAddr {
		return false
	}
	_ = m.hypr.FocusWindow(masterAddr)
	_ = m.hypr.LayoutMsg("swapwithmaster master")
	return true
}
