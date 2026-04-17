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

	"dotfiles/daemons/daemon"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

// EventLoop mirrors Hyprland's event stream into daemon state and fans out workspace changes to subscribers.
//
// Shutdown is driven exclusively by closing the done channel passed at construction.
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

// Run dials Hyprland's event socket, seeds state from a one-shot query, then dispatches events until shutdown.
//
// A scanner error during shutdown is treated as normal termination.
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

	// exclude special workspaces (id <= 0): scratchpads, shadow, etc.
	wsSet := make(map[int]bool)
	for _, c := range clients {
		if c.Workspace.ID > 0 {
			wsSet[c.Workspace.ID] = true
		}
	}

	e.state.SetOccupied(slices.Sorted(maps.Keys(wsSet)))
	return nil
}

// handleEvent parses an "event>>data" line and applies it to state.
// Payload schema: https://wiki.hypr.land/IPC/ — unrecognized events are dropped.
func (e *EventLoop) handleEvent(line string) {
	parts := strings.SplitN(line, ">>", 2)
	if len(parts) != 2 {
		return
	}

	event, data := parts[0], parts[1]

	switch event {
	case "workspace", "workspacev2":
		// workspacev2 payload is "id,name"; workspace is just "id".
		wsStr := data
		if idx := strings.Index(data, ","); idx > 0 {
			wsStr = data[:idx]
		}
		if ws, err := strconv.Atoi(wsStr); err == nil {
			e.state.SetWorkspace(ws)
			e.notifyWorkspace()
		}

	case "focusedmon":
		// payload is "monname,workspaceid".
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
		// closewindow emits bare hex (no 0x prefix).
		addr := data
		if !strings.HasPrefix(addr, "0x") {
			addr = "0x" + addr
		}
		// Three-body / monocle cleanup must run before ClearWindowState wipes the entries they read.
		e.handleThreeBodyClose(addr)
		e.handleMonocleClose(addr)
		e.state.ClearWindowState(addr)
		e.updateOccupied()
		e.notifyWorkspace()
	}
}

// handleThreeBodyClose dissolves a three-body triple when any member closes.
// If a visible member (active/master) closes, the hidden shadow is pulled back before dissolving.
//
// Caller contract: run before ClearWindowState, which wipes the entries this reads.
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
// Hidden members that close on the special workspace are swept later by ClearWindowState.
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
