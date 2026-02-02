// hyprd — Unified Hyprland daemon for window management and eww integration.
package main

// Core daemon lifecycle and command routing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dotfiles/cmd/hyprd/commands"
	"dotfiles/cmd/hyprd/hypr"
	"dotfiles/cmd/internal/daemon"
)

const SocketPath = "/tmp/hyprd.sock"

// Daemon is the main hyprd daemon.
type Daemon struct {
	hypr   *hypr.Client
	state  *State
	server *daemon.Server
}

// New creates a new daemon instance.
func New() (*Daemon, error) {
	hyprClient, err := hypr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("connect to hyprland: %w", err)
	}

	state := NewState()
	d := &Daemon{
		hypr:  hyprClient,
		state: state,
	}

	d.server = daemon.NewServer(SocketPath, d.handleCommand)
	d.server.OnSubscribe = d.sendInitialState

	return d, nil
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run() error {
	// Verify Hyprland connection
	clients, err := d.hypr.Clients()
	if err != nil {
		return fmt.Errorf("hyprland query failed: %w", err)
	}
	fmt.Printf("hyprd: connected to hyprland (%d clients)\n", len(clients))

	if err := d.server.Start(); err != nil {
		return err
	}
	fmt.Printf("hyprd: listening on %s\n", SocketPath)

	// Start event loop
	events := NewEventLoop(d.hypr, d.state, d.server.Subs, d.server.Done())
	go func() {
		if err := events.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd: event loop error: %v\n", err)
		}
	}()

	// Wait for signal
	sig := d.server.WaitForSignal()
	fmt.Printf("\nhyprd: received %s, shutting down\n", sig)
	d.server.Shutdown()

	return nil
}

// sendInitialState sends current state to a new subscriber.
func (d *Daemon) sendInitialState(sub *daemon.Subscriber, topics []string) {
	if sub.WantsTopic("workspace") {
		data := map[string]any{
			"current":  d.state.GetWorkspace(),
			"occupied": d.state.GetOccupied(),
		}
		sub.SendEvent("workspace", data)
	}

	if sub.WantsTopic("monocle") {
		monocle := d.state.GetMonocle()
		var data any
		if monocle != nil {
			data = map[string]any{
				"address":   monocle.Address,
				"origin_ws": monocle.OriginWS,
			}
		}
		sub.SendEvent("monocle", data)
	}

	if sub.WantsTopic("split") {
		sub.SendEvent("split", d.state.GetSplitRatio())
	}
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
		result, err := d.query(topic)
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

// query returns the current state for requested topics.
func (d *Daemon) query(topic string) (string, error) {
	switch topic {
	case "workspace":
		data := map[string]any{
			"current":  d.state.GetWorkspace(),
			"occupied": d.state.GetOccupied(),
		}
		jsonData, err := json.Marshal(data)
		return string(jsonData), err

	case "monocle":
		monocle := d.state.GetMonocle()
		if monocle == nil {
			return "null", nil
		}
		jsonData, err := json.Marshal(monocle)
		return string(jsonData), err

	case "pseudo":
		pseudo := d.state.GetPseudo()
		if pseudo == nil {
			return "null", nil
		}
		jsonData, err := json.Marshal(pseudo)
		return string(jsonData), err

	case "split":
		return fmt.Sprintf(`"%s"`, d.state.GetSplitRatio()), nil

	case "all", "":
		jsonData, err := d.state.JSON()
		return string(jsonData), err

	default:
		return "", fmt.Errorf("unknown topic: %s", topic)
	}
}
