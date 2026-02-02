// Package main implements hyprd, a daemon for managing Hyprland window layouts
// and state synchronization with eww widgets.
//
// hyprd connects to Hyprland's IPC sockets to monitor workspace changes,
// window events, and execute window management commands. It maintains
// state for features like monocle mode, split ratios, and hidden windows.
//
// # Architecture
//
// The daemon consists of three main components:
//
//   - Daemon: Manages the command socket and routes client requests
//   - EventLoop: Subscribes to Hyprland events and updates state
//   - State: Thread-safe storage for workspace and window tracking
//
// # Commands
//
// Clients connect via Unix socket and send text commands:
//
//   - monocle: Toggle focused window to float on dedicated workspace
//   - split: Cycle or set the master/slave split ratio
//   - hide: Toggle visibility of slave windows
//   - swap: Exchange master and slave window positions
//   - ws: Switch workspace with automatic master focus
//   - focus: Focus window by class, unhiding if necessary
//   - layout: Apply predefined window arrangements
//   - query: Retrieve current state as JSON
//   - subscribe: Stream state change events
//
// # Integration
//
// hyprd provides real-time state updates to eww widgets through a
// subscription mechanism. Widgets subscribe to topics (workspace, monocle,
// split) and receive JSON events when state changes.
package main

import (
	"dotfiles/cmd/internal/daemon"
	"fmt"
	"os"
	"strings"
)

var client = daemon.NewClient(SocketPath)

func main() {
	if len(os.Args) < 2 {
		runDaemon()
		return
	}

	switch os.Args[1] {
	case "status":
		cmdStatus()
	case "monocle":
		cmdMonocle()
	case "split":
		cmdSplit()
	case "hide":
		cmdHide()
	case "swap":
		cmdSwap()
	case "ws":
		cmdWS()
	case "focus":
		cmdFocus()
	case "query":
		cmdQuery()
	case "subscribe":
		cmdSubscribe()
	case "layout":
		cmdLayout()
	case "help", "-h", "--help":
		cmdHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runDaemon() {
	if client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon already running")
		os.Exit(1)
	}

	d, err := New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: %v\n", err)
		os.Exit(1)
	}

	if err := d.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd: %v\n", err)
		os.Exit(1)
	}
}

func cmdStatus() {
	// Check for –json flag
	jsonOutput := false
	for _, arg := range os.Args[2:] {
		if arg == "--json" || arg == "-j" {
			jsonOutput = true
		}
	}

	if !client.IsRunning() {
		if jsonOutput {
			fmt.Println(`{"status":"not running"}`)
		} else {
			fmt.Println("not running")
		}
		os.Exit(1)
	}

	if jsonOutput {
		// Get full state from daemon
		resp, err := client.Send("state")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)
	} else {
		fmt.Println("running")
	}
}

func cmdMonocle() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	resp, err := client.Send("monocle")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdSplit() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	cmd := "split"
	if len(os.Args) > 2 {
		cmd += " " + os.Args[2]
	}

	resp, err := client.Send(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdHide() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	resp, err := client.Send("hide")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdSwap() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	resp, err := client.Send("swap")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdWS() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd ws <number>")
		os.Exit(1)
	}

	resp, err := client.Send("ws " + os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdFocus() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd focus <class> [title]")
		os.Exit(1)
	}

	cmd := "focus " + strings.Join(os.Args[2:], " ")
	resp, err := client.Send(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdQuery() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	topic := "all"
	if len(os.Args) > 2 {
		topic = os.Args[2]
	}

	resp, err := client.Send("query " + topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdSubscribe() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	cmd := "subscribe"
	if len(os.Args) > 2 {
		cmd += " " + strings.Join(os.Args[2:], " ")
	}

	err := client.Stream(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLayout() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	arg := ""
	if len(os.Args) > 2 {
		arg = os.Args[2]
	}

	resp, err := client.Send("layout " + arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdHelp() {
	fmt.Println(`hyprd — Unified Hyprland daemon

Usage:
  hyprd                  Start daemon (foreground)
  hyprd status           Check if daemon is running
  hyprd status --json    Return full state as JSON

Window commands:
  hyprd monocle          Toggle monocle mode (float to WS6)
  hyprd hide             Toggle hide/show slave (special workspace)
  hyprd swap             Toggle swap between master and slave
  hyprd split            Cycle split ratio (xs → default → lg)
  hyprd split -x|-d|-l   Set specific split ratio
  hyprd ws <n>           Switch to workspace n, focus master
  hyprd focus <class> [title]  Focus window, unhide if hidden

Sessions:
  hyprd layout --list    List available sessions
  hyprd layout <name>    Open session (loads from ~/.config/hyprd/sessions.yaml)

Query/Subscribe (for eww):
  hyprd query [topic]    Get state (workspace|monocle|hidden|split|all)
  hyprd subscribe [...]  Stream events (workspace monocle split)`)
}
