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
	notifypkg "dotfiles/daemons/hyprd/notify"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
	case "init":
		cmdInit()
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
	case "browser":
		cmdBrowser()
	case "three-body":
		cmdThreeBody()
	case "shadow":
		cmdShadow()
	case "project":
		cmdProject()
	case "notify":
		cmdNotify()
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

func sendCommand(cmd string) {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}
	resp, err := client.Send(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func requireArg(usage string) string {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	return os.Args[2]
}

func cmdInit() {
	// Push Wayland env to the user bus, then start our daemons so they
	// inherit it. Without this, ewwd spawns `eww daemon` without
	// WAYLAND_DISPLAY and eww exits immediately. Both calls are idempotent.
	exec.Command("systemctl", "--user", "import-environment",
		"WAYLAND_DISPLAY", "XDG_CURRENT_DESKTOP", "HYPRLAND_INSTANCE_SIGNATURE").Run()
	exec.Command("systemctl", "--user", "start", "ewwd.service", "hyprd.service").Run()

	// Wait up to 10s for the daemon socket to appear.
	for range 100 {
		if client.IsRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	sendCommand("init")
}
func cmdHide()    { sendCommand("hide") }
func cmdMonocle() { sendCommand("monocle") }
func cmdSwap()    { sendCommand("swap") }
func cmdSplit()   { sendCommand("split " + strings.Join(os.Args[2:], " ")) }
func cmdLayout()  { sendCommand("layout " + strings.Join(os.Args[2:], " ")) }
func cmdBrowser() {
	_ = requireArg("usage: hyprd browser {windows|snapshot|show|hypr|restore} ...")
	sendCommand("browser " + strings.Join(os.Args[2:], " "))
}
func cmdProject() { sendCommand("project " + strings.Join(os.Args[2:], " ")) }
func cmdQuery()   { sendCommand("query " + strings.Join(os.Args[2:], " ")) }
func cmdBG()      { sendCommand("bg " + requireArg("usage: hyprd bg {ensure|kill}")) }
func cmdWS()      { sendCommand("ws " + requireArg("usage: hyprd ws <number|up|down>")) }
func cmdTab()     { sendCommand("tab " + requireArg("usage: hyprd tab <name|alias>")) }
func cmdThreeBody() {
	sendCommand("three-body " + requireArg("usage: hyprd three-body {editor|agents|browser|shadow}"))
}
func cmdShadow() { sendCommand("shadow " + strings.Join(os.Args[2:], " ")) }
func cmdFocus() {
	_ = requireArg("usage: hyprd focus <class> [title]")
	sendCommand("focus " + strings.Join(os.Args[2:], " "))
}
func cmdTabs() {
	_ = requireArg("usage: hyprd tabs {init|refresh} <profile|name> <pid>")
	sendCommand("tabs " + strings.Join(os.Args[2:], " "))
}
func cmdNotify() { notifypkg.CmdNotify(client, os.Args[2:]) }

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

func cmdHelp() {
	fmt.Println(`hyprd — Unified Hyprland daemon

Usage:
  hyprd                  Start daemon (foreground, auto-inits on fresh session)
  hyprd init             Manually run the boot sequence
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
  hyprd tab <name|alias>      Focus editor + switch kitty tab (aliases like nvim::fe-nvim supported)
  hyprd tabs init <profile> <pid>    Create tabs from profile (editor|agents|leadpier)
  hyprd tabs refresh <name|all> <pid> Refresh tab(s) in current profile

Three-body (2-visible, 1-shadow window management):
  hyprd three-body editor    Focus/launch editor window
  hyprd three-body agents    Focus/launch agents (checks notifications first)
  hyprd three-body browser   Focus/launch browser window
  hyprd three-body shadow    Toggle active/shadow slave

Shadow workspace (special:shadow):
  hyprd shadow               Toggle visibility of shadow workspace
  hyprd shadow list          List windows parked on shadow workspace

Sessions:
  hyprd layout --list    List available sessions
  hyprd layout <name>    Open session (loads from ~/dotfiles/daemons/configs/hyprd.yaml)

Browser snapshots:
  hyprd browser windows [--all] [--profile <name|path>]
  hyprd browser snapshot <name> [active|largest|index] [--profile <name|path>]
  hyprd browser show <name> [snapshot]
  hyprd browser hypr <name> [snapshot]
  hyprd browser restore <name> [snapshot] [--mode urls|exact] [--force] [--dry-run]

Query/Subscribe (for eww):
  hyprd query [topic]    Get state (workspace|hidden|split|three-body|all)
  hyprd subscribe [...]  Stream events (workspace split)

Notifications:
  hyprd notify hook claude <event>     Read Claude hook JSON from stdin
  hyprd notify hook codex              Read Codex notify JSON from argv/stdin
  hyprd notify dunst [approval]        Handle Dunst script callbacks
  hyprd notify kitty-finish <command>  Emit kitty command-finish notification`)
}
