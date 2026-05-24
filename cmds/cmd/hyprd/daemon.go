package main

// daemon.go wires daemon lifecycle, command routing, config hot-reload, and self-restart behavior.

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/daemon"
	"dotfiles/cmds/internal/hyprd/browser"
	"dotfiles/cmds/internal/hyprd/hypr"
	notifypkg "dotfiles/cmds/internal/hyprd/notify"
	"dotfiles/cmds/internal/hyprd/session"
	"dotfiles/cmds/internal/hyprd/state"
	"dotfiles/cmds/internal/hyprd/windows"
	"dotfiles/cmds/internal/hyprd/wm"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// SocketPath is the daemon command socket used by the CLI front-end.
const SocketPath = "/tmp/hyprd.sock"

// stateFile is the one-shot handoff used by `hyprd rebuild` before execing the new binary.
const stateFile = "/tmp/hyprd-state.json"

const computeCPUs = "0-6,8-14,16-1023"

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Daemon                                                                       │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Daemon owns the Hyprland IPC client, shared state, command server, and hot-reloadable config.
//
// Config is atomic so the watcher can swap it without locking command handlers.
type Daemon struct {
	hypr      *hypr.Client
	state     *state.State
	server    *daemon.Server
	config    atomic.Pointer[config.HyprConfig]
	lockCtl   *session.Lock
	pickerCtl *session.Picker
	restartCh chan struct{}
}

// New connects to Hyprland's IPC socket and prepares the daemon.
func New() (*Daemon, error) {
	cfg := config.LoadHypr()

	hyprClient, err := hypr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connect to hyprland: %w", err)
	}

	stateStore := state.NewState(&cfg)
	d := &Daemon{
		hypr:      hyprClient,
		state:     stateStore,
		lockCtl:   session.NewLock(hyprClient, stateStore),
		pickerCtl: session.NewPicker(hyprClient, stateStore),
		restartCh: make(chan struct{}, 1),
	}
	d.config.Store(&cfg)

	d.server = daemon.NewServer(SocketPath, d.handleCommand)
	d.server.OnSubscribe = d.sendInitialState

	return d, nil
}

// Run starts the server, event loop, and config watcher, then blocks until SIGINT/SIGTERM.
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		fmt.Printf("\nhyprd: received %s, shutting down\n", sig)
		d.server.Shutdown()
		return nil
	case <-d.restartCh:
		time.Sleep(50 * time.Millisecond)
		fmt.Println("hyprd: restarting...")
		d.server.Shutdown()
		return d.execSelf()
	}
}

// sendInitialState seeds a new subscriber with current values so eww widgets don't flicker.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	if sub.WantsTopic("workspace") {
		sub.SendEvent("workspace", d.workspacePayload())
	}

	if sub.WantsTopic("split") {
		sub.SendEvent("split", d.state.GetSplitRatio())
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Command dispatch                                                             │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// handleCommand routes one line from the daemon socket: `<verb> [raw args]`.
func (d *Daemon) handleCommand(command string) string {
	cmd, arg, _ := strings.Cut(command, " ")
	arg = strings.TrimSpace(arg)

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
		monocle := wm.NewMonocle(d.hypr, d.state)
		if _, err := monocle.DeactivateIfActive(); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		tb := wm.NewThreeBody(d.hypr, d.state)
		tbResult, tbErr := tb.SwapMaster()
		if tbErr != nil {
			return fmt.Sprintf("error: %v", tbErr)
		}
		if tbResult != "" {
			return tbResult
		}
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
		class, title, _ := strings.Cut(arg, " ")
		title = strings.TrimSpace(title)
		focus := wm.NewFocus(d.hypr, d.state)
		result, err := focus.Execute(class, title)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "tab":
		tabName, filePath := parseTabArg(arg)
		tab := session.NewTab(d.hypr, d.state)
		result, err := tab.Execute(tabName, filePath)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "tabs":
		tabs := session.NewTabs(d.hypr, d.state)
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
	case "shadow":
		return d.handleShadow(arg)
	case "init":
		init := d.newInit()
		result, err := init.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "lock":
		result, err := d.lockCtl.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "picker":
		result, err := d.pickerCtl.Execute(arg)
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
	case "browser":
		return d.handleBrowser(arg)
	case "project":
		return d.handleProject(arg)
	case "notify":
		return d.handleNotify(arg)
	case "rebuild":
		return d.handleRebuild()
	default:
		return fmt.Sprintf("unknown command: %s", cmd)
	}
}

