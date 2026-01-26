// ewwd — System utilities daemon for eww statusbar integration.
//
// Usage:
//
//	ewwd              Start daemon (foreground)
//	ewwd status       Check if daemon is running
package main

// ================================================================================
// Entry point and CLI command dispatcher
// ================================================================================

import (
	"ewwd/daemon"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		runDaemon()
		return
	}

	switch os.Args[1] {
	case "status":
		cmdStatus()
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
	if daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon already running")
		os.Exit(1)
	}

	d, err := daemon.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: %v\n", err)
		os.Exit(1)
	}

	if err := d.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: %v\n", err)
		os.Exit(1)
	}
}

func cmdStatus() {
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

func cmdQuery() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
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
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
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

func cmdAction() {
	if !daemon.IsRunning() {
		fmt.Fprintln(os.Stderr, "ewwd: daemon not running")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ewwd action <provider> [args...]")
		os.Exit(1)
	}

	cmd := "action " + strings.Join(os.Args[2:], " ")
	resp, err := daemon.SendCommand(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}

func cmdHelp() {
	fmt.Println(`ewwd — System utilities daemon for eww

Usage:
  ewwd                  Start daemon (foreground)
  ewwd status           Check if daemon is running
  ewwd status --json    Return full state as JSON

Query/Subscribe (for eww):
  ewwd query [topic]    Get state (gpu|network|date|brightness|audio|music|timer|weather|...)
  ewwd subscribe [...]  Stream events (gpu network date brightness audio music timer weather)

Actions (for eww buttons/scrolls):
  ewwd action brightness reset              Set to 100%
  ewwd action brightness night              Set to 40%
  ewwd action brightness adjust up          Increase by 10%
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
  gpu        - AMD GPU stats (busy%, VRAM, mclk)
  network    - Network speed monitoring
  date       - Date/time, clockface icons, weeks alive
  brightness - Screen brightness control
  audio      - PulseAudio volume (sink/source with offset)
  music      - Spotify playback (status, track info, album art)
  timer      - Timer/alarm countdown with notifications
  weather    - OpenWeatherMap data (temp, conditions, moon, wind)`)
}
