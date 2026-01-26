// Package daemon implements the ewwd background service that manages system providers,
// provides Unix socket IPC at /tmp/ewwd.sock, and streams events to subscribers like eww widgets.
package daemon

// ================================================================================
// Core daemon lifecycle, socket server, and command routing
// ================================================================================

import (
	"context"
	"ewwd/providers"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const SocketPath = "/tmp/ewwd.sock"

// Daemon is the main ewwd daemon.
type Daemon struct {
	state     *State
	subs      *SubscriptionManager
	providers []providers.Provider
	listener  net.Listener
	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new daemon instance.
func New() (*Daemon, error) {
	ctx, cancel := context.WithCancel(context.Background())
	state := NewState()
	return &Daemon{
		state:  state,
		subs:   NewSubscriptionManager(state),
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run() error {
	// Remove stale socket
	os.Remove(SocketPath)

	listener, err := net.Listen("unix", SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", SocketPath, err)
	}
	d.listener = listener

	// Make socket world-accessible (for eww, scripts, etc.)
	if err := os.Chmod(SocketPath, 0o666); err != nil {
		listener.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	fmt.Printf("ewwd: listening on %s\n", SocketPath)

	// Initialize and start providers
	d.initProviders()
	for _, p := range d.providers {
		go func(p providers.Provider) {
			notify := func(data any) {
				d.subs.Notify(p.Name(), data)
			}
			if err := p.Start(d.ctx, notify); err != nil {
				fmt.Fprintf(os.Stderr, "ewwd: provider %s error: %v\n", p.Name(), err)
			}
		}(p)
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go d.acceptLoop()

	// Wait for signal
	sig := <-sigCh
	fmt.Printf("\newwd: received %s, shutting down\n", sig)
	d.cancel()
	close(d.done)
	d.listener.Close()
	os.Remove(SocketPath)

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

// acceptLoop handles incoming client connections.
func (d *Daemon) acceptLoop() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.done:
				return
			default:
				fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
				continue
			}
		}
		go d.handleClient(conn)
	}
}

// handleClient processes a single client connection.
func (d *Daemon) handleClient(conn net.Conn) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		conn.Close()
		return
	}

	command := strings.TrimSpace(string(buf[:n]))

	// Handle subscribe specially - keep connection open
	if strings.HasPrefix(command, "subscribe") {
		topics := ParseSubscribeCommand(command)
		d.subs.Subscribe(conn, topics)
		// Block until client disconnects
		buf := make([]byte, 1)
		conn.Read(buf)
		d.subs.Unsubscribe(conn)
		conn.Close()
		return
	}

	// Normal request-response
	response := d.handleCommand(command)
	// Ignore write errors - client may disconnect before receiving response
	conn.Write([]byte(response))
	conn.Close()
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
		result, err := Query(d.state, topic)
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

// IsRunning checks if ewwd daemon is already running.
func IsRunning() bool {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.Write([]byte("ping"))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	return string(buf[:n]) == "pong"
}
