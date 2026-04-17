// Package session manages Hyprland workspace session lifecycles.
//
// Handles per-workspace initialization, wallpaper, kitty tab restoration, and layout application. Sessions are
// defined in config and drive the state of a workspace when first activated.
package session

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

// NotifyFunc delivers a notification to the user.
//
// Injected to avoid importing the notify package here, which would create a cycle.
type NotifyFunc func(app, urgency, title, body string)

// Init drives first-boot session setup: background, network wait, per-workspace layouts, post-init execs, and lock.
type Init struct {
	hypr   *hypr.Client
	state  *state.State
	notify NotifyFunc
	lock   *Lock
}

// NewInit constructs an Init bound to the given hypr client and state.
//
// Call SetNotify before Execute to enable user-visible notifications on network failure.
// Call SetLock to share the daemon's Lock controller so init can drop into pseudo-lock.
func NewInit(h *hypr.Client, s *state.State) *Init {
	return &Init{hypr: h, state: s}
}

func (i *Init) SetLock(l *Lock) {
	i.lock = l
}

func (i *Init) SetNotify(fn NotifyFunc) {
	i.notify = fn
}

// Execute runs the full init sequence in order:
//   - ensure background wallpaper is alive
//   - wait for network (bounded by config.NetworkTimeout)
//   - open each workspace marked Init in sorted order
//   - run post-init exec commands
//   - focus the configured landing workspace
//   - optionally lock the screen after LockDelay
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

	if init.Lock {
		if init.LockDelay > 0 {
			fmt.Printf("hyprd init: waiting %s before lock\n", init.LockDelay)
			time.Sleep(init.LockDelay)
		}
		fmt.Println("hyprd init: locking screen")
		exec.Command("hyprlock").Run()
		exec.Command("ewwd", "open").Run()
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
