// Package daemon implements the hyprd background service that manages Hyprland window state,
// provides Unix socket IPC at /tmp/hyprd.sock, and streams events to subscribers like eww widgets.
package daemon

// ================================================================================
// Core daemon lifecycle, socket server, and command routing
// ================================================================================

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"hyprd/commands"
	"hyprd/hypr"
)

const SocketPath = "/tmp/hyprd.sock"

// Daemon is the main hyprd daemon.
type Daemon struct {
	hypr     *hypr.Client
	state    *State
	subs     *SubscriptionManager
	listener net.Listener
	done     chan struct{}
}

// New creates a new daemon instance.
func New() (*Daemon, error) {
	hyprClient, err := hypr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connect to hyprland: %w", err)
	}

	state := NewState()
	return &Daemon{
		hypr:  hyprClient,
		state: state,
		subs:  NewSubscriptionManager(state),
		done:  make(chan struct{}),
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

	// Verify Hyprland connection
	clients, err := d.hypr.Clients()
	if err != nil {
		listener.Close()
		return fmt.Errorf("hyprland query failed: %w", err)
	}
	fmt.Printf("hyprd: connected to hyprland (%d clients)\n", len(clients))
	fmt.Printf("hyprd: listening on %s\n", SocketPath)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go d.acceptLoop()

	// Start event loop
	events := NewEventLoop(d.hypr, d.state, d.subs, d.done)
	go func() {
		if err := events.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd: event loop error: %v\n", err)
		}
	}()

	// Wait for signal
	sig := <-sigCh
	fmt.Printf("\nhyprd: received %s, shutting down\n", sig)
	close(d.done)
	d.listener.Close()
	os.Remove(SocketPath)

	return nil
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
		// Block until client disconnects (read will fail when connection closes)
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
	// Parse command and arguments
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
	case "monocle":
		monocle := commands.NewMonocle(d.hypr, d.state)
		result, err := monocle.Execute()
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
	case "pseudo":
		pseudo := commands.NewPseudo(d.hypr, d.state)
		result, err := pseudo.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "swap":
		swap := commands.NewSwap(d.hypr, d.state)
		result, err := swap.Execute()
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "ws":
		if arg == "" {
			return "error: workspace number required"
		}
		ws := commands.NewWS(d.hypr)
		result, err := ws.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	case "focus":
		// Parse "class" or "class title"
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
	case "layout":
		layout := commands.NewLayout(d.hypr, d.state)
		result, err := layout.Execute(arg)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return result
	default:
		return fmt.Sprintf("unknown command: %s", cmd)
	}
}

// IsRunning checks if hyprd daemon is already running.
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
