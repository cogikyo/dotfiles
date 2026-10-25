package main

// daemon.go defines ewwd runtime orchestration, provider wiring, and socket command handlers.

import (
	"cmp"
	"context"
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/daemon"
	"dotfiles/cmds/internal/ewwd/providers"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const SocketPath = "/tmp/ewwd.sock"

const hyprdSocketPath = "/tmp/hyprd.sock"

// importSystemdEnv backfills env vars (WAYLAND_DISPLAY et al.) from the systemd user environment.
func importSystemdEnv() {
	out, err := exec.Command("systemctl", "--user", "show-environment").Output()
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := line[:eq]
		if os.Getenv(key) == "" {
			os.Setenv(key, line[eq+1:])
		}
	}
}

// Daemon orchestrates providers and routes client commands to state updates.
type Daemon struct {
	state       *State
	server      *daemon.Server
	providers   []providers.Provider
	ctx         context.Context
	cancel      context.CancelFunc
	config      *config.Config
	autoOpen    bool
	openMu      sync.Mutex
	reconcileMu sync.Mutex
	desiredOpen bool
	pending     bool
	commanded   bool
	ewwDone     chan error
}

func New(autoOpen bool) (*Daemon, error) {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	state := NewState()
	d := &Daemon{
		state:    state,
		ctx:      ctx,
		cancel:   cancel,
		config:   cfg,
		autoOpen: autoOpen,
	}

	d.server = daemon.NewServer(SocketPath, d.handleCommand)
	d.server.OnSubscribe = d.sendInitialState

	return d, nil
}

// Run starts the server, launches providers, optionally opens eww windows, and blocks until signalled.
func (d *Daemon) Run() error {
	importSystemdEnv()

	if err := d.server.Start(); err != nil {
		return err
	}
	fmt.Printf("ewwd: listening on %s\n", SocketPath)

	d.initProviders()
	for _, p := range d.providers {
		go func(p providers.Provider) {
			notify := func(data any) {
				d.server.Subs.Notify(p.Name(), data)
			}
			if err := p.Start(d.ctx, notify); err != nil {
				fmt.Fprintf(os.Stderr, "ewwd: provider %s error: %v\n", p.Name(), err)
			}
		}(p)
	}

	go d.healthLoop()

	if d.autoOpen {
		// eww startup can take seconds; don't block signal handling.
		go func() {
			if result := d.openWindows(openPass); result != "" {
				fmt.Printf("ewwd: %s\n", result)
			}
		}()
	} else {
		go d.resume()
	}

	sig := d.server.WaitForSignal()
	fmt.Printf("\newwd: received %s, shutting down\n", sig)
	d.cancel()
	d.server.Shutdown()

	for _, p := range d.providers {
		p.Stop()
	}

	return nil
}

func (d *Daemon) initProviders() {
	cfg := d.config.Eww
	audio := providers.NewAudio(d.state, cfg.Audio)
	d.providers = []providers.Provider{
		providers.NewNetwork(d.state, cfg.Network),
		providers.NewDate(d.state, cfg.Date),
		providers.NewClock(d.state),
		providers.NewComputer(d.state),
		providers.NewBattery(d.state),
		providers.NewCycle(d.state, "cycle-5", 5*time.Second),
		providers.NewCycle(d.state, "cycle-6", 6*time.Second),
		audio,
		providers.NewBluetooth(d.state, d.config.Hypr.Bluetooth.Device, audio.Wear),
		providers.NewMusic(d.state, d.server.Subs.Notify, cfg.Music.SpDc),
		providers.NewTimer(d.state, cfg.Timer),
		providers.NewWeather(d.state, cfg.Weather),
	}
}

// sendInitialState replays current state so new subscribers render without waiting for the next tick.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	for topic, data := range d.state.GetAll() {
		if data != nil && sub.WantsTopic(topic) {
			sub.SendEvent(topic, data)
		}
	}
}

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
	case "open":
		return d.openWindows(openPass)
	case "restore":
		return d.openWindows(restorePass)
	case "close":
		return d.closeWindows()
	case "relayout":
		return d.relayoutWindows()
	case "action":
		actionParts := strings.Fields(arg)
		if len(actionParts) == 0 {
			return "error: provider name required"
		}
		provider := actionParts[0]
		args := actionParts[1:]
		return d.handleAction(provider, args)
	default:
		return fmt.Sprintf("unknown command: %s", cmd)
	}
}

