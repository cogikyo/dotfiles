package commands

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"dotfiles/daemons/hyprd/hypr"
)

// NotifyFunc sends a notification with the given app, urgency, title, and body.
type NotifyFunc func(app, urgency, title, body string)

// Init runs the one-time boot sequence: wallpaper, network wait, sessions, app launches,
// greeting notification, workspace switch, and optional screen lock.
type Init struct {
	hypr   *hypr.Client
	state  StateManager
	notify NotifyFunc
}

// NewInit creates an Init command handler.
func NewInit(h *hypr.Client, s StateManager) *Init {
	return &Init{hypr: h, state: s}
}

// SetNotify sets the notification callback for init events.
func (i *Init) SetNotify(fn NotifyFunc) {
	i.notify = fn
}

// Execute runs the full init sequence. Intended to be called once on fresh boot
// (no existing windows) or manually via `hyprd init`.
func (i *Init) Execute() (string, error) {
	cfg := i.state.GetConfig()
	init := cfg.Init

	// 1. Ensure wallpaper is running
	EnsureBG(&cfg.Background)
	fmt.Println("hyprd init: background ready")

	// 2. Wait for network connectivity
	if init.NetworkTimeout > 0 {
		if ok := i.waitNetwork(init.NetworkTimeout); ok {
			fmt.Println("hyprd init: network ready")
		}
	}

	// 3. Open layout sessions
	layout := NewLayout(i.hypr, i.state)
	for _, name := range init.Sessions {
		result, err := layout.Execute(name)
		if err != nil {
			fmt.Printf("hyprd init: session %s: %v\n", name, err)
		} else {
			fmt.Printf("hyprd init: %s\n", result)
		}
	}

	// 4. Launch apps/execs via hyprctl dispatch (inherits Hyprland env, applies window rules)
	for _, cmd := range init.Execs {
		i.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(200 * time.Millisecond)
	}

	// 5. Switch to target workspace
	if init.Workspace > 0 {
		i.hypr.Dispatch(fmt.Sprintf("workspace %d", init.Workspace))
	}

	// 6. Greeting notification
	if init.Greeting != "" && i.notify != nil {
		hostname, _ := os.Hostname()
		user := os.Getenv("USER")
		i.notify("attention", "low", fmt.Sprintf("%s@%s", user, hostname), init.Greeting)
	}

	// 7. Lock screen (blocks until unlocked)
	if init.Lock {
		fmt.Println("hyprd init: locking screen")
		exec.Command("hyprlock").Run()
	}

	fmt.Println("hyprd init: complete")
	return "init: complete", nil
}

// waitNetwork polls for TCP connectivity to 1.1.1.1:53.
// Returns true if connected, false if timed out.
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
