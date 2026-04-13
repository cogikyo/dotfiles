// Package main implements hyprd, a daemon for managing Hyprland window layouts
// and state synchronization with eww widgets.
//
// hyprd connects to Hyprland's IPC sockets to monitor workspace changes,
// window events, and execute window management commands. It maintains
// state for features like split ratios, hidden windows, and three-body layouts.
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
// subscription mechanism. Widgets subscribe to topics (workspace, split)
// and receive JSON events when state changes.
package main

import (
	"dotfiles/daemons/daemon"
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
	case "bg":
		cmdBG()
	case "split":
		cmdSplit()
	case "hide":
		cmdHide()
	case "monocle":
		cmdMonocle()
	case "swap":
		cmdSwap()
	case "ws":
		cmdWS()
	case "focus":
		cmdFocus()
	case "tab":
		cmdTab()
	case "tabs":
		cmdTabs()
	case "query":
		cmdQuery()
	case "subscribe":
		cmdSubscribe()
	case "layout":
		cmdLayout()
	case "three-body":
		cmdThreeBody()
	case "project":
		cmdProject()
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

func cmdBG() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd bg {ensure|kill}")
		os.Exit(1)
	}

	resp, err := client.Send("bg " + os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdStatus() {
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
		fmt.Fprintln(os.Stderr, "usage: hyprd ws <number|up|down>")
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

func cmdTab() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd tab {term|nvim|nvimtree|git|xplr}")
		os.Exit(1)
	}

	resp, err := client.Send("tab " + os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdTabs() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd tabs {init|refresh} <profile|name> <pid>")
		os.Exit(1)
	}

	cmd := "tabs " + strings.Join(os.Args[2:], " ")
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

func cmdThreeBody() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd three-body {editor|agents|browser|shadow}")
		os.Exit(1)
	}

	resp, err := client.Send("three-body " + os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdProject() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	cmd := "project"
	if len(os.Args) > 2 {
		cmd += " " + strings.Join(os.Args[2:], " ")
	}

	resp, err := client.Send(cmd)
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
  hyprd bg <mode>        Background: code, music, kill, lock, ensure
  hyprd hide             Toggle hide/show slave (special workspace)
  hyprd monocle          Toggle monocle (isolate focused window)
  hyprd swap             Toggle swap between master and slave
  hyprd split            Cycle split ratio (xs → default → lg)
  hyprd split -x|-l      Set specific split ratio
  hyprd ws <n>           Switch to workspace n, focus master
  hyprd ws up|down       Move active window between workspaces 2..5
  hyprd focus <class> [title]  Focus window, unhide if hidden
  hyprd tab <name>            Focus editor + switch kitty tab (term|nvim|nvimtree|git|xplr)
  hyprd tabs init <profile> <pid>    Create tabs from profile (editor|agents|leadpier)
  hyprd tabs refresh <name|all> <pid> Refresh tab(s) in current profile

Three-body (2-visible, 1-shadow window management):
  hyprd three-body editor    Focus/launch editor window
  hyprd three-body agents    Focus/launch agents (checks notifications first)
  hyprd three-body browser   Focus/launch browser window
  hyprd three-body shadow    Toggle active/shadow slave

Sessions:
  hyprd layout --list    List available sessions
  hyprd layout <name>    Open session (loads from ~/.config/hyprd/sessions.yaml)

Query/Subscribe (for eww):
  hyprd query [topic]    Get state (workspace|hidden|split|three-body|all)
  hyprd subscribe [...]  Stream events (workspace split)`)
}
