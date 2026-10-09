package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/daemon"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
	"dotfiles/cmds/internal/hyprd/wm"
)

// EventLoop mirrors Hyprland's event stream into daemon state and notifies subscribers.
type EventLoop struct {
	hypr            *hypr.Client
	state           *state.State
	subs            *daemon.SubscriptionManager
	accent          *Accent
	done            <-chan struct{}
	browserQAPlaced map[string]bool
}

func NewEventLoop(hypr *hypr.Client, state *state.State, subs *daemon.SubscriptionManager, accent *Accent, done <-chan struct{}) *EventLoop {
	return &EventLoop{
		hypr:            hypr,
		state:           state,
		subs:            subs,
		accent:          accent,
		done:            done,
		browserQAPlaced: make(map[string]bool),
	}
}

// Run dials Hyprland's event socket, seeds state, then dispatches events until shutdown.
func (e *EventLoop) Run() error {
	socketPath := e.hypr.EventSocketPath()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to event socket: %w", err)
	}
	defer conn.Close()

	fmt.Printf("hyprd: subscribed to hyprland events\n")

	if err := e.syncState(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: initial sync failed: %v\n", err)
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-e.done:
			return nil
		default:
		}

		e.handleEvent(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-e.done:
			return nil
		default:
			return fmt.Errorf("event read error: %w", err)
		}
	}

	return nil
}

func (e *EventLoop) syncState() error {
	data, err := e.hypr.Request("j/activeworkspace")
	if err != nil {
		return err
	}

	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(data, &ws); err == nil {
		e.state.SetWorkspace(ws.ID)
	}

	if err := e.refreshClients(); err != nil {
		return err
	}
	e.notifyWorkspace()
	e.resetAccent()
	e.state.LockLayout()
	e.reseedSplit()
	e.refit()
	e.state.UnlockLayout()

	return nil
}

func (e *EventLoop) refreshClients() error {
	clients, err := e.hypr.Clients()
	if err != nil {
		return err
	}

	wsSet := make(map[int]bool, len(clients))
	for _, c := range clients {
		if c.Workspace.ID > 0 {
			wsSet[c.Workspace.ID] = true
		}
	}

	e.state.SetOccupied(slices.Sorted(maps.Keys(wsSet)))
	e.syncBrowserQA(clients)
	return nil
}

// handleEvent dispatches one Hyprland event-socket line: `event>>data`.
func (e *EventLoop) handleEvent(line string) {
	event, data, ok := strings.Cut(line, ">>")
	if !ok {
		return
	}

	switch event {
	case "workspace", "workspacev2":
		wsStr := data
		if idx := strings.Index(data, ","); idx > 0 {
			wsStr = data[:idx]
		}
		if ws, err := strconv.Atoi(wsStr); err == nil {
			e.state.SetWorkspace(ws)
			e.notifyWorkspace()
			e.resetAccent()
			e.state.LockLayout()
			e.reseedSplit()
			e.refreshSplit()
			if event == "workspacev2" {
				e.refitWorkspace(ws)
			}
			e.state.UnlockLayout()
		}

	case "focusedmon":
		if idx := strings.LastIndex(data, ","); idx >= 0 {
			if ws, err := strconv.Atoi(data[idx+1:]); err == nil {
				e.state.SetWorkspace(ws)
				e.notifyWorkspace()
				e.resetAccent()
				e.state.LockLayout()
				e.reseedSplit()
				e.refreshSplit()
				e.state.UnlockLayout()
			}
		}

	case "activewindow", "activewindowv2":
		e.applyAccent()
		e.state.LockLayout()
		e.refreshSplit()
		e.state.UnlockLayout()

	case "activespecial":
		if name, _, _ := strings.Cut(data, ","); name == windows.ShadowWorkspace {
			e.state.LockLayout()
			e.closeShadow()
			e.state.UnlockLayout()
		}

	case "configreloaded":
		if e.accent != nil {
			e.accent.Invalidate()
		}
		e.applyAccent()
		e.state.LockLayout()
		e.reseedSplit()
		e.refit()
		e.state.UnlockLayout()

	case "createworkspace", "destroyworkspace":
		e.refreshClients()
		e.notifyWorkspace()

	case "destroyworkspacev2":
		id, _, _ := strings.Cut(data, ",")
		if ws, err := strconv.Atoi(id); err == nil {
			e.state.ClearSplitMark(ws)
		}

	case "openwindow":
		e.state.LockLayout()
		e.autoPark(data)
		e.refit()
		e.state.UnlockLayout()
		e.refreshClients()
		e.notifyWorkspace()

	case "movewindowv2":
		e.state.LockLayout()
		e.refit()
		e.state.UnlockLayout()
		e.refreshClients()
		e.notifyWorkspace()

	case "movewindow", "windowtitle", "windowtitlev2":
		e.refreshClients()
		e.notifyWorkspace()

	case "closewindow":
		addr := hexAddress(data)
		e.state.LockLayout()
		e.handleThreeBodyClose(addr) // must run before ClearWindowState wipes the entries
		e.handleMonocleClose(addr)
		e.state.ClearWindowState(addr)
		e.refit()
		e.state.UnlockLayout()
		e.refreshClients()
		e.notifyWorkspace()
		e.applyAccent()
	}
}

