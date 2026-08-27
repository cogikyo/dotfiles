package wm

import (
	"errors"
	"fmt"
	"strconv"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
)

// Monocle zooms the active window to a configured size, parking siblings until toggled off.
//
// Three-body state is saved and restored around the monocle lifecycle.
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
	parked, err := m.parked(wsID)
	if err != nil {
		return "", err
	}
	if len(parked) > 0 {
		return m.deactivate(wsID)
	}
	return m.activate()
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

// activate floats the active window at monocle geometry and parks siblings, saving any three-body state.
func (m *Monocle) activate() (string, error) {
	wsID, err := m.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}

	cfg := m.state.GetConfig()
	var savedTB *state.ThreeBodyState
	if tb := m.state.GetThreeBody(wsID); tb != nil {
		_ = m.hypr.MoveWindowToWorkspace(tb.Shadow, strconv.Itoa(wsID), false)
		m.state.ClearThreeBody(wsID)
		savedTB = tb
	}

	tiled, err := windows.GetTiledWindows(m.hypr, wsID)
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
	if len(tiled) == 0 {
		return "monocle: no tiled windows", nil
	}

	master := tiled[0].Address
	monoWS := monoWorkspace(wsID)
	var displaced []state.MonocleWindow
	for _, w := range tiled {
		if w.Address == active.Address {
			continue
		}
		if err := m.hypr.MoveWindowToWorkspace(w.Address, monoWS, false); err != nil {
			return "", fmt.Errorf("monocle hide %s: %w", w.Address, err)
		}
		displaced = append(displaced, state.MonocleWindow{Address: w.Address, OriginWS: wsID})
	}

	w, h := cfg.MonocleSize()
	ox, oy := cfg.MonocleOffset()
	if err := m.hypr.ToggleFloatActive(); err != nil {
		return "", fmt.Errorf("monocle float: %w", err)
	}
	if err := m.hypr.ResizeActiveExact(w, h); err != nil {
		return "", fmt.Errorf("monocle resize: %w", err)
	}
	if err := m.hypr.CenterActive(); err != nil {
		return "", fmt.Errorf("monocle center: %w", err)
	}
	if err := m.hypr.MoveActiveRelative(ox, oy); err != nil {
		return "", fmt.Errorf("monocle offset: %w", err)
	}
	windows.CenterCursor(m.hypr)

	m.state.SetMonocle(wsID, &state.MonocleState{
		Focused:         active.Address,
		Master:          master,
		Windows:         displaced,
		SavedThreeBody:  savedTB,
		SavedSplitRatio: m.state.GetSplitRatio(),
	})
	return fmt.Sprintf("monocle: ws%d, %d windows hidden", wsID, len(displaced)), nil
}

// deactivate restores parked windows, master position, three-body state, and split ratio.
//
// Restore set is recorded Windows union clients on special:mono{n}.
// Orphan-only heal (nil receipt) yanks those clients back and skips unfloat/extras.
func (m *Monocle) deactivate(wsID int) (string, error) {
	ms := m.state.GetMonocle(wsID)
	parked, err := m.parked(wsID)
	if err != nil {
		return "", err
	}
	restore := restoreSet(ms, parked)
	if ms == nil && len(restore) == 0 {
		return "", nil
	}

	if ms != nil && ms.Focused != "" {
		_ = m.hypr.FocusWindow(ms.Focused)
		_ = m.hypr.ToggleFloatActive()
	}

	var errs []error
	moved := 0
	for _, mw := range restore {
		dest := mw.OriginWS
		if dest == 0 {
			dest = wsID
		}
		if err := m.hypr.MoveWindowToWorkspace(mw.Address, strconv.Itoa(dest), false); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", mw.Address, err))
			continue
		}
		moved++
	}
	if len(restore) > 0 && moved == 0 {
		return "", fmt.Errorf("monocle restore ws%d: %w", wsID, errors.Join(errs...))
	}

	if ms != nil {
		m.ensureMaster(wsID, ms.Master)
		if ms.SavedThreeBody != nil {
			m.restoreThreeBody(wsID, ms.SavedThreeBody)
		}
		m.restoreSplitRatio(ms.SavedSplitRatio, m.state.GetConfig())
		if ms.Focused != "" {
			_ = m.hypr.FocusWindow(ms.Focused)
		}
		m.state.ClearMonocle(wsID)
	}

	if len(errs) > 0 {
		return "", fmt.Errorf("monocle restore ws%d: %d/%d moved: %w", wsID, moved, len(restore), errors.Join(errs...))
	}
	return fmt.Sprintf("monocle off: ws%d, %d windows restored", wsID, moved), nil
}

func monoWorkspace(wsID int) string {
	return fmt.Sprintf("special:mono%d", wsID)
}

// parked returns every client whose workspace name is special:mono{n}.
func (m *Monocle) parked(wsID int) ([]state.MonocleWindow, error) {
	clients, err := m.hypr.Clients()
	if err != nil {
		return nil, err
	}
	name := monoWorkspace(wsID)
	var out []state.MonocleWindow
	for _, c := range clients {
		if c.Workspace.Name != name || c.Address == "" {
			continue
		}
		out = append(out, state.MonocleWindow{Address: c.Address, OriginWS: wsID})
	}
	return out, nil
}

// restoreSet unions receipt windows with clients found on special:mono{n}.
func restoreSet(ms *state.MonocleState, parked []state.MonocleWindow) []state.MonocleWindow {
	seen := make(map[string]struct{})
	var out []state.MonocleWindow
	if ms != nil {
		for _, mw := range ms.Windows {
			if mw.Address == "" {
				continue
			}
			if _, ok := seen[mw.Address]; ok {
				continue
			}
			seen[mw.Address] = struct{}{}
			out = append(out, mw)
		}
	}
	for _, mw := range parked {
		if mw.Address == "" {
			continue
		}
		if _, ok := seen[mw.Address]; ok {
			continue
		}
		seen[mw.Address] = struct{}{}
		out = append(out, mw)
	}
	return out
}

// ensureMaster swaps the saved master back to position 0 if Hyprland re-tiled in a different order.
func (m *Monocle) ensureMaster(wsID int, masterAddr string) {
	if masterAddr == "" {
		return
	}
	tiled, err := windows.GetTiledWindows(m.hypr, wsID)
	if err != nil || len(tiled) == 0 {
		return
	}
	if tiled[0].Address == masterAddr {
		return
	}
	_ = m.hypr.FocusWindow(masterAddr)
	_ = m.hypr.LayoutMsg("swapwithmaster master")
}

func (m *Monocle) restoreSplitRatio(ratio string, cfg *config.HyprConfig) {
	if ratio == "" {
		ratio = "default"
	}
	var mfact string
	switch ratio {
	case "xs":
		mfact = cfg.Windows.Split.XS
	case "lg":
		mfact = cfg.Windows.Split.LG
	default:
		ratio = "default"
		mfact = cfg.Windows.Split.Default
	}
	_ = m.hypr.LayoutMsg(fmt.Sprintf("mfact exact %s", mfact))
	m.state.SetSplitRatio(ratio)
}

func (m *Monocle) restoreThreeBody(wsID int, saved *state.ThreeBodyState) {
	_ = m.hypr.MoveWindowToWorkspace(saved.Shadow, windows.ShadowWorkspace, false)
	_ = m.hypr.FocusWindow(saved.Active)
	m.state.SetThreeBody(wsID, saved)
}
