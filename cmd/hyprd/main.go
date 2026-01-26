// hyprd — Unified Hyprland daemon for window management and eww integration.
//
// Usage:
//
//	hyprd              Start daemon (foreground)
//	hyprd status       Check if daemon is running
package main

// ================================================================================
// Entry point and CLI command dispatcher
// ================================================================================

import (
	"fmt"
	"os"
	"strings"

	"hyprd/daemon"
)

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
	case "pseudo":
		cmdPseudo()
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
	if daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon already running")
		os.Exit(1)
	}

	d, err := daemon.New()
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
	// Check for --json flag
	jsonOutput := false
	for _, arg := range os.Args[2:] {
		if arg == "--json" || arg == "-j" {
			jsonOutput = true
		}
	}

	if !daemon.IsRunning() {
		if jsonOutput {
			fmt.Println(`{"status":"not running"}`)
		} else {
			fmt.Println("not running")
		}
		os.Exit(1)
	}

	if jsonOutput {
		// Get full state from daemon
		resp, err := daemon.SendCommand("state")
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
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	resp, err := daemon.SendCommand("monocle")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdSplit() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	cmd := "split"
	if len(os.Args) > 2 {
		cmd += " " + os.Args[2]
	}

	resp, err := daemon.SendCommand(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdPseudo() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	resp, err := daemon.SendCommand("pseudo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdSwap() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	resp, err := daemon.SendCommand("swap")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdWS() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd ws <number>")
		os.Exit(1)
	}

	resp, err := daemon.SendCommand("ws " + os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdFocus() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyprd focus <class> [title]")
		os.Exit(1)
	}

	cmd := "focus " + strings.Join(os.Args[2:], " ")
	resp, err := daemon.SendCommand(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdQuery() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	topic := "all"
	if len(os.Args) > 2 {
		topic = os.Args[2]
	}

	resp, err := daemon.SendCommand("query " + topic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdSubscribe() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	cmd := "subscribe"
	if len(os.Args) > 2 {
		cmd += " " + strings.Join(os.Args[2:], " ")
	}

	resp, err := daemon.StreamCommand(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(resp)
}

func cmdLayout() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "hyprd: daemon not running")
		os.Exit(1)
	}

	arg := ""
	if len(os.Args) > 2 {
		arg = os.Args[2]
	}

	resp, err := daemon.SendCommand("layout " + arg)
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
  hyprd pseudo           Toggle pseudo-master (float slave over stack)
  hyprd swap             Toggle swap between master and slave
  hyprd split            Cycle split ratio (xs → default → lg)
  hyprd split -x|-d|-l   Set specific split ratio
  hyprd ws <n>           Switch to workspace n, focus master
  hyprd focus <class>    Focus window by class on current workspace

Sessions:
  hyprd layout --list    List available sessions
  hyprd layout <name>    Open session (acr, dotfiles, nosvagor)

Query/Subscribe (for eww):
  hyprd query [topic]    Get state (workspace|monocle|split|all)
  hyprd subscribe [...]  Stream events (workspace monocle split)`)
}