func parseTabArg(arg string) (string, string) {
	name, filePath, ok := strings.Cut(strings.TrimSpace(arg), " -- ")
	if !ok {
		return strings.TrimSpace(arg), ""
	}
	return strings.TrimSpace(name), strings.TrimSpace(filePath)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Subcommand handlers                                                          │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (d *Daemon) handleShadow(arg string) string {
	shadowWS := windows.ShadowWorkspace
	special := strings.TrimPrefix(shadowWS, "special:")

	switch strings.TrimSpace(arg) {
	case "", "toggle":
		if err := d.hypr.Dispatch("togglespecialworkspace " + special); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "toggled " + shadowWS
	case "list":
		clients, err := d.hypr.Clients()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		var stranded []map[string]string
		for _, c := range clients {
			if c.Workspace.Name == shadowWS {
				stranded = append(stranded, map[string]string{
					"address": c.Address,
					"class":   c.Class,
					"title":   c.Title,
				})
			}
		}
		if len(stranded) == 0 {
			return "[]"
		}
		data, err := json.MarshalIndent(stranded, "", "  ")
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return string(data)
	default:
		return "usage: shadow [toggle|list]"
	}
}

func (d *Daemon) handleThreeBody(arg string) string {
	name := strings.TrimSpace(arg)
	if name == "" {
		return "usage: three-body {editor|agents|browser|shadow}"
	}
	monocle := wm.NewMonocle(d.hypr, d.state)
	if _, err := monocle.DeactivateIfActive(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	tb := wm.NewThreeBody(d.hypr, d.state)
	tb.SetNotifyHooks(hasDisplayedNotifications, d.tryDunstAction)
	result, err := tb.Execute(name)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

func (d *Daemon) handleBrowser(arg string) string {
	b := browser.NewBrowser(d.hypr, d.state)
	result, err := b.Execute(strings.TrimSpace(arg))
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

	notifier := notifypkg.NewNotifier(d.hypr, d.state, d.config.Load())
	req = notifier.Prepare(req)
	if !notifier.CanDispatch(req) {
		return "missing-context"
	}

	go func() {
		if err := notifier.Handle(req); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd notify: %v\n", err)
		}
	}()

	return "ok"
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Hot rebuild                                                                  │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// handleRebuild builds ./cmd/hyprd from the dotfiles Go workspace, installs ~/.local/bin/hyprd, and restarts in place.
//
// Runtime state is written to stateFile before the binary swap and consumed once by restoreState after exec.
func (d *Daemon) handleRebuild() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	srcDir := filepath.Join(home, "dotfiles", "cmds")
	if dotfiles := os.Getenv("DOTFILES"); dotfiles != "" {
		srcDir = filepath.Join(dotfiles, "cmds")
	}
	binPath := filepath.Join(home, ".local", "bin", "hyprd")
	tmpBin := binPath + ".new"
	if err := os.MkdirAll(filepath.Dir(binPath), 0755); err != nil {
		return fmt.Sprintf("error: install dir: %v", err)
	}

	cmd := exec.Command("taskset", "-c", computeCPUs, "go", "build", "-o", tmpBin, "./cmd/hyprd")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmpBin)
		return fmt.Sprintf("error: build failed: %v\n%s", err, out)
	}

	stateData, err := d.state.JSON()
	if err != nil {
		os.Remove(tmpBin)
		return fmt.Sprintf("error: state dump: %v", err)
	}
	if err := os.WriteFile(stateFile, stateData, 0600); err != nil {
		os.Remove(tmpBin)
		return fmt.Sprintf("error: state write: %v", err)
	}

	if err := os.Rename(tmpBin, binPath); err != nil {
		os.Remove(tmpBin)
		os.Remove(stateFile)
		return fmt.Sprintf("error: install: %v", err)
	}

	select {
	case d.restartCh <- struct{}{}:
	default:
	}
	return "rebuilt: restarting..."
}

func (d *Daemon) restoreState() {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}
	os.Remove(stateFile)
	if err := d.state.Restore(data); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: state restore: %v\n", err)
		return
	}
	fmt.Println("hyprd: state restored")
}

