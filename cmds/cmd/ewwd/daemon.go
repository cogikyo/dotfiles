package main

// daemon.go defines ewwd runtime orchestration, provider wiring, and socket command handlers.

import (
	"context"
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/daemon"
	"dotfiles/cmds/internal/ewwd/providers"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const SocketPath = "/tmp/ewwd.sock"

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
	openPending bool
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
			if result := d.openWindows(true); result != "" {
				fmt.Printf("ewwd: %s\n", result)
			}
		}()
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
	d.providers = []providers.Provider{
		providers.NewNetwork(d.state, cfg.Network),
		providers.NewDate(d.state, cfg.Date),
		providers.NewAudio(d.state, cfg.Audio),
		providers.NewBluetooth(d.state, d.config.Hypr.Bluetooth.Device),
		providers.NewMusic(d.state, cfg.Music.SpDc),
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
		return d.openWindows(true)
	case "restore":
		return d.openWindows(false)
	case "close":
		return d.closeWindows()
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

// openWindows ensures the eww daemon is running and reconciles configured windows.
func (d *Daemon) openWindows(reload bool) string {
	d.openMu.Lock()
	d.desiredOpen = true
	d.openPending = true
	d.openMu.Unlock()

	go d.reconcileLatest(reload)
	return "open: scheduled"
}

func (d *Daemon) closeWindows() string {
	d.openMu.Lock()
	d.desiredOpen = false
	d.openPending = false
	d.openMu.Unlock()

	go d.reconcileLatest(false)
	return "close: scheduled"
}

func (d *Daemon) desiredWidgetsOpen() bool {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	return d.desiredOpen
}

func (d *Daemon) widgetsOpenPending() bool {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	return d.desiredOpen && d.openPending
}

func (d *Daemon) markWidgetsOpen() {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	if d.desiredOpen {
		d.openPending = false
	}
}

func (d *Daemon) reconcileLatest(reload bool) {
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
		d.closeEwwWindows()
		return
	}

	result, ok := d.reconcileWindows(reload)
	if !ok {
		fmt.Fprintf(os.Stderr, "ewwd: reconcile failed: %s\n", result)
	}
	if !d.desiredWidgetsOpen() {
		d.closeEwwWindows()
		return
	}
	if ok {
		d.markWidgetsOpen()
	}
	if result != "" {
		fmt.Printf("ewwd: %s\n", result)
	}
}

func (d *Daemon) reconcileWindows(reload bool) (string, bool) {
	windows := d.config.Eww.Windows
	verb := "restore"
	if reload {
		verb = "open"
	}
	if len(windows) == 0 {
		return verb + ": no windows configured", false
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		importSystemdEnv()
		if os.Getenv("WAYLAND_DISPLAY") == "" {
			return "error: WAYLAND_DISPLAY not set", false
		}
	}

	ewwRunning := exec.Command("eww", "ping").Run() == nil
	if err := d.ensureEwwDaemon(); err != nil {
		return fmt.Sprintf("error: %v", err), false
	}
	if reload && ewwRunning {
		if out, err := exec.Command("eww", "reload").CombinedOutput(); err != nil {
			return fmt.Sprintf("error: eww reload: %v%s", err, commandOutput(out)), false
		}
	}

	args := append([]string{"--no-daemonize", "open-many"}, windows...)
	if out, err := exec.Command("eww", args...).CombinedOutput(); err != nil {
		return fmt.Sprintf("error: eww open-many: %v%s", err, commandOutput(out)), false
	}

	return fmt.Sprintf("%s: %s", verb, strings.Join(windows, " ")), true
}

func (d *Daemon) ensureEwwDaemon() error {
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

	cmd := exec.CommandContext(d.ctx, "eww", "--no-daemonize", "daemon")
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

func (d *Daemon) closeEwwWindows() {
	if exec.Command("eww", "ping").Run() != nil {
		fmt.Fprintln(os.Stderr, "ewwd: eww is not responding while closing; stopping any stale processes")
		if err := d.stopEww(); err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: stop unresponsive eww: %v\n", err)
		}
		return
	}
	if out, err := exec.Command("eww", "close-all").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: eww close-all: %v%s\n", err, commandOutput(out))
	}
	if d.ewwDone == nil {
		fmt.Fprintln(os.Stderr, "ewwd: stopping unmanaged eww daemon after close")
		if err := d.stopEww(); err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: stop unmanaged eww: %v\n", err)
		}
	}
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
			go d.reconcileLatest(false)
			continue
		}
		if d.widgetsOpenPending() {
			go d.reconcileLatest(false)
			continue
		}
		if !d.desiredWidgetsOpen() {
			continue
		}
		if exec.Command("eww", "ping").Run() == nil {
			continue
		}

		fmt.Fprintln(os.Stderr, "ewwd: eww ping failed; reopening windows")
		go d.reconcileLatest(false)
	}
}

func commandOutput(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}
