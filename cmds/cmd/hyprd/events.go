package main

// events.go consumes Hyprland event-socket lines and mirrors them into state plus subscriber updates.

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
)

// EventLoop mirrors Hyprland's event stream into daemon state and notifies subscribers.
type EventLoop struct {
	hypr  *hypr.Client
	state *state.State
	subs  *daemon.SubscriptionManager
	done  <-chan struct{}
}

func NewEventLoop(hypr *hypr.Client, state *state.State, subs *daemon.SubscriptionManager, done <-chan struct{}) *EventLoop {
	return &EventLoop{
		hypr:  hypr,
		state: state,
		subs:  subs,
		done:  done,
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

	if err := e.updateOccupied(); err != nil {
		return err
	}

	return nil
}

func (e *EventLoop) updateOccupied() error {
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
	return nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Event dispatch                                                               │
// ╰──────────────────────────────────────────────────────────────────────────────╯

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
		}

	case "focusedmon":
		if idx := strings.LastIndex(data, ","); idx >= 0 {
			if ws, err := strconv.Atoi(data[idx+1:]); err == nil {
				e.state.SetWorkspace(ws)
				e.notifyWorkspace()
			}
		}

	case "createworkspace", "destroyworkspace", "openwindow", "movewindow":
		e.updateOccupied()
		e.notifyWorkspace()

	case "closewindow":
		addr := data // closewindow emits bare hex (no 0x prefix)
		if !strings.HasPrefix(addr, "0x") {
			addr = "0x" + addr
		}
		e.handleThreeBodyClose(addr) // must run before ClearWindowState wipes the entries
		e.handleMonocleClose(addr)
		e.state.ClearWindowState(addr)
		e.updateOccupied()
		e.notifyWorkspace()
	}
}

// handleThreeBodyClose dissolves a three-body triple when any member closes, pulling the shadow back if needed.
func (e *EventLoop) handleThreeBodyClose(addr string) {
	for ws, tb := range e.state.AllThreeBody() {
		if tb.Shadow == addr {
			e.state.ClearThreeBody(ws)
			return
		}
		if tb.Active == addr || tb.Master == addr {
			e.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", ws, tb.Shadow))
			e.state.ClearThreeBody(ws)
			return
		}
	}
}

// handleMonocleClose restores displaced windows when the focused monocle window closes.
func (e *EventLoop) handleMonocleClose(addr string) {
	for ws, ms := range e.state.AllMonocle() {
		if ms.Focused != addr {
			continue
		}
		for _, mw := range ms.Windows {
			e.hypr.Dispatch(fmt.Sprintf("movetoworkspacesilent %d,address:%s", mw.OriginWS, mw.Address))
		}
		e.state.ClearMonocle(ws)
		return
	}
}

func (e *EventLoop) notifyWorkspace() {
	if e.subs == nil {
		return
	}
	current := e.state.GetWorkspace()
	occupied := e.state.GetOccupied()

	data := map[string]any{
		"current":      current,
		"current_str":  strconv.Itoa(current),
		"occupied":     occupied,
		"occupied_str": joinWorkspaceIDs(occupied),
	}
	e.subs.Notify("workspace", data)
}
