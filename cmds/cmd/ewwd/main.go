// Package main implements ewwd, the daemon that publishes system state for eww widgets.
//
// Responsibilities:
// - Start providers and keep shared widget state synchronized.
// - Serve query, subscribe, and action commands over a Unix socket.
// - Expose a CLI for daemon lifecycle and provider actions.
package main

// main.go contains the ewwd CLI entrypoint and command routing.

import (
	"dotfiles/cmds/internal/daemon"
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
	case "open":
		cmdOpen()
	case "query":
		cmdQuery()
	case "subscribe":
		cmdSubscribe()
	case "action":
		cmdAction()
	case "help", "-h", "--help":
		cmdHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runDaemon() {
	if client.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon already running")
		os.Exit(1)
	}

	d, err := New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: %v\n", err)
		os.Exit(1)
	}

	if err := d.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: %v\n", err)
		os.Exit(1)
	}
}

// cmdStatus prints "running"/"not running" and exits non-zero when unreachable.
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

func cmdOpen() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
		os.Exit(1)
	}

	resp, err := client.Send("open")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdQuery() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
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
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
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

func cmdAction() {
	if !client.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ewwd action <provider> [args...]")
		os.Exit(1)
	}

	cmd := "action " + strings.Join(os.Args[2:], " ")
	resp, err := client.Send(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdHelp() {
	fmt.Println(`ewwd — System utilities daemon for eww

Usage:
  ewwd                  Start daemon (foreground, auto-opens eww windows)
  ewwd open             Reload eww config and reopen configured windows
  ewwd status           Check if daemon is running
  ewwd status --json    Return full state as JSON

Query/Subscribe (for eww):
  ewwd query [topic]    Get state (network|date|audio|music|timer|weather|...)
  ewwd subscribe [...]  Stream events (network date audio music timer weather)

Actions (for eww buttons/scrolls):
  ewwd action audio mute <sink|source>      Mute device
  ewwd action audio change_volume sink up   Adjust ±10
  ewwd action audio set_default both        Preset volumes
  ewwd action music play                    Start playback
  ewwd action music pause                   Pause playback
  ewwd action music toggle                  Toggle play/pause
  ewwd action music next                    Next track
  ewwd action music previous                Previous track
  ewwd action music volume up [0.05]        Increase volume
  ewwd action music volume down [0.05]      Decrease volume
  ewwd action music seek up                 Seek forward 10s
  ewwd action music seek down               Seek backward 10s
  ewwd action timer timer up <minutes>      Add minutes to timer
  ewwd action timer timer down <minutes>    Subtract minutes from timer
  ewwd action timer timer start             Start timer countdown
  ewwd action timer timer reset             Stop and reset to 01:30
  ewwd action timer alarm up <minutes>      Add minutes to alarm target
  ewwd action timer alarm down <minutes>    Subtract minutes from alarm
  ewwd action timer alarm start             Start alarm countdown
  ewwd action timer alarm reset             Stop and reset to +6 hours

Providers:
  network    - Network speed monitoring
  date       - Date/time, clockface icons, weeks alive
  audio      - PulseAudio volume (sink/source with offset)
  music      - Spotify playback (status, track info, album art)
  timer      - Timer/alarm countdown with notifications
  weather    - OpenWeatherMap data (temp, conditions, moon, wind)`)
}