// query returns JSON for a topic, the whole store for "all", or "null" for unknown topics.
func (d *Daemon) query(topic string) (string, error) {
	if topic == "all" || topic == "" {
		jsonData, err := d.state.JSON()
		return string(jsonData), err
	}

	data := d.state.Get(topic)
	if data == nil {
		return "null", nil
	}
	jsonData, err := json.Marshal(data)
	return string(jsonData), err
}

func (d *Daemon) handleAction(providerName string, args []string) string {
	for _, p := range d.providers {
		if p.Name() == providerName {
			if ap, ok := p.(providers.ActionProvider); ok {
				result, err := ap.HandleAction(args)
				if err != nil {
					return fmt.Sprintf("error: %v", err)
				}
				return result
			}
			return fmt.Sprintf("error: %s does not support actions", providerName)
		}
	}
	return fmt.Sprintf("error: unknown provider: %s", providerName)
}

type pass string

const (
	restorePass  pass = "restore"
	openPass     pass = "open"
	relayoutPass pass = "relayout"
)

// openWindows ensures the eww daemon is running and reconciles configured windows.
func (d *Daemon) openWindows(p pass) string {
	d.openMu.Lock()
	d.desiredOpen = true
	d.pending = true
	d.commanded = true
	d.openMu.Unlock()

	go d.reconcileLatest(p)
	return "open: scheduled"
}

func (d *Daemon) closeWindows() string {
	d.openMu.Lock()
	d.desiredOpen = false
	d.pending = true
	d.commanded = true
	d.openMu.Unlock()

	go d.reconcileLatest(restorePass)
	return "close: scheduled"
}

func (d *Daemon) resume() {
	display, err := queryDisplay()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: hyprd display unavailable, widgets stay closed: %v\n", err)
		return
	}
	d.openMu.Lock()
	defer d.openMu.Unlock()
	if d.commanded {
		return
	}
	d.desiredOpen = !display.Hidden
	d.pending = true
	go d.reconcileLatest(openPass)
}

func (d *Daemon) relayoutWindows() string {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	if !d.desiredOpen {
		return "relayout: widgets closed"
	}
	d.pending = true
	go d.reconcileLatest(relayoutPass)
	return "relayout: scheduled"
}

func (d *Daemon) desiredWidgetsOpen() bool {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	return d.desiredOpen
}

func (d *Daemon) reconcilePending() bool {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	return d.pending
}

func (d *Daemon) markReconciled(open bool) {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	if d.desiredOpen == open {
		d.pending = false
	}
}

func (d *Daemon) reconcileLatest(p pass) {
	d.reconcileMu.Lock()
	defer d.reconcileMu.Unlock()

	healed, err := d.healDuplicateEww()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: duplicate eww recovery failed: %v\n", err)
		return
	}
	if healed {
		if err := d.ensureEwwDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: duplicate eww restart failed: %v\n", err)
			return
		}
	}

	if !d.desiredWidgetsOpen() {
		if d.closeEwwWindows() {
			d.markReconciled(false)
		}
		return
	}

	display, err := queryDisplay()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: hyprd display unavailable, using full: %v\n", err)
		display.Layout = config.LayoutFull
	}
	if display.Hidden {
		if d.closeEwwWindows() {
			d.markReconciled(true)
		}
		fmt.Println("ewwd: hyprd reports widgets hidden; keeping eww closed")
		return
	}

	result, ok := d.reconcileWindows(p, display)
	if !ok {
		fmt.Fprintf(os.Stderr, "ewwd: reconcile failed: %s\n", result)
	}
	if !d.desiredWidgetsOpen() {
		if d.closeEwwWindows() {
			d.markReconciled(false)
		}
		return
	}
	if ok {
		d.markReconciled(true)
	}
	if result != "" {
		fmt.Printf("ewwd: %s\n", result)
	}
}

