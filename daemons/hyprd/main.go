// Package main provides the hyprd daemon process and CLI front-end.
//
// Responsibilities:
// - Boot and supervise the daemon runtime.
// - Route CLI verbs over the Unix socket protocol.
// - Expose status, subscription, and maintenance commands.
package main

// main.go is the executable entrypoint that parses CLI verbs and forwards daemon commands.

import (
	"dotfiles/daemons/daemon"
	"dotfiles/daemons/hyprd/cli"
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
	case "picker":
		cmdPicker()
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
	case "lock":
		cmdLock()
	case "notify":
		cmdNotify()
	case "vpn":
		cli.VPN()
	case "screenshot":
		cli.Screenshot()
	case "ssh":
		cli.SSH()
	case "rebuild":
		cmdRebuild()
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
	d.restoreState()

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

// cmdInit imports Wayland env into systemd, starts user services, and waits for hyprd.
func cmdInit() {
	envNames := []string{
		"WAYLAND_DISPLAY", "HYPRLAND_INSTANCE_SIGNATURE",
		"XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE", "XDG_SESSION_DESKTOP",
		"HYPRCURSOR_THEME", "HYPRCURSOR_SIZE",
		"XCURSOR_THEME", "XCURSOR_SIZE",
		"QT_QPA_PLATFORM", "QT_QPA_PLATFORMTHEME",
		"QT_WAYLAND_DISABLE_WINDOWDECORATION", "QT_SCALE_FACTOR", "QT_STYLE_OVERRIDE",
		"GDK_BACKEND", "GDK_DPI_SCALE",
		"SDL_VIDEODRIVER",
	}
	systemdArgs := append([]string{"--user", "import-environment"}, envNames...)
	exec.Command("systemctl", systemdArgs...).Run()
	dbusArgs := append([]string{"--systemd"}, envNames...)
	exec.Command("dbus-update-activation-environment", dbusArgs...).Run()
	exec.Command("systemctl", "--user", "start", "hyprd.service", "opencode.service").Run()

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
func cmdPicker()  { sendCommand("picker " + strings.Join(os.Args[2:], " ")) }
func cmdLayout()  { sendCommand("layout " + strings.Join(os.Args[2:], " ")) }
func cmdBrowser() {
	_ = requireArg("usage: hyprd browser {windows|snapshot|show|hypr|restore} ...")
	sendCommand("browser " + strings.Join(os.Args[2:], " "))
}
func cmdProject() { sendCommand("project " + strings.Join(os.Args[2:], " ")) }
func cmdLock()    { sendCommand("lock " + strings.Join(os.Args[2:], " ")) }
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
func cmdNotify()  { notifypkg.CmdNotify(client, os.Args[2:]) }
func cmdRebuild() { sendCommand("rebuild") }

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
  hyprd rebuild          Rebuild binary and hot-restart (preserves state)

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
  hyprd picker open      Open interactive layout picker overlay
  hyprd picker close     Close picker without action
  hyprd picker confirm   Confirm selection (open session on workspace)
  hyprd layout --list    List available sessions
  hyprd layout <name>    Open session (loads from ~/dotfiles/daemons/config/hyprd.yaml)

Lock:
  hyprd lock             Pseudo-lock (visual blackout + submap)
  hyprd lock unlock      Exit pseudo-lock (alias: -u)
  hyprd lock full        Full lock (wraps hyprlock with pre/post hooks)

Browser:
  hyprd browser launch [--profile <name|path>]
  hyprd browser windows [--all] [--profile <name|path>]
  hyprd browser snapshot <name> [active|largest|index] [--profile <name|path>]
  hyprd browser show <name>
  hyprd browser hypr <name>
  hyprd browser restore <name> [--mode urls|exact] [--force] [--dry-run]

Query/Subscribe (for eww):
  hyprd query [topic]    Get state (workspace|hidden|split|three-body|all)
  hyprd subscribe [...]  Stream events (workspace split)

Screenshot:
  hyprd screenshot              Region screenshot to clipboard
  hyprd screenshot annotate     Region screenshot → satty annotation → clipboard

VPN:
  hyprd vpn work                Toggle configured NetworkManager VPN alias
  hyprd vpn work up|down        Connect/disconnect explicitly
  hyprd vpn install work        Import staged .nmconnection via NetworkManager
  hyprd vpn export work         Export NetworkManager profile to staged file

Notifications:
  hyprd notify hook claude <event>     Read Claude hook JSON from stdin
  hyprd notify hook opencode           Read OpenCode notify JSON from argv/stdin
	  hyprd notify dunst                   Handle Dunst script callbacks
  hyprd notify kitty-finish <command>  Emit kitty command-finish notification`)
}
