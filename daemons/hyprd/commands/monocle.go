package commands

import (
	"dotfiles/daemons/config"
	"encoding/json"
	"fmt"

	"dotfiles/daemons/hyprd/hypr"
)

// Monocle isolates the focused window by moving all other tiled windows
// to per-workspace special workspaces, then floating and resizing the
// focused window to a configured monocle size. Toggling off unfloats
// and restores all displaced windows across all workspaces.
type Monocle struct {
	hypr  *hypr.Client
	state StateManager
}

// NewMonocle creates a Monocle command handler.
func NewMonocle(h *hypr.Client, s StateManager) *Monocle {
	return &Monocle{hypr: h, state: s}
}

// Execute toggles monocle mode. If any workspace has monocle active, all
// displaced windows are restored. Otherwise, all non-focused tiled windows
// on the current workspace are moved to a temp special workspace.
func (m *Monocle) Execute() (string, error) {
	if m.state.HasAnyMonocle() {
		return m.deactivateAll()
	}
	return m.activate()
}

// activate moves all tiled windows except the focused one to special:mono<ws>,
// then floats and resizes the focused window to monocle dimensions.
func (m *Monocle) activate() (string, error) {
	wsID, err := m.activeWorkspace()
	if err != nil {
		return "", err
	}

	cfg := m.state.GetConfig()

	// Save and dissolve three-body if active
	var savedTB *ThreeBodyState
	if tb := m.state.GetThreeBody(wsID); tb != nil {
		m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, tb.Shadow))
		m.state.ClearThreeBody(wsID)
		savedTB = tb
	}

	tiled, err := GetTiledWindows(m.hypr, wsID, cfg.Windows.IgnoredClasses)
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
	var displaced []MonocleWindow

	for _, w := range tiled {
		if w.Address == active.Address {
			continue
		}
		if err := m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
			monoWS, w.Address)); err != nil {
			return "", fmt.Errorf("monocle hide %s: %w", w.Address, err)
		}
		displaced = append(displaced, MonocleWindow{
			Address:  w.Address,
			OriginWS: wsID,
		})
	}

	// Float, resize, center, and offset the focused window
	w, h := cfg.MonocleSize()
	ox, oy := cfg.MonocleOffset()
	batch := fmt.Sprintf(
		"dispatch togglefloating; "+
			"dispatch resizeactive exact %d %d; "+
			"dispatch centerwindow; "+
			"dispatch moveactive %d %d",
		w, h, ox, oy,
	)
	m.hypr.Request("[[BATCH]]" + batch)
	centerCursor(m.hypr)

	m.state.SetMonocle(wsID, &MonocleState{
		Focused:        active.Address,
		Master:         master,
		Windows:        displaced,
		SavedThreeBody: savedTB,
	})
	return fmt.Sprintf("monocle: ws%d, %d windows hidden", wsID, len(displaced)), nil
}

// deactivateAll unfloats monocled windows, restores displaced windows,
// and re-enrolls three-body with the exact saved state.
func (m *Monocle) deactivateAll() (string, error) {
	all := m.state.AllMonocle()
	restored := 0
	cfg := m.state.GetConfig()

	for ws, ms := range all {
		// Step 1: unfloat the monocled window
		if ms.Focused != "" {
			m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
			m.hypr.Dispatch("togglefloating")
		}

		// Step 2: restore all displaced windows
		for _, mw := range ms.Windows {
			m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s",
				mw.OriginWS, mw.Address))
			restored++
		}

		// Step 3: fix master position — all windows are now tiled
		m.ensureMaster(ws, ms.Master, cfg)

		// Step 4: restore three-body with saved state
		if ms.SavedThreeBody != nil {
			m.restoreThreeBody(ws, ms.SavedThreeBody, cfg)
		}

		// Step 5: refocus the window that was monocled
		if ms.Focused != "" {
			m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
		}

		m.state.ClearMonocle(ws)
	}

	return fmt.Sprintf("monocle off: %d windows restored", restored), nil
}

// ensureMaster puts the desired window in master position if it isn't already.
func (m *Monocle) ensureMaster(wsID int, masterAddr string, cfg *config.HyprConfig) {
	if masterAddr == "" {
		return
	}

	tiled, err := GetTiledWindows(m.hypr, wsID, cfg.Windows.IgnoredClasses)
	if err != nil || len(tiled) == 0 {
		return
	}

	if tiled[0].Address == masterAddr {
		return // already master
	}

	// Focus the desired master (currently a slave) and swap it into master
	m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", masterAddr))
	m.hypr.Dispatch("layoutmsg swapwithmaster master")
}

// restoreThreeBody re-enrolls three-body with the exact saved addresses.
func (m *Monocle) restoreThreeBody(wsID int, saved *ThreeBodyState, cfg *config.HyprConfig) {
	// Hide the saved shadow window
	m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		cfg.Windows.ShadowWorkspace, saved.Shadow))

	// Focus the active slave
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