func (d *Daemon) execSelf() error {
	home, _ := os.UserHomeDir()
	bin := filepath.Join(home, ".local", "bin", "hyprd")
	return syscall.Exec(bin, []string{"hyprd"}, os.Environ())
}

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

	sub, val, _ := strings.Cut(strings.TrimSpace(arg), " ")
	val = strings.TrimSpace(val)

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
	init.SetLock(d.lockCtl)
	init.SetNotify(func(app, urgency, title, body string) {
		notifier := notifypkg.NewNotifier(d.hypr, d.state, d.config.Load())
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

// hasDisplayedNotifications keeps the agents keybind's old behavior: Dunst gets
// first chance whenever a visible notification exists.
func hasDisplayedNotifications() bool {
	return displayedNotifications() > 0
}

func (d *Daemon) tryDunstAction() bool {
	before := displayedNotifications()
	if before == 0 {
		return false
	}
	kind := latestDisplayedNotificationKind()
	activeBefore := d.activeWindowAddress()
	if kind.App != "" && kind.Category != "hyprd" {
		d.prefocusNotificationApp(kind.App)
	}

	if err := exec.Command("dunstctl", "action").Run(); err != nil {
		exec.Command("dunstctl", "close").Run()
		return false
	}
	if kind.Category == "hyprd" {
		return true
	}

	time.Sleep(75 * time.Millisecond)
	activeAfter := d.activeWindowAddress()
	return activeBefore != "" && activeAfter != "" && activeAfter != activeBefore
}

func (d *Daemon) prefocusNotificationApp(app string) {
	class := notificationAppClass(app)
	if class == "" {
		return
	}

	clients, err := d.hypr.Clients()
	if err != nil {
		return
	}
	var fallback string
	for _, client := range clients {
		if !strings.EqualFold(client.Class, class) {
			continue
		}
		if fallback == "" {
			fallback = client.Address
		}
		if client.Workspace.ID == 2 {
			d.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", client.Address))
			return
		}
	}
	if fallback != "" {
		d.hypr.Dispatch(fmt.Sprintf("focuswindow address:%s", fallback))
	}
}

type displayedNotificationKind struct {
	App      string
	Category string
}

func notificationAppClass(app string) string {
	switch strings.ToLower(strings.TrimSpace(app)) {
	case "slack":
		return "Slack"
	case "discord":
		return "discord"
	case "firefox developer edition":
		return "firefox-developer-edition"
	default:
		return ""
	}
}

func latestDisplayedNotificationKind() displayedNotificationKind {
	displayed := displayedNotifications()
	if displayed == 0 {
		return displayedNotificationKind{}
	}
	history, err := exec.Command("dunstctl", "history").Output()
	if err != nil {
		return displayedNotificationKind{}
	}
	var payload struct {
		Data [][]map[string]struct {
			Data any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(history, &payload); err != nil || len(payload.Data) == 0 {
		return displayedNotificationKind{}
	}
	for i, item := range payload.Data[0] {
		if i >= displayed {
			break
		}
		category, _ := item["category"].Data.(string)
		if category == "hyprd" {
			app, _ := item["appname"].Data.(string)
			return displayedNotificationKind{App: app, Category: category}
		}
		app, _ := item["appname"].Data.(string)
		if notificationAppClass(app) != "" {
			return displayedNotificationKind{App: app, Category: category}
		}
	}
	return displayedNotificationKind{}
}

func displayedNotifications() int {
	out, err := exec.Command("dunstctl", "count", "displayed").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return n
}

func (d *Daemon) activeWindowAddress() string {
	data, err := d.hypr.Request("j/activewindow")
	if err != nil {
		return ""
	}
	var win struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(data, &win); err != nil {
		return ""
	}
	return win.Address
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Config watcher                                                               │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// watchConfig hot-reloads ~/dotfiles/cmds/config/hyprd.yaml on change.
//
// Watches the parent directory because nvim rename-and-replaces on save, killing file-level watches.
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

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ Query / subscribe                                                            │
// ╰──────────────────────────────────────────────────────────────────────────────╯

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

	return map[string]any{
		"current":      current,
		"current_str":  strconv.Itoa(current),
		"occupied":     occupied,
		"occupied_str": joinWorkspaceIDs(occupied),
	}
}

func joinWorkspaceIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, " ")
}

func (d *Daemon) notifyWorkspace() {
	if d.server == nil || d.server.Subs == nil {
		return
	}
	d.server.Subs.Notify("workspace", d.workspacePayload())
}
