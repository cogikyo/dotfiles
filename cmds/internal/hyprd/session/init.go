// Package session orchestrates workspace sessions, lock flows, and kitty tab automation.
//
// Responsibilities:
// - Run startup initialization for configured workspaces.
// - Launch and arrange per-workspace session layouts.
// - Provide lock, picker, and tab-control helpers used by daemon commands.
package session

// init.go executes boot-time session initialization, including wallpaper, optional early lock, network wait, and layout open.

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/browser"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

var startupExecs = []string{
	"glava -e bars_rc.glsl",
	"glava -e bars_r_rc.glsl",
	"glava -e radial_rc.glsl",
	"spotify-launcher",
}

const (
	bluetoothctlPath         = "/usr/bin/bluetoothctl"
	bluetoothConnectAttempts = 8
	bluetoothRetryDelay      = 2 * time.Second
	bluetoothTryTimeout      = 8 * time.Second
)

// dispatchStartup runs hardcoded startup commands and optionally connects bluetooth.
func dispatchStartup(h hyprDispatcher, bt config.BluetoothConfig) {
	for _, cmd := range startupExecs {
		h.Dispatch(fmt.Sprintf("exec %s", cmd))
	}
	if bt.Enabled && bt.Device != "" {
		connectBluetooth(bt.Device)
	}
}

func connectBluetooth(device string) {
	go func() {
		var lastErr error
		var lastOut []byte
		for attempt := 1; attempt <= bluetoothConnectAttempts; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), bluetoothTryTimeout)
			cmd := exec.CommandContext(ctx, bluetoothctlPath, "connect", device)
			out, err := cmd.CombinedOutput()
			cancel()
			if err == nil {
				if attempt > 1 {
					fmt.Printf("hyprd bluetooth: connected %s after %d attempts\n", device, attempt)
				}
				return
			}
			lastErr = err
			lastOut = out
			if attempt < bluetoothConnectAttempts {
				time.Sleep(bluetoothRetryDelay)
			}
		}
		if len(lastOut) > 0 {
			fmt.Fprintf(os.Stderr, "hyprd bluetooth: connect %s failed after %d attempts: %v: %s\n", device, bluetoothConnectAttempts, lastErr, lastOut)
			return
		}
		fmt.Fprintf(os.Stderr, "hyprd bluetooth: connect %s failed after %d attempts: %v\n", device, bluetoothConnectAttempts, lastErr)
	}()
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

	if err := EnsureBGBoot(&cfg.Background); err != nil {
		return "", fmt.Errorf("background ready before lock: %w", err)
	}
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

	restoreEwwWidgets(true)

	layout := NewLayout(i.hypr, i.state)
	var initSessions []config.Session
	for _, s := range cfg.Sessions {
		if s.Init {
			initSessions = append(initSessions, s)
		}
	}
	sort.Slice(initSessions, func(a, b int) bool {
		return initSessions[a].Workspace < initSessions[b].Workspace
	})

	if err := layout.restoreInitBrowsers(initSessions); err != nil {
		return "", fmt.Errorf("browser restore: %w", err)
	}

	for _, session := range initSessions {
		result, err := layout.openSession(session)
		if err != nil {
			fmt.Printf("hyprd init: ws%d: %v\n", session.Workspace, err)
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

func (l *Layout) restoreInitBrowsers(sessions []config.Session) error {
	var browserSessions []config.Session
	b := browser.NewBrowser(l.hypr, l.state)
	for _, session := range sessions {
		if session.Browser.Snapshot != "coms" || !b.UsesExactRestore(session.Browser) {
			continue
		}
		browserSessions = append(browserSessions, session)
	}
	if len(browserSessions) == 0 {
		return nil
	}

	for _, session := range browserSessions {
		if _, err := b.RestoreConfiguredSnapshot(session.Browser, false); err != nil {
			return err
		}
	}
	l.markBatchRestoredBrowsers(browserSessions)
	for _, session := range browserSessions {
		if err := l.claimBrowserWindow(b, session); err != nil {
			return fmt.Errorf("claim browser window for %s: %w", session.Name, err)
		}
	}
	return nil
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
