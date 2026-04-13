package wm

import (
	"encoding/json"
	"fmt"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/windows"
)

type Monocle struct {
	hypr  *hypr.Client
	state *state.State
}

func NewMonocle(h *hypr.Client, s *state.State) *Monocle {
	return &Monocle{hypr: h, state: s}
}

func (m *Monocle) Execute() (string, error) {
	if m.state.HasAnyMonocle() {
		return m.deactivateAll()
	}
	return m.activate()
}

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
	if len(tiled) < 2 {
		return "monocle: need at least 2 tiled windows", nil
	}

	active, err := m.hypr.ActiveWindow()
	if err != nil {
		return "", err
	}
	if active == nil {
		return "monocle: no active window", nil
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
		Focused:        active.Address,
		Master:         master,
		Windows:        displaced,
		SavedThreeBody: savedTB,
	})
	return fmt.Sprintf("monocle: ws%d, %d windows hidden", wsID, len(displaced)), nil
}

func (m *Monocle) deactivateAll() (string, error) {
	all := m.state.AllMonocle()
	restored := 0
	cfg := m.state.GetConfig()

	for ws, ms := range all {
		if ms.Focused != "" {
			m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
			m.hypr.Dispatch("togglefloating")
		}
		for _, mw := range ms.Windows {
			m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", mw.OriginWS, mw.Address))
			restored++
		}
		m.ensureMaster(ws, ms.Master, cfg)
		if ms.SavedThreeBody != nil {
			m.restoreThreeBody(ws, ms.SavedThreeBody, cfg)
		}
		if ms.Focused != "" {
			m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
		}
		m.state.ClearMonocle(ws)
	}

	return fmt.Sprintf("monocle off: %d windows restored", restored), nil
}

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