func (e *EventLoop) applyAccent() {
	if e.accent == nil {
		return
	}
	if err := e.accent.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd accent: %v\n", err)
	}
}

// refreshSplit reapplies the active workspace's split preset when its mark is stale.
func (e *EventLoop) refreshSplit() {
	if err := wm.NewSplit(e.hypr, e.state).Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd split: %v\n", err)
	}
}

// reseedSplit points master.mfact at the active workspace's preset after startup, a focus change, or a config reload.
func (e *EventLoop) reseedSplit() {
	if err := wm.NewSplit(e.hypr, e.state).Reseed(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd split: %v\n", err)
	}
}

func (e *EventLoop) resetAccent() {
	if e.accent == nil {
		return
	}
	if err := e.accent.Reset(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd accent: %v\n", err)
	}
}

// closeShadow keeps the shadow workspace from ever being on screen.
//
// Focusing a parked window makes Hyprland open its special workspace over the current one, so the window that took focus is shown in its own workspace instead.
func (e *EventLoop) closeShadow() {
	active, err := e.hypr.ActiveWindow()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd shadow: %v\n", err)
		return
	}
	monitors, err := e.hypr.Monitors()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd shadow: %v\n", err)
		return
	}
	if slices.ContainsFunc(monitors, func(m hypr.Monitor) bool { return m.Focused && m.SpecialWS.Name == windows.ShadowWorkspace }) {
		if err := e.hypr.ToggleSpecialWorkspace(strings.TrimPrefix(windows.ShadowWorkspace, "special:")); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd shadow: close: %v\n", err)
		}
	}
	if active == nil || active.Workspace.Name != windows.ShadowWorkspace {
		return
	}
	if err := wm.NewThreeBody(e.hypr, e.state).Show(active.Address, 0); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd shadow: show %s: %v\n", active.Address, err)
	}
}

// handleThreeBodyClose dissolves a three-body triple when any member closes, pulling the shadow back if needed.
func (e *EventLoop) handleThreeBodyClose(addr string) {
	for ws, tb := range e.state.AllThreeBody() {
		if !tb.Has(addr) {
			continue
		}
		e.state.ClearThreeBody(ws)
		clients, err := e.hypr.Clients()
		if err != nil {
			fmt.Fprintf(os.Stderr, "hyprd three-body close: %v\n", err)
			return
		}
		if shadow := windows.Parked(tb, clients); shadow != nil && shadow.Address != addr {
			if err := windows.Unpark(e.hypr, shadow.Address, ws); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd three-body close: unpark %s: %v\n", shadow.Address, err)
			}
		}
		return
	}
}

func (e *EventLoop) autoPark(data string) {
	addr, rest, _ := strings.Cut(data, ",")
	name, _, _ := strings.Cut(rest, ",")
	wsID, err := strconv.Atoi(name)
	if err != nil || wsID < 1 || wsID > 5 || e.state.Arranging(wsID) || e.state.GetMonocle(wsID) != nil {
		return
	}
	addr = hexAddress(addr)
	clients, err := e.hypr.Clients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd auto-park: %v\n", err)
		return
	}
	i := slices.IndexFunc(clients, func(c hypr.Window) bool { return c.Address == addr })
	if i < 0 || clients[i].Floating || clients[i].Workspace.ID != wsID || windows.IsIgnored(clients[i].Class) {
		return
	}
	if windows.Parked(e.state.GetThreeBody(wsID), clients) != nil {
		return
	}
	tiled := windows.Tiled(clients, wsID)
	slaves := windows.GetSlaves(tiled)
	j := windows.SlaveIndex(slaves, addr)
	if len(tiled) != 3 || len(slaves) != 2 || j < 0 {
		return
	}
	prev := slaves[1-j].Address
	if err := windows.Park(e.hypr, slaves[1-j], wsID); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd auto-park: park %s: %v\n", prev, err)
		return
	}
	if err := e.hypr.FocusWindow(addr); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd auto-park: focus %s: %v\n", addr, err)
	}
	e.state.SetThreeBody(wsID, &state.ThreeBodyState{Master: tiled[0].Address, Active: addr, Shadow: prev})
}

func (e *EventLoop) refit() {
	if err := windows.RefitAll(e.hypr, e.state); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd refit: %v\n", err)
	}
}

func (e *EventLoop) refitWorkspace(ws int) {
	if err := windows.Refit(e.hypr, e.state, ws); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd refit: %v\n", err)
	}
}

func hexAddress(addr string) string {
	if strings.HasPrefix(addr, "0x") {
		return addr
	}
	return "0x" + addr
}

// handleMonocleClose restores displaced windows when the focused monocle window closes.
func (e *EventLoop) handleMonocleClose(addr string) {
	for ws, ms := range e.state.AllMonocle() {
		if ms.Focused != addr {
			continue
		}
		for _, mw := range ms.Windows {
			e.hypr.MoveWindowToWorkspace(mw.Address, strconv.Itoa(mw.OriginWS), false)
		}
		e.state.ClearMonocle(ws)
		return
	}
}

func (e *EventLoop) notifyWorkspace() {
	if e.subs == nil {
		return
	}
	e.subs.Notify("workspace", workspacePayload(e.state))
}
