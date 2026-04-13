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
	"dotfiles/daemons/hyprd/hypr"
	notifypkg "dotfiles/daemons/hyprd/notify"
	"dotfiles/daemons/hyprd/session"
	"dotfiles/daemons/hyprd/state"
	"dotfiles/daemons/hyprd/wm"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

const SocketPath = "/tmp/hyprd.sock"

// Daemon coordinates Hyprland IPC, state management, and command execution.
// It handles client requests over a Unix socket and publishes state changes to subscribers.
type Daemon struct {
	hypr   *hypr.Client                      // Hyprland IPC connection
	state  *state.State                      // Thread-safe workspace and window state
	server *daemon.Server                    // Unix socket command server
	config atomic.Pointer[config.HyprConfig] // Monitor geometry and layout configuration
}

// New connects to Hyprland's IPC socket and initializes the daemon state.
func New() (*Daemon, error) {
	cfg := config.LoadHypr()

	hyprClient, err := hypr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connect to hyprland: %w", err)
	}

	stateStore := state.NewState(&cfg)
	d := &Daemon{
		hypr:  hyprClient,
		state: stateStore,
	}
	d.config.Store(&cfg)

	d.server = daemon.NewServer(SocketPath, d.handleCommand)
	d.server.OnSubscribe = d.sendInitialState

	return d, nil
}

// Run starts the command server and event loop, blocking until SIGINT/SIGTERM.
// On a fresh Hyprland session (no existing windows), the init sequence runs
// automatically in the background.
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

	// Fresh boot: no windows exist yet, run init sequence
	if len(clients) == 0 {
		go func() {
			fmt.Println("hyprd: fresh session detected, running init")
			init := d.newInit()
			if _, err := init.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd: init error: %v\n", err)
			}
		}()
	}

	sig := d.server.WaitForSignal()
	fmt.Printf("\nhyprd: received %s, shutting down\n", sig)
	d.server.Shutdown()

	return nil
}

// sendInitialState bootstraps a subscriber with the current state for their topics.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	if sub.WantsTopic("workspace") {
		sub.SendEvent("workspace", d.workspacePayload())
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
		bg := session.NewBG(&d.config.Load().Background)
		result, err := bg.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "split":
		split := wm.NewSplit(d.hypr, d.state)
		result, err := split.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "hide":
		hide := wm.NewHide(d.hypr, d.state)
		result, err := hide.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "swap":
		// When three-body is active, swap shadow into master position
		tb := wm.NewThreeBody(d.hypr, d.state)
		tbResult, tbErr := tb.SwapMaster()
		if tbErr != nil {
			return fmt.Sprintf("error: %v", tbErr)
		}
		if tbResult != "" {
			return tbResult
		}
		// No three-body active — fall through to normal swap
		swap := wm.NewSwap(d.hypr, d.state)
		result, err := swap.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "ws":
		if arg == "" {
			return "error: workspace target required"
		}
		ws := wm.NewWS(d.hypr, d.state)
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
		focus := wm.NewFocus(d.hypr, d.state)
		result, err := focus.Execute(class, title)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "tab":
		tab := session.NewTab(d.hypr, d.state)
		result, err := tab.Execute(strings.TrimSpace(arg))
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "tabs":
		tabs := session.NewTabs(d.state)
		result, err := tabs.Execute(strings.TrimSpace(arg))
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
		monocle := wm.NewMonocle(d.hypr, d.state)
		result, err := monocle.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		d.notifyWorkspace()
		return result
	case "three-body":
		return d.handleThreeBody(arg)
	case "init":
		init := d.newInit()
		result, err := init.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "layout":
		layout := session.NewLayout(d.hypr, d.state)
		result, err := layout.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "project":
		return d.handleProject(arg)
	case "notify":
		return d.handleNotify(arg)
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
	tb := wm.NewThreeBody(d.hypr, d.state)
	tb.SetNotifyHooks(hasNotifications, func() { runCmd("dunstctl", "action") })
	result, err := tb.Execute(name)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

func (d *Daemon) handleNotify(arg string) string {
	var req notifypkg.NotifyRequest
	if err := json.Unmarshal([]byte(arg), &req); err != nil {
		return fmt.Sprintf("error: parse notify request: %v", err)
	}

	notifier := notifypkg.NewNotifier(d.hypr, d.config.Load())
	go func() {
		if err := notifier.Handle(req); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd notify: %v\n", err)
		}
	}()

	return "ok"
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

func (d *Daemon) newInit() *session.Init {
	init := session.NewInit(d.hypr, d.state)
	init.SetNotify(func(app, urgency, title, body string) {
		notifier := notifypkg.NewNotifier(d.hypr, d.config.Load())
		notifier.Handle(notifypkg.NotifyRequest{
			Source:  "send",
			App:     app,
			Urgency: urgency,
			Summary: title,
			Body:    body,
		})
	})
	return init
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
	if err := exec.Command(name, args...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: runCmd %s: %v\n", name, err)
	}
}

// watchConfig monitors configs/hyprd.yaml for changes and hot-reloads the config pointer.
// Watches the parent directory (not the file) because editors like nvim do
// rename-and-replace on save, which creates a new inode and kills file-level watches.
func (d *Daemon) watchConfig(done <-chan struct{}) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: config watcher: %v\n", err)
		return
	}
	configFile := filepath.Join(home, config.ConfigPath("hyprd"))
	configDir := filepath.Dir(configFile)
	configBase := filepath.Base(configFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: config watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(configDir); err != nil {
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
			if filepath.Base(event.Name) != configBase {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(100*time.Millisecond, func() {
				cfg := config.LoadHypr()
				d.state.ReloadConfig(&cfg)
				d.config.Store(&cfg)
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
		data := d.workspacePayload()
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

func (d *Daemon) workspacePayload() map[string]any {
	current := d.state.GetWorkspace()
	occupied := d.state.GetOccupied()
	monocle := d.state.ActiveMonocleWorkspace()

	return map[string]any{
		"current":      current,
		"current_str":  strconv.Itoa(current),
		"occupied":     occupied,
		"occupied_str": joinWorkspaceIDs(occupied),
		"monocle":      monocle,
		"monocle_str":  workspaceIDString(monocle),
	}
}

func joinWorkspaceIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, " ")
}

func workspaceIDString(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func (d *Daemon) notifyWorkspace() {
	if d.server == nil || d.server.Subs == nil {
		return
	}
	d.server.Subs.Notify("workspace", d.workspacePayload())
}
