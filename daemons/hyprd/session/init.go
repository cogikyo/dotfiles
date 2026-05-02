// Package session orchestrates workspace sessions, lock flows, and kitty tab automation.
//
// Responsibilities:
// - Run startup initialization for configured workspaces.
// - Launch and arrange per-workspace session layouts.
// - Provide lock, picker, and tab-control helpers used by daemon commands.
package session

// init.go executes boot-time session initialization, including wallpaper, optional early lock, network wait, and layout open.

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

var startupExecs = []string{
	"glava -e bars_rc.glsl",
	"glava -e bars_r_rc.glsl",
	"glava -e radial_rc.glsl",
	"spotify-launcher",
}

// dispatchStartup runs hardcoded startup commands and optionally connects bluetooth.
func dispatchStartup(h *hypr.Client, bt config.BluetoothConfig) {
	for _, cmd := range startupExecs {
		h.Dispatch(fmt.Sprintf("exec %s", cmd))
	}
	if bt.Enabled && bt.Device != "" {
		h.Dispatch(fmt.Sprintf("exec bluetoothctl connect %s", bt.Device))
	}
}

// NotifyFunc delivers a notification to the user, injected to break a cycle with the notify package.
type NotifyFunc func(app, urgency, title, body string)

// Init drives first-boot session setup: background, optional early lock, network wait, and per-workspace layouts.
type Init struct {
	hypr   *hypr.Client
	state  *state.State
	notify NotifyFunc
	lock   *Lock
}

func NewInit(h *hypr.Client, s *state.State) *Init {
	return &Init{hypr: h, state: s}
}

func (i *Init) SetLock(l *Lock) {
	i.lock = l
}

func (i *Init) SetNotify(fn NotifyFunc) {
	i.notify = fn
}

// Execute runs the full init sequence: background, optional early lock, network, workspace layouts, and startup execs.
//
// Inter-dispatch sleeps are tuned for Hyprland to settle; shortening them races layout application.
func (i *Init) Execute() (string, error) {
	cfg := i.state.GetConfig()
	init := cfg.Init

	EnsureBG(&cfg.Background)
	fmt.Println("hyprd init: background ready")

	fullLocked := init.Lock && i.lock != nil
	if fullLocked {
		fmt.Println("hyprd init: full-locking")
		if _, err := i.lock.FullImmediateWait(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd init: full-lock: %v\n", err)
		}
		fmt.Println("hyprd init: unlocked")
	}

	if init.NetworkTimeout > 0 {
		if ok := i.waitNetwork(init.NetworkTimeout); ok {
			fmt.Println("hyprd init: network ready")
		}
	}

	layout := NewLayout(i.hypr, i.state)
	var initWS []int
	for _, s := range cfg.Sessions {
		if s.Init {
			initWS = append(initWS, s.Workspace)
		}
	}
	sort.Ints(initWS)

	for _, ws := range initWS {
		result, err := layout.Execute(strconv.Itoa(ws))
		if err != nil {
			fmt.Printf("hyprd init: ws%d: %v\n", ws, err)
		} else {
			fmt.Printf("hyprd init: %s\n", result)
		}
	}

	if !fullLocked {
		dispatchStartup(i.hypr, cfg.Bluetooth)
	}

	if init.Workspace > 0 {
		i.hypr.Dispatch(fmt.Sprintf("workspace %d", init.Workspace))
	}

	fmt.Println("hyprd init: complete")
	return "init: complete", nil
}

func (i *Init) waitNetwork(timeout int) bool {
	for range timeout {
		conn, err := net.DialTimeout("tcp", "1.1.1.1:53", time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(time.Second)
	}
	if i.notify != nil {
		i.notify("attention", "critical", "Network", fmt.Sprintf("Failed to connect after %ds", timeout))
	}
	return false
}
