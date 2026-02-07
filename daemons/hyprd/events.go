package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/daemon"
)

// EventLoop synchronizes daemon state by listening to Hyprland's event socket
// and notifying subscribers of workspace changes.
type EventLoop struct {
	hypr  *hypr.Client                    // Hyprland IPC client
	state *State                          // Shared daemon state
	subs  *daemon.SubscriptionManager     // Pub/sub for workspace events
	done  <-chan struct{}                 // Shutdown signal
}

// NewEventLoop creates an EventLoop.
func NewEventLoop(hypr *hypr.Client, state *State, subs *daemon.SubscriptionManager, done <-chan struct{}) *EventLoop {
	return &EventLoop{
		hypr:  hypr,
		state: state,
		subs:  subs,
		done:  done,
	}
}

// Run connects to Hyprland's event socket, performs initial state sync,
// then processes events until the done channel closes.
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

		line := scanner.Text()
		e.handleEvent(line)
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-e.done:
			return nil // Normal shutdown
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

	wsSet := make(map[int]bool)
	for _, c := range clients {
		if c.Workspace.ID > 0 { // Skip special workspaces
			wsSet[c.Workspace.ID] = true
		}
	}

	occupied := make([]int, 0, len(wsSet))
	for ws := range wsSet {
		occupied = append(occupied, ws)
	}
	sort.Ints(occupied)

	e.state.SetOccupied(occupied)
	return nil
}

func (e *EventLoop) handleEvent(line string) {
	parts := strings.SplitN(line, ">>", 2)
	if len(parts) != 2 {
		return
	}

	event, data := parts[0], parts[1]

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

	case "createworkspace", "destroyworkspace":
		e.updateOccupied()
		e.notifyWorkspace()

	case "openwindow":
		e.updateOccupied()
		e.notifyWorkspace()

	case "closewindow":
		addr := data
		if !strings.HasPrefix(addr, "0x") {
			addr = "0x" + addr
		}
		e.state.ClearWindowState(addr)
		e.updateOccupied()
		e.notifyWorkspace()

	case "movewindow":
		e.updateOccupied()
		e.notifyWorkspace()
	}
}

func (e *EventLoop) notifyWorkspace() {
	if e.subs == nil {
		return
	}
	data := map[string]any{
		"current":  e.state.GetWorkspace(),
		"occupied": e.state.GetOccupied(),
	}
	e.subs.Notify("workspace", data)
}
