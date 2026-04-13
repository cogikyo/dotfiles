package main

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
)

const SocketPath = "/tmp/ewwd.sock"

// Daemon orchestrates providers and routes commands between clients and state updates.
type Daemon struct {
	state     *State               // Thread-safe data store
	server    *daemon.Server       // Unix socket server
	providers []providers.Provider // Data sources
	ctx       context.Context      // Cancellation context
	cancel    context.CancelFunc   // Triggers provider shutdown
	config    *config.Config       // YAML configuration
}

// New loads configuration and initializes the daemon with server and state.
func New() (*Daemon, error) {
	cfg := config.Load()

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

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run() error {
	if err := d.server.Start(); err != nil {
		return err
	}
	fmt.Printf("ewwd: listening on %s\n", SocketPath)

	// Initialize and start providers
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

	// Open eww windows
	go func() {
		if result := d.openWindows(); result != "" {
			fmt.Printf("ewwd: %s\n", result)
		}
	}()

	// Wait for signal
	sig := d.server.WaitForSignal()
	fmt.Printf("\newwd: received %s, shutting down\n", sig)
	d.cancel()
	d.server.Shutdown()

	// Stop all providers
	for _, p := range d.providers {
		p.Stop()
	}

	return nil
}

// initProviders instantiates all providers with their configuration.
func (d *Daemon) initProviders() {
	cfg := d.config.Eww
	d.providers = []providers.Provider{
		providers.NewGPU(d.state, cfg.GPU),
		providers.NewNetwork(d.state, cfg.Network),
		providers.NewDate(d.state, cfg.Date),
		providers.NewBrightness(d.state, cfg.Brightness),
		providers.NewAudio(d.state, cfg.Audio),
		providers.NewMusic(d.state),
		providers.NewTimer(d.state, cfg.Timer),
		providers.NewWeather(d.state, cfg.Weather),
	}
}

// sendInitialState sends existing state to new subscribers for their requested topics.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	allState := d.state.GetAll()

	for topic, data := range allState {
		if data != nil && sub.WantsTopic(topic) {
			sub.SendEvent(topic, data)
		}
	}
}

// handleCommand parses and routes client commands (status, query, action) to handlers.
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
		// Format: action <provider> [args...]
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

// query returns JSON state for a topic or all state if topic is "all" or empty.
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

// handleAction finds the provider by name and delegates action execution with args.
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
	windows := d.config.Eww.Windows
	if len(windows) == 0 {
		return "open: no windows configured"
	}

	// Ensure eww daemon is running
	if err := exec.Command("eww", "ping").Run(); err != nil {
		if err := exec.Command("eww", "daemon").Run(); err != nil {
			return fmt.Sprintf("error: eww daemon failed: %v", err)
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
