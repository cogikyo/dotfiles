// Package main implements hyprd, a daemon for managing Hyprland window layouts
// and state synchronization with eww widgets.
//
// hyprd connects to Hyprland's IPC sockets to monitor workspace changes,
// window events, and execute window management commands. It maintains
// state for features like split ratios, hidden windows, and three-body layouts.
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
//   - split: Cycle or set the master/slave split ratio
//   - hide: Toggle visibility of slave windows
//   - swap: Exchange master and slave window positions
//   - ws: Switch workspace with automatic master focus
//   - focus: Focus window by class, unhiding if necessary
//   - layout: Apply predefined window arrangements
//   - three-body: Three-window layout with shadow workspace swapping
//   - query: Retrieve current state as JSON
//   - subscribe: Stream state change events
//
// # Integration
//
// hyprd provides real-time state updates to eww widgets through a
// subscription mechanism. Widgets subscribe to topics (workspace, split)
// and receive JSON events when state changes.
package main

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/daemon"
	"dotfiles/daemons/hyprd/commands"
	"dotfiles/daemons/hyprd/hypr"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const SocketPath = "/tmp/hyprd.sock"

// Daemon coordinates Hyprland IPC, state management, and command execution.
// It handles client requests over a Unix socket and publishes state changes to subscribers.
type Daemon struct {
	hypr   *hypr.Client       // Hyprland IPC connection
	state  *State             // Thread-safe workspace and window state
	server *daemon.Server     // Unix socket command server
	config *config.HyprConfig // Monitor geometry and layout configuration
}

// New connects to Hyprland's IPC socket and initializes the daemon state.
func New() (*Daemon, error) {
	cfg := config.Load()

	hyprClient, err := hypr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connect to hyprland: %w", err)
	}

	state := NewState(&cfg.Hypr)
	d := &Daemon{
		hypr:   hyprClient,
		state:  state,
		config: &cfg.Hypr,
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

	go d.watchConfig(d.server.Done())

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

	if sub.WantsTopic("split") {
		sub.SendEvent("split", d.state.GetSplitRatio())
	}
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
	case "bg":
		bg := commands.NewBG(&d.config.Background)
		result, err := bg.Execute(arg)
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
		// When three-body is active, swap shadow into master position
		tb := commands.NewThreeBody(d.hypr, d.state)
		tbResult, tbErr := tb.SwapMaster()
		if tbErr != nil {
			return fmt.Sprintf("error: %v", tbErr)
		}
		if tbResult != "" {
			return tbResult
		}
		// No three-body active — fall through to normal swap
		swap := commands.NewSwap(d.hypr, d.state)
		result, err := swap.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "ws":
		if arg == "" {
			return "error: workspace target required"
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
	case "tab":
		tab := commands.NewTab(d.hypr, d.state)
		result, err := tab.Execute(strings.TrimSpace(arg))
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
	case "monocle":
		monocle := commands.NewMonocle(d.hypr, d.state)
		result, err := monocle.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "three-body":
		return d.handleThreeBody(arg)
	case "layout":
		layout := commands.NewLayout(d.hypr, d.state)
		result, err := layout.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "project":
		return d.handleProject(arg)
	default:
		return fmt.Sprintf("unknown command: %s", cmd)
	}
}

// handleThreeBody routes named three-body subcommands.
func (d *Daemon) handleThreeBody(arg string) string {
	name := strings.TrimSpace(arg)
	if name == "" {
		return "usage: three-body {editor|agents|browser|shadow}"
	}
	tb := commands.NewThreeBody(d.hypr, d.state)
	tb.SetNotifyHooks(hasNotifications, func() { runCmd("dunstctl", "action") })
	result, err := tb.Execute(name)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

// handleProject gets or sets the project path for the current workspace.
// "project" or "project get" returns the current path.
// "project set <path>" sets it explicitly.
// "project clear" removes it.
func (d *Daemon) handleProject(arg string) string {
	wsData, err := d.hypr.Request("j/activeworkspace")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	var ws struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		return fmt.Sprintf("error: parse workspace: %v", err)
	}

	parts := strings.SplitN(strings.TrimSpace(arg), " ", 2)
	sub := parts[0]
	val := ""
	if len(parts) > 1 {
		val = parts[1]
	}

	switch sub {
	case "", "get":
		p := d.state.GetProjectPath(ws.ID)
		if p == "" {
			return fmt.Sprintf("ws%d: (none)", ws.ID)
		}
		return fmt.Sprintf("ws%d: %s", ws.ID, p)
	case "set":
		if val == "" {
			return "usage: project set <path>"
		}
		d.state.SetProjectPath(ws.ID, val)
		return fmt.Sprintf("ws%d: %s", ws.ID, val)
	case "clear":
		d.state.SetProjectPath(ws.ID, "")
		return fmt.Sprintf("ws%d: cleared", ws.ID)
	default:
		return "usage: project [get|set <path>|clear]"
	}
}

// hasNotifications returns true if dunst has displayed notifications.
func hasNotifications() bool {
	out, err := exec.Command("dunstctl", "count", "displayed").Output()
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	return err == nil && n > 0
}

// runCmd executes a command without waiting for output.
func runCmd(name string, args ...string) {
	exec.Command(name, args...).Run()
}

// watchConfig monitors hyprd.yaml for changes and hot-reloads the config pointer.
func (d *Daemon) watchConfig(done <-chan struct{}) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: config watcher: %v\n", err)
		return
	}
	configFile := filepath.Join(home, "dotfiles/daemons/daemons.yaml")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: config watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: config watcher: %v\n", err)
		return
	}

	var debounce *time.Timer
	for {
		select {
		case <-done:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == 0 {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(100*time.Millisecond, func() {
				cfg := config.Load()
				d.state.ReloadConfig(&cfg.Hypr)
				d.config = &cfg.Hypr
				fmt.Printf("hyprd: config reloaded\n")
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "hyprd: config watcher error: %v\n", err)
		}
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

	case "hidden":
		hidden := d.state.GetHidden()
		if len(hidden) == 0 {
			return "null", nil
		}
		jsonData, err := json.Marshal(hidden)
		return string(jsonData), err

	case "split":
		return fmt.Sprintf(`"%s"`, d.state.GetSplitRatio()), nil

	case "three-body":
		allTB := d.state.AllThreeBody()
		if len(allTB) == 0 {
			return "null", nil
		}
		jsonData, err := json.Marshal(allTB)
		return string(jsonData), err

	case "all", "":
		jsonData, err := d.state.JSON()
		return string(jsonData), err

	default:
		return "", fmt.Errorf("unknown topic: %s", topic)
	}
}
