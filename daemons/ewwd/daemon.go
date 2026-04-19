package main

// daemon.go defines ewwd runtime orchestration, provider wiring, and socket command handlers.
import (
	"context"
	"dotfiles/daemons/config"
	"dotfiles/daemons/daemon"
	"dotfiles/daemons/ewwd/providers"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	state     *State
	server    *daemon.Server
	providers []providers.Provider
	ctx       context.Context
	cancel    context.CancelFunc
	config    config.EwwConfig
}

func New() (*Daemon, error) {
	cfg := config.LoadEww()

	ctx, cancel := context.WithCancel(context.Background())
	state := NewState()
	d := &Daemon{
		state:  state,
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
	}

	d.server = daemon.NewServer(SocketPath, d.handleCommand)
	d.server.OnSubscribe = d.sendInitialState

	return d, nil
}

// Run starts the server, launches providers, opens eww windows, and blocks until signalled.
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

	// eww startup can take seconds; don't block signal handling.
	go func() {
		if result := d.openWindows(); result != "" {
			fmt.Printf("ewwd: %s\n", result)
		}
	}()

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
		providers.NewGPU(d.state, cfg.GPU),
		providers.NewNetwork(d.state, cfg.Network),
		providers.NewDate(d.state, cfg.Date),
		providers.NewAudio(d.state, cfg.Audio),
		providers.NewMusic(d.state),
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
		return d.openWindows()
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

// openWindows ensures the eww daemon is running, reloads config, and opens configured windows.
func (d *Daemon) openWindows() string {
	windows := d.config.Windows
	if len(windows) == 0 {
		return "open: no windows configured"
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		importSystemdEnv()
		if os.Getenv("WAYLAND_DISPLAY") == "" {
			return "error: WAYLAND_DISPLAY not set"
		}
	}

	// Kill stale daemon and respawn, waiting up to 10s for readiness.
	if err := exec.Command("eww", "ping").Run(); err != nil {
		exec.Command("eww", "kill").Run()
		time.Sleep(200 * time.Millisecond)
		cmd := exec.Command("eww", "daemon")
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
		}
		ready := false
		for range 50 {
			if exec.Command("eww", "ping").Run() == nil {
				ready = true
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if !ready {
			return "error: eww daemon failed to start"
		}
	}

	exec.Command("eww", "reload").Run()
	exec.Command("eww", "close-all").Run()

	args := append([]string{"open-many"}, windows...)
	if err := exec.Command("eww", args...).Run(); err != nil {
		return fmt.Sprintf("error: eww open-many: %v", err)
	}

	return fmt.Sprintf("open: %s", strings.Join(windows, " "))
}
