// ewwd — System utilities daemon for eww statusbar integration.
package main

// Core daemon lifecycle and command routing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dotfiles/cmd/ewwd/providers"
	"dotfiles/cmd/internal/daemon"
)

const SocketPath = "/tmp/ewwd.sock"

// Daemon is the main ewwd daemon.
type Daemon struct {
	state     *State
	server    *daemon.Server
	providers []providers.Provider
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new daemon instance.
func New() (*Daemon, error) {
	ctx, cancel := context.WithCancel(context.Background())
	state := NewState()
	d := &Daemon{
		state:  state,
		ctx:    ctx,
		cancel: cancel,
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

// initProviders registers all data providers.
func (d *Daemon) initProviders() {
	d.providers = []providers.Provider{
		providers.NewGPU(d.state),
		providers.NewNetwork(d.state),
		providers.NewDate(d.state),
		providers.NewBrightness(d.state),
		providers.NewAudio(d.state),
		providers.NewMusic(d.state),
		providers.NewTimer(d.state),
		providers.NewWeather(d.state),
	}
}

// sendInitialState sends current state to a new subscriber.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	allState := d.state.GetAll()

	for topic, data := range allState {
		if data != nil && sub.WantsTopic(topic) {
			sub.SendEvent(topic, data)
		}
	}
}

// handleCommand dispatches commands and returns responses.
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

// query returns the current state for requested topics.
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

// handleAction routes action commands to the appropriate provider.
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
