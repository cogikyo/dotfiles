package main

// daemon.go defines ewwd runtime orchestration, provider wiring, and socket command handlers.

import (
	"context"
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/daemon"
	"dotfiles/cmds/internal/ewwd/providers"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	config      config.EwwConfig
	autoOpen    bool
	openMu      sync.Mutex
	reconcileMu sync.Mutex
	desiredOpen bool
}

func New(autoOpen bool) (*Daemon, error) {
	cfg := config.LoadEww()

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
	cfg := d.config
	d.providers = []providers.Provider{
		providers.NewNetwork(d.state, cfg.Network),
		providers.NewDate(d.state, cfg.Date),
		providers.NewAudio(d.state, cfg.Audio),
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
	d.openMu.Unlock()

	go d.reconcileLatest(reload)
	return "open: scheduled"
}

func (d *Daemon) closeWindows() string {
	d.openMu.Lock()
	d.desiredOpen = false
	d.openMu.Unlock()

	go d.killEww()
	return "close: scheduled"
}

func (d *Daemon) desiredWidgetsOpen() bool {
	d.openMu.Lock()
	defer d.openMu.Unlock()
	return d.desiredOpen
}

func (d *Daemon) reconcileLatest(reload bool) {
	d.reconcileMu.Lock()
	defer d.reconcileMu.Unlock()

	if !d.desiredWidgetsOpen() {
		d.killEww()
		return
	}

	result, ok := d.reconcileWindows(reload)
	if !ok {
		fmt.Fprintf(os.Stderr, "ewwd: reconcile failed: %s\n", result)
	}
	if !d.desiredWidgetsOpen() {
		d.killEww()
		return
	}
	if result != "" {
		fmt.Printf("ewwd: %s\n", result)
	}
}

func (d *Daemon) reconcileWindows(reload bool) (string, bool) {
	windows := d.config.Windows
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

	d.killEww()
	if err := ensureEwwDaemon(); err != nil {
		return fmt.Sprintf("error: %v", err), false
	}
	if reload {
		if out, err := exec.Command("eww", "reload").CombinedOutput(); err != nil {
			return fmt.Sprintf("error: eww reload: %v%s", err, commandOutput(out)), false
		}
	}

	for _, window := range windows {
		if out, err := exec.Command("eww", "close", window).CombinedOutput(); err != nil && len(out) > 0 {
			fmt.Fprintf(os.Stderr, "ewwd: eww close %s ignored: %v%s\n", window, err, commandOutput(out))
		}
	}

	args := append([]string{"open-many"}, windows...)
	if out, err := exec.Command("eww", args...).CombinedOutput(); err != nil {
		return fmt.Sprintf("error: eww open-many: %v%s", err, commandOutput(out)), false
	}

	return fmt.Sprintf("%s: %s", verb, strings.Join(windows, " ")), true
}

func (d *Daemon) killEww() {
	if out, err := exec.Command("pkill", "-x", "eww").CombinedOutput(); err != nil && !isExitCode(err, 1) {
		fmt.Fprintf(os.Stderr, "ewwd: pkill eww: %v%s\n", err, commandOutput(out))
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

		if !d.desiredWidgetsOpen() {
			continue
		}
		if exec.Command("eww", "ping").Run() == nil {
			continue
		}

		fmt.Fprintln(os.Stderr, "ewwd: eww ping failed; restarting daemon and reconciling windows")
		go d.reconcileLatest(false)
	}
}

func isExitCode(err error, code int) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == code
}

func ensureEwwDaemon() error {
	if out, err := exec.Command("eww", "ping").CombinedOutput(); err == nil {
		return nil
	} else if len(out) > 0 {
		fmt.Fprintf(os.Stderr, "ewwd: initial eww ping failed: %v%s\n", err, commandOutput(out))
	}

	cmd := exec.Command("eww", "daemon")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("eww daemon: %w", err)
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	var lastPing []byte
	var lastPingErr error
	for range 20 {
		select {
		case err := <-waitErr:
			if err != nil {
				return fmt.Errorf("eww daemon exited before ready: %w", err)
			}
			return fmt.Errorf("eww daemon exited before ready")
		default:
		}

		lastPing, lastPingErr = exec.Command("eww", "ping").CombinedOutput()
		if lastPingErr == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("eww daemon did not become ready: last ping: %v%s", lastPingErr, commandOutput(lastPing))
}

func commandOutput(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}