func (d *Daemon) reconcileWindows(p pass, display display) (string, bool) {
	windows, others := d.config.Eww.Windows.For(display.Layout)
	if len(windows) == 0 {
		return fmt.Sprintf("%s: no %s windows configured", p, display.Layout), false
	}

	if p == relayoutPass {
		if err := d.stopEww(); err != nil {
			return fmt.Sprintf("error: %v", err), false
		}
	}
	ewwRunning := exec.Command("eww", "ping").Run() == nil
	if err := d.ensureEwwDaemon(); err != nil {
		return fmt.Sprintf("error: %v", err), false
	}
	if p == openPass && ewwRunning {
		if out, err := exec.Command("eww", "reload").CombinedOutput(); err != nil {
			return fmt.Sprintf("error: eww reload: %v%s", err, commandOutput(out)), false
		}
	}

	if err := closeActive(others); err != nil {
		return fmt.Sprintf("error: %v", err), false
	}
	if err := openSet(windows, display.Monitor); err != nil {
		return fmt.Sprintf("error: %v", err), false
	}

	return fmt.Sprintf("%s: %s", p, strings.Join(windows, " ")), true
}

type display struct {
	Layout  config.Layout `json:"layout"`
	Monitor string        `json:"monitor"`
	Hidden  bool          `json:"hidden"`
}

func queryDisplay() (display, error) {
	var d display
	resp, err := daemon.NewClient(hyprdSocketPath).Send("query display")
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal([]byte(resp), &d); err != nil {
		return d, fmt.Errorf("parse %q: %w", resp, err)
	}
	return d, nil
}

func openSet(windows []string, monitor string) error {
	args := append([]string{"--no-daemonize", "open-many"}, windows...)
	if monitor != "" {
		args = append(args, "--arg", "screen="+monitor)
	}
	if out, err := exec.Command("eww", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("eww open-many: %w%s", err, commandOutput(out))
	}
	return nil
}

func closeActive(windows []string) error {
	out, err := exec.Command("eww", "active-windows").Output()
	if err != nil {
		return fmt.Errorf("eww active-windows: %w", err)
	}
	var open []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if id, _, ok := strings.Cut(line, ":"); ok && slices.Contains(windows, id) {
			open = append(open, id)
		}
	}
	if len(open) == 0 {
		return nil
	}
	if out, err := exec.Command("eww", append([]string{"close"}, open...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("eww close: %w%s", err, commandOutput(out))
	}
	return nil
}

func (d *Daemon) ensureEwwDaemon() error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		importSystemdEnv()
		if os.Getenv("WAYLAND_DISPLAY") == "" {
			return fmt.Errorf("WAYLAND_DISPLAY not set")
		}
	}
	if exec.Command("eww", "ping").Run() == nil {
		if d.ewwDone != nil {
			return nil
		}
		fmt.Fprintln(os.Stderr, "ewwd: replacing unmanaged eww daemon")
		if err := d.stopEww(); err != nil {
			return err
		}
	}

	if d.ewwDone != nil {
		select {
		case err := <-d.ewwDone:
			d.ewwDone = nil
			if err != nil {
				fmt.Fprintf(os.Stderr, "ewwd: managed eww daemon exited: %v\n", err)
			}
		default:
			fmt.Fprintln(os.Stderr, "ewwd: managed eww daemon is not responding; restarting eww")
			if err := d.stopEww(); err != nil {
				return err
			}
		}
	}

	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		return fmt.Errorf("eww icons: XDG_RUNTIME_DIR is unset")
	}
	eww := config.ExpandPath("~/dotfiles/config/eww")
	parts := filepath.Join(runtime, "ewwd")
	if err := splitIcons(filepath.Join(eww, "icons"), filepath.Join(parts, "icons")); err != nil {
		return fmt.Errorf("eww icons: %w", err)
	}
	dataDirs := cmp.Or(os.Getenv("XDG_DATA_DIRS"), "/usr/local/share:/usr/share")
	cmd := exec.CommandContext(d.ctx, "eww", "--no-daemonize", "daemon")
	cmd.Env = append(os.Environ(), "XDG_DATA_DIRS="+eww+":"+parts+":"+dataDirs)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("eww daemon: %w", err)
	}
	done := make(chan error, 1)
	d.ewwDone = done
	go func() { done <- cmd.Wait() }()

	for range 40 {
		select {
		case err := <-d.ewwDone:
			d.ewwDone = nil
			if err != nil {
				return fmt.Errorf("eww daemon exited before ready: %w", err)
			}
			return fmt.Errorf("eww daemon exited before ready")
		default:
		}
		if exec.Command("eww", "ping").Run() == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("eww daemon did not become ready")
}

