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

	// Dissolve three-body if active (restore shadow to workspace first)
	hadThreeBody := false
	if tb := m.state.GetThreeBody(wsID); tb != nil {
		m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", wsID, tb.Shadow))
		m.state.ClearThreeBody(wsID)
		hadThreeBody = true
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

	// Track original master before displacing windows
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

	// Float and resize the focused window to monocle dimensions
	w := cfg.MonocleWidth()
	h := cfg.MonocleHeight()
	batch := fmt.Sprintf(
		"dispatch togglefloating; "+
			"dispatch resizeactive exact %d %d; "+
			"dispatch centerwindow",
		w, h,
	)
	m.hypr.Request("[[BATCH]]" + batch)
	centerCursor(m.hypr)

	m.state.SetMonocle(wsID, &MonocleState{
		Focused:      active.Address,
		Master:       master,
		Windows:      displaced,
		HadThreeBody: hadThreeBody,
	})
	return fmt.Sprintf("monocle: ws%d, %d windows hidden", wsID, len(displaced)), nil
}

// deactivateAll unfloats monocled windows and restores all displaced windows.
// If three-body was active before monocle, re-enrolls by detecting current positions.
func (m *Monocle) deactivateAll() (string, error) {
	all := m.state.AllMonocle()
	restored := 0

	cfg := m.state.GetConfig()

	for ws, ms := range all {
		// Unfloat the focused window first
		if ms.Focused != "" {
			m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", ms.Focused))
			m.hypr.Dispatch("togglefloating")
		}

		// Restore displaced windows
		for _, mw := range ms.Windows {
			if err := m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s",
				mw.OriginWS, mw.Address)); err != nil {
				return "", fmt.Errorf("monocle restore %s: %w", mw.Address, err)
			}
			restored++
		}

		// Ensure original master is back in master position (leftmost).
		// After unfloat + restore, the focused window may have taken master.
		m.restoreMasterPosition(ws, ms.Master, cfg)

		// Re-enroll three-body by detecting actual positions
		if ms.HadThreeBody {
			m.reenrollThreeBody(ws, ms.Focused, cfg)
		}

		m.state.ClearMonocle(ws)
	}

	return fmt.Sprintf("monocle off: %d windows restored", restored), nil
}

// restoreMasterPosition ensures the original master window is in master (leftmost) position.
// After monocle deactivation, the unfloated window may have taken master position.
func (m *Monocle) restoreMasterPosition(wsID int, masterAddr string, cfg *config.HyprConfig) {
	if masterAddr == "" {
		return
	}

	tiled, err := GetTiledWindows(m.hypr, wsID, cfg.Windows.IgnoredClasses)
	if err != nil || len(tiled) == 0 {
		return
	}

	// Already in master position
	if tiled[0].Address == masterAddr {
		return
	}

	// Focus the original master and swap it into master position
	m.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", masterAddr))
	m.hypr.Dispatch("layoutmsg swapwithmaster master")
}

// reenrollThreeBody detects current tiling positions and re-enrolls three-body.
// The focused (monocled) window becomes the active slave if it's not master.
func (m *Monocle) reenrollThreeBody(wsID int, focused string, cfg *config.HyprConfig) {
	tiled, err := GetTiledWindows(m.hypr, wsID, cfg.Windows.IgnoredClasses)
	if err != nil || len(tiled) != 3 {
		return
	}

	master := tiled[0]
	slaves := GetSlaves(tiled)
	if len(slaves) != 2 {
		return
	}

	// Determine active/shadow: the focused window stays visible, the other hides
	var active, shadow *hypr.Window
	if master.Address == focused {
		// Focused is master — pick first slave as active, second as shadow
		active = &slaves[0]
		shadow = &slaves[1]
	} else {
		for i := range slaves {
			if slaves[i].Address == focused {
				active = &slaves[i]
			} else {
				shadow = &slaves[i]
			}
		}
		// Focused not found in slaves (closed?) — pick defaults
		if active == nil {
			active = &slaves[0]
			shadow = &slaves[1]
		}
	}

	if shadow == nil {
		return
	}

	m.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %s,address:%s",
		cfg.Windows.ShadowWorkspace, shadow.Address))

	m.state.SetThreeBody(wsID, &ThreeBodyState{
		Master: master.Address,
		Active: active.Address,
		Shadow: shadow.Address,
	})
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
