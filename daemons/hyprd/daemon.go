// Package main implements hyprd, a daemon for managing Hyprland window layouts
// and state synchronization with eww widgets.
//
// hyprd connects to Hyprland's IPC sockets to monitor workspace changes,
// window events, and execute window management commands. It maintains
// state for features like monocle mode, split ratios, and hidden windows.
//
// # Architecture
//
// The daemon consists of three main components:
//
//   - Daemon: Manages the command socket and routes client requests
//   - EventLoop: Subscribes to Hyprland events and updates state
//   - State: Thread-safe storage for workspace and window tracking
//
// # Commands
//
// Clients connect via Unix socket and send text commands:
//
//   - monocle: Toggle focused window to float on dedicated workspace
//   - split: Cycle or set the master/slave split ratio
//   - hide: Toggle visibility of slave windows
//   - swap: Exchange master and slave window positions
//   - ws: Switch workspace with automatic master focus
//   - focus: Focus window by class, unhiding if necessary
//   - layout: Apply predefined window arrangements
//   - query: Retrieve current state as JSON
//   - subscribe: Stream state change events
//
// # Integration
//
// hyprd provides real-time state updates to eww widgets through a
// subscription mechanism. Widgets subscribe to topics (workspace, monocle,
// split) and receive JSON events when state changes.
package main

import (
	"dotfiles/daemons/hyprd/commands"
	"dotfiles/daemons/hyprd/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/internal/daemon"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const SocketPath = "/tmp/hyprd.sock"

// Daemon coordinates Hyprland IPC, state management, and command execution.
// It handles client requests over a Unix socket and publishes state changes to subscribers.
type Daemon struct {
	hypr   *hypr.Client       // Hyprland IPC connection
	state  *State             // Thread-safe workspace and window state
	server *daemon.Server     // Unix socket command server
	config *config.Config     // Monitor geometry and layout configuration
}

// New connects to Hyprland's IPC socket and initializes the daemon state.
func New() (*Daemon, error) {
	cfg := config.Load()

	hyprClient, err := hypr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connect to hyprland: %w", err)
	}

	state := NewState(cfg)
	d := &Daemon{
		hypr:   hyprClient,
		state:  state,
		config: cfg,
	}

	d.server = daemon.NewServer(SocketPath, d.handleCommand)
	d.server.OnSubscribe = d.sendInitialState

	return d, nil
}

// Run starts the command server and event loop, blocking until SIGINT/SIGTERM.
func (d *Daemon) Run() error {
	clients, err := d.hypr.Clients()
	if err != nil {
		return fmt.Errorf("hyprland query failed: %w", err)
	}
	fmt.Printf("hyprd: connected to hyprland (%d clients)\n", len(clients))

	if err := d.initGeometry(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: geometry query failed, using defaults: %v\n", err)
	}

	if err := d.server.Start(); err != nil {
		return err
	}
	fmt.Printf("hyprd: listening on %s\n", SocketPath)

	events := NewEventLoop(d.hypr, d.state, d.server.Subs, d.server.Done())
	go func() {
		if err := events.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd: event loop error: %v\n", err)
		}
	}()

	sig := d.server.WaitForSignal()
	fmt.Printf("\nhyprd: received %s, shutting down\n", sig)
	d.server.Shutdown()

	return nil
}

// sendInitialState bootstraps a subscriber with the current state for their topics.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	if sub.WantsTopic("workspace") {
		data := map[string]any{
			"current":  d.state.GetWorkspace(),
			"occupied": d.state.GetOccupied(),
		}
		sub.SendEvent("workspace", data)
	}

	if sub.WantsTopic("monocle") {
		monocle := d.state.GetMonocle()
		var data any
		if monocle != nil {
			data = map[string]any{
				"address":   monocle.Address,
				"origin_ws": monocle.OriginWS,
			}
		}
		sub.SendEvent("monocle", data)
	}

	if sub.WantsTopic("split") {
		sub.SendEvent("split", d.state.GetSplitRatio())
	}
}

// initGeometry queries monitor dimensions from Hyprland and computes monocle window size.
func (d *Daemon) initGeometry() error {
	monitor, err := d.hypr.FocusedMonitor()
	if err != nil {
		return err
	}

	width := d.config.Monitor.Width
	height := d.config.Monitor.Height
	if monitor != nil {
		width = monitor.Width
		height = monitor.Height
	}

	geo := commands.ComputeGeometry(
		width,
		height,
		d.config.Monitor.Reserved.Top,
		d.config.Monitor.Reserved.Bottom,
		d.config.Monitor.Reserved.Left,
		d.config.Monocle.WidthRatio,
		d.config.Monocle.HeightRatio,
	)
	d.state.SetGeometry(geo)
	if monitor != nil {
		fmt.Printf("hyprd: monitor %s (%dx%d), monocle %dx%d\n",
			monitor.Name, width, height, geo.MonocleW, geo.MonocleH)
	}
	return nil
}

// handleCommand parses and routes incoming client commands, returning results as strings.
func (d *Daemon) handleCommand(command string) string {
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "status":
		return "running"
	case "ping":
		return "pong"
	case "state":
		data, err := d.state.JSON()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return string(data)
	case "monocle":
		monocle := commands.NewMonocle(d.hypr, d.state)
		result, err := monocle.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "split":
		split := commands.NewSplit(d.hypr, d.state)
		result, err := split.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "hide":
		hide := commands.NewHide(d.hypr, d.state)
		result, err := hide.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "swap":
		swap := commands.NewSwap(d.hypr, d.state)
		result, err := swap.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "ws":
		if arg == "" {
			return "error: workspace number required"
		}
		ws := commands.NewWS(d.hypr, d.state)
		result, err := ws.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "focus":
		parts := strings.SplitN(arg, " ", 2)
		class := ""
		title := ""
		if len(parts) >= 1 {
			class = parts[0]
		}
		if len(parts) >= 2 {
			title = parts[1]
		}
		focus := commands.NewFocus(d.hypr, d.state)
		result, err := focus.Execute(class, title)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "query":
		topic := arg
		if topic == "" {
			topic = "all"
		}
		result, err := d.query(topic)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "layout":
		layout := commands.NewLayout(d.hypr, d.state)
		result, err := layout.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	default:
		return fmt.Sprintf("unknown command: %s", cmd)
	}
}

// query returns JSON-encoded state for a specific topic or all state if topic is "all".
func (d *Daemon) query(topic string) (string, error) {
	switch topic {
	case "workspace":
		data := map[string]any{
			"current":  d.state.GetWorkspace(),
			"occupied": d.state.GetOccupied(),
		}
		jsonData, err := json.Marshal(data)
		return string(jsonData), err

	case "monocle":
		monocle := d.state.GetMonocle()
		if monocle == nil {
			return "null", nil
		}
		jsonData, err := json.Marshal(monocle)
		return string(jsonData), err

	case "hidden":
		hidden := d.state.GetHidden()
		if len(hidden) == 0 {
			return "null", nil
		}
		jsonData, err := json.Marshal(hidden)
		return string(jsonData), err

	case "split":
		return fmt.Sprintf(`"%s"`, d.state.GetSplitRatio()), nil

	case "all", "":
		jsonData, err := d.state.JSON()
		return string(jsonData), err

	default:
		return "", fmt.Errorf("unknown topic: %s", topic)
	}
}