func (d *Daemon) healDuplicateEww() (bool, error) {
	duplicates, err := duplicateEwwListeners()
	if err != nil || len(duplicates) == 0 {
		return false, err
	}

	fmt.Fprintf(os.Stderr, "ewwd: duplicate eww listeners detected (%s); restarting eww\n", strings.Join(duplicates, ", "))
	if err := d.stopEww(); err != nil {
		return false, err
	}
	return true, nil
}

func (d *Daemon) stopEww() error {
	if err := killEwwTrees(); err != nil {
		return err
	}
	_ = exec.Command("hyprd", "picker", "close").Run()

	if d.ewwDone != nil {
		select {
		case <-d.ewwDone:
			d.ewwDone = nil
		case <-time.After(time.Second):
			return fmt.Errorf("managed eww daemon did not exit")
		}
	}

	for range 40 {
		counts, err := readEwwListenerCounts()
		if err != nil {
			return err
		}
		if len(counts) == 0 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("eww listeners remained after kill")
}

type process struct {
	parent int
	name   string
}

func killEwwTrees() error {
	before, err := processSnapshot()
	if err != nil {
		return err
	}

	var roots []int
	for pid, proc := range before {
		if proc.name == "eww" {
			roots = append(roots, pid)
		}
	}
	for _, pid := range roots {
		_ = syscall.Kill(pid, syscall.SIGSTOP)
	}

	members := make(map[int]bool, len(roots))
	for _, pid := range roots {
		members[pid] = true
	}
	for {
		beforeCount := len(members)
		snapshot, err := processSnapshot()
		if err != nil {
			for pid := range members {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
			return err
		}
		for changed := true; changed; {
			changed = false
			for pid, proc := range snapshot {
				if members[proc.parent] && !members[pid] {
					members[pid] = true
					changed = true
				}
			}
		}
		for pid := range members {
			_ = syscall.Kill(pid, syscall.SIGSTOP)
		}
		if len(members) == beforeCount {
			break
		}
	}

	rootSet := make(map[int]bool, len(roots))
	for _, pid := range roots {
		rootSet[pid] = true
	}
	for pid := range members {
		if !rootSet[pid] {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	for _, pid := range roots {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	return nil
}

func processSnapshot() (map[int]process, error) {
	out, err := exec.Command("ps", "-e", "-o", "pid=", "-o", "ppid=", "-o", "comm=").Output()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	processes := make(map[int]process)
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pid, pidErr := strconv.Atoi(fields[0])
		parent, parentErr := strconv.Atoi(fields[1])
		if pidErr == nil && parentErr == nil {
			processes[pid] = process{parent: parent, name: fields[2]}
		}
	}
	return processes, nil
}

func duplicateEwwListeners() ([]string, error) {
	counts, err := readEwwListenerCounts()
	if err != nil {
		return nil, err
	}

	var duplicates []string
	for path, count := range counts {
		if count > 1 {
			duplicates = append(duplicates, path)
		}
	}
	return duplicates, nil
}

func readEwwListenerCounts() (map[string]int, error) {
	data, err := os.ReadFile("/proc/net/unix")
	if err != nil {
		return nil, fmt.Errorf("read unix sockets: %w", err)
	}
	return parseEwwListenerCounts(data, os.Getenv("XDG_RUNTIME_DIR")), nil
}

func parseEwwListenerCounts(data []byte, runtimeDir string) map[string]int {
	prefix := filepath.Join(runtimeDir, "eww-server_")
	counts := make(map[string]int)
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 || fields[3] != "00010000" || fields[5] != "01" {
			continue
		}
		path := fields[7]
		if strings.HasPrefix(path, prefix) {
			counts[path]++
		}
	}
	return counts
}

func (d *Daemon) closeEwwWindows() bool {
	if err := d.stopEww(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: stop eww: %v\n", err)
		return false
	}
	if err := d.ensureEwwDaemon(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: start eww: %v\n", err)
		return false
	}
	return true
}

func (d *Daemon) healthLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}

		if duplicates, err := duplicateEwwListeners(); err == nil && len(duplicates) > 0 {
			go d.reconcileLatest(restorePass)
			continue
		}
		if d.reconcilePending() {
			go d.reconcileLatest(restorePass)
			continue
		}
		if exec.Command("eww", "ping").Run() == nil {
			continue
		}

		fmt.Fprintln(os.Stderr, "ewwd: eww ping failed; restarting eww")
		go d.reconcileLatest(restorePass)
	}
}

func commandOutput(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}
