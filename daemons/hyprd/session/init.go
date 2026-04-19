// Package session orchestrates workspace sessions, lock flows, and kitty tab automation.
//
// It:
//  1. Runs startup initialization for configured workspaces.
//  2. Launches and arranges per-workspace session layouts.
//  3. Provides lock, picker, and tab-control helpers used by daemon commands.
package session

// init.go executes boot-time session initialization, including wallpaper, network wait, layout open, and pseudo-lock.

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

// NotifyFunc delivers a notification to the user, injected to break a cycle with the notify package.
type NotifyFunc func(app, urgency, title, body string)

// Init drives first-boot session setup: background, network wait, per-workspace layouts, and pseudo-lock.
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

// Execute runs the full init sequence: background, network, workspace layouts, execs, and pseudo-lock.
//
// Inter-dispatch sleeps are tuned for Hyprland to settle; shortening them races layout application.
func (i *Init) Execute() (string, error) {
	cfg := i.state.GetConfig()
	init := cfg.Init

	EnsureBG(&cfg.Background)
	fmt.Println("hyprd init: background ready")

	if init.NetworkTimeout > 0 {
		if ok := i.waitNetwork(init.NetworkTimeout); ok {
			fmt.Println("hyprd init: network ready")
		}
	}

	layout := NewLayout(i.hypr, i.state)
	var initWS []int
	for ws, as := range cfg.ActiveSessions {
		if as.Init {
			initWS = append(initWS, ws)
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

	for _, cmd := range init.Execs {
		i.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(200 * time.Millisecond)
	}

	if init.Workspace > 0 {
		i.hypr.Dispatch(fmt.Sprintf("workspace %d", init.Workspace))
	}

	if init.Lock && i.lock != nil {
		if init.LockDelay > 0 {
			fmt.Printf("hyprd init: waiting %s before pseudo-lock\n", init.LockDelay)
			time.Sleep(init.LockDelay)
		}
		fmt.Println("hyprd init: pseudo-locking")
		if _, err := i.lock.Pseudo(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd init: pseudo-lock: %v\n", err)
		}
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
