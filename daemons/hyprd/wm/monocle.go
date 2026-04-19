package wm

// monocle.go toggles monocle mode by floating the focused window and parking sibling windows per workspace.

import (
	"encoding/json"
	"fmt"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

// Monocle zooms the active window to a configured size, parking siblings until toggled off.
// Three-body state is saved and restored around the monocle lifecycle.
type Monocle struct {
	hypr  *hypr.Client
	state *state.State
}

func NewMonocle(h *hypr.Client, s *state.State) *Monocle {
	return &Monocle{hypr: h, state: s}
}

// Execute toggles monocle on the current workspace.
func (m *Monocle) Execute() (string, error) {
	wsID, err := m.activeWorkspace()
	if err != nil {
		return "", err
	}
	if m.state.GetMonocle(wsID) != nil {
		return m.deactivate(wsID)
	}
	return m.activate()
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ activate / deactivate                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// activate floats the active window at monocle geometry and parks siblings, saving any three-body state.
func (m *Monocle) activate() (string, error) {
	wsID, err := m.activeWorkspace()
	if err != nil {
		return "", err
	}

	cfg := m.state.GetConfig()
	var savedTB *state.ThreeBodyState
	if tb := m.state.GetThreeBody(wsID); tb != nil {
		m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, tb.Shadow))
		m.state.ClearThreeBody(wsID)
		savedTB = tb
	}

	tiled, err := windows.GetTiledWindows(m.hypr, wsID, cfg.Windows.IgnoredClasses)
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
	monoWS := fmt.Sprintf("special:mono%d", wsID)
	var displaced []state.MonocleWindow
	for _, w := range tiled {
		if w.Address == active.Address {
			continue
		}
		if err := m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", monoWS, w.Address)); err != nil {
			return "", fmt.Errorf("monocle hide %s: %w", w.Address, err)
		}
		displaced = append(displaced, state.MonocleWindow{Address: w.Address, OriginWS: wsID})
	}

	w, h := cfg.MonocleSize()
	ox, oy := cfg.MonocleOffset()
	batch := fmt.Sprintf("dispatch togglefloating; dispatch resizeactive exact %d %d; dispatch centerwindow; dispatch moveactive %d %d", w, h, ox, oy)
	m.hypr.Request("[[BATCH]]" + batch)
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
func (m *Monocle) deactivate(wsID int) (string, error) {
	ms := m.state.GetMonocle(wsID)
	if ms == nil {
		return "", nil
	}
	cfg := m.state.GetConfig()

	if ms.Focused != "" {
		m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
		m.hypr.Dispatch("togglefloating")
	}
	for _, mw := range ms.Windows {
		m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", mw.OriginWS, mw.Address))
	}
	m.ensureMaster(wsID, ms.Master, cfg)
	if ms.SavedThreeBody != nil {
		m.restoreThreeBody(wsID, ms.SavedThreeBody, cfg)
	}
	m.restoreSplitRatio(ms.SavedSplitRatio, cfg)
	if ms.Focused != "" {
		m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
	}
	m.state.ClearMonocle(wsID)

	return fmt.Sprintf("monocle off: ws%d, %d windows restored", wsID, len(ms.Windows)), nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ restore helpers                                                              │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// ensureMaster swaps the saved master back to position 0 if Hyprland re-tiled in a different order.
func (m *Monocle) ensureMaster(wsID int, masterAddr string, cfg *config.HyprConfig) {
	if masterAddr == "" {
		return
	}
	tiled, err := windows.GetTiledWindows(m.hypr, wsID, cfg.Windows.IgnoredClasses)
	if err != nil || len(tiled) == 0 {
		return
	}
	if tiled[0].Address == masterAddr {
		return
	}
	m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", masterAddr))
	m.hypr.Dispatch("layoutmsg swapwithmaster master")
}

func (m *Monocle) restoreSplitRatio(ratio string, cfg *config.HyprConfig) {
	if ratio == "" {
		ratio = "default"
	}
	var mfact string
	switch ratio {
	case "xs":
		mfact = cfg.Split.XS
	case "lg":
		mfact = cfg.Split.LG
	default:
		ratio = "default"
		mfact = cfg.Split.Default
	}
	m.hypr.Dispatch(fmt.Sprintf("layoutmsg mfact exact %s", mfact))
	m.state.SetSplitRatio(ratio)
}

func (m *Monocle) restoreThreeBody(wsID int, saved *state.ThreeBodyState, cfg *config.HyprConfig) {
	m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s", cfg.Windows.ShadowWorkspace, saved.Shadow))
	m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", saved.Active))
	m.state.SetThreeBody(wsID, saved)
}

func (m *Monocle) activeWorkspace() (int, error) {
	data, err := m.hypr.Request("j/activeworkspace")
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
