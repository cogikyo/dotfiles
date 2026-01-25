package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"hyprd/hypr"
)

// EventLoop connects to Hyprland's event socket and updates state.
type EventLoop struct {
	hypr  *hypr.Client
	state *State
	subs  *SubscriptionManager
	done  chan struct{}
}

// NewEventLoop creates an event loop.
func NewEventLoop(hypr *hypr.Client, state *State, subs *SubscriptionManager, done chan struct{}) *EventLoop {
	return &EventLoop{
		hypr:  hypr,
		state: state,
		subs:  subs,
		done:  done,
	}
}

// Run starts listening for Hyprland events. Blocks until done channel closes.
func (e *EventLoop) Run() error {
	socketPath := e.hypr.EventSocketPath()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("connect to event socket: %w", err)
	}
	defer conn.Close()

	fmt.Printf("hyprd: subscribed to hyprland events\n")

	// Initialize state from current Hyprland state
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

// syncState queries Hyprland for current state.
func (e *EventLoop) syncState() error {
	// Get current workspace
	data, err := e.hypr.Request("j/activeworkspace")
	if err != nil {
		return err
	}

	// Parse workspace ID from JSON
	// Format: {"id":3,"name":"3",...}
	if idx := strings.Index(string(data), `"id":`); idx >= 0 {
		rest := string(data)[idx+5:]
		if end := strings.IndexAny(rest, ",}"); end > 0 {
			if ws, err := strconv.Atoi(rest[:end]); err == nil {
				e.state.SetWorkspace(ws)
			}
		}
	}

	// Get occupied workspaces
	if err := e.updateOccupied(); err != nil {
		return err
	}

	return nil
}

// updateOccupied queries Hyprland for occupied workspaces.
func (e *EventLoop) updateOccupied() error {
	clients, err := e.hypr.Clients()
	if err != nil {
		return err
	}

	// Collect unique workspace IDs
	wsSet := make(map[int]bool)
	for _, c := range clients {
		if c.Workspace.ID > 0 { // Skip special workspaces
			wsSet[c.Workspace.ID] = true
		}
	}

	// Convert to sorted slice
	occupied := make([]int, 0, len(wsSet))
	for ws := range wsSet {
		occupied = append(occupied, ws)
	}
	sort.Ints(occupied)

	e.state.SetOccupied(occupied)
	return nil
}

// handleEvent processes a single Hyprland event.
func (e *EventLoop) handleEvent(line string) {
	// Events are formatted as: eventname>>data
	parts := strings.SplitN(line, ">>", 2)
	if len(parts) != 2 {
		return
	}

	event, data := parts[0], parts[1]

	switch event {
	case "workspace", "workspacev2":
		// workspace>>ID or workspacev2>>ID,name
		wsStr := data
		if idx := strings.Index(data, ","); idx > 0 {
			wsStr = data[:idx]
		}
		if ws, err := strconv.Atoi(wsStr); err == nil {
			e.state.SetWorkspace(ws)
			e.notifyWorkspace()
		}

	case "focusedmon":
		// data: monitorname,workspaceID
		if idx := strings.LastIndex(data, ","); idx >= 0 {
			if ws, err := strconv.Atoi(data[idx+1:]); err == nil {
				e.state.SetWorkspace(ws)
				e.notifyWorkspace()
			}
		}

	case "createworkspace", "destroyworkspace":
		// Workspace list changed, update occupied
		e.updateOccupied()
		e.notifyWorkspace()

	case "openwindow":
		// data: address,workspace,class,title
		e.updateOccupied()
		e.notifyWorkspace()

	case "closewindow":
		// data: window address
		addr := data
		if !strings.HasPrefix(addr, "0x") {
			addr = "0x" + addr
		}
		e.state.ClearWindowState(addr)
		e.updateOccupied()
		e.notifyWorkspace()

	case "movewindow":
		// data: address,workspace
		e.updateOccupied()
		e.notifyWorkspace()
	}
}

// notifyWorkspace sends workspace state to subscribers.
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
