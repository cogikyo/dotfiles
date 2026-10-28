// Package session manages desktop session startup, workspace layouts, lock flows, wallpaper, sharing, and selection.
package session

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/browser"
	"dotfiles/cmds/internal/hyprd/detach"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/notify"
	"dotfiles/cmds/internal/hyprd/state"
)

var startupExecs = []string{
	"spotify-launcher",
}

const (
	bluetoothctlPath         = "/usr/bin/bluetoothctl"
	bluetoothConnectAttempts = 8
	bluetoothRetryDelay      = 2 * time.Second
	bluetoothTryTimeout      = 8 * time.Second
)

// dispatchStartup runs hardcoded startup commands and optionally connects bluetooth.
func dispatchStartup(h *hypr.Client, bt config.BluetoothConfig) {
	for _, cmd := range startupExecs {
		if err := h.Exec(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd startup: exec %q: %v\n", cmd, err)
		}
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

type NotifyFunc func(app, urgency, title, body string)

// Init manages session startup and delays widgets, browsers, and sessions until the full lock is released.
type Init struct {
	hypr   *hypr.Client
	state  *state.State
	lock   *Lock
	share  *Share
	notify NotifyFunc
}

func NewInit(h *hypr.Client, s *state.State, lock *Lock, share *Share) *Init {
	return &Init{hypr: h, state: s, lock: lock, share: share}
}

func (i *Init) SetNotify(fn NotifyFunc) {
	i.notify = fn
}

// Execute applies the background, optionally waits for the network, then waits for full lock release before restoring widgets, browsers, and sessions.
// It then runs startup commands and selects the initial workspace.
//
// Inter-dispatch sleeps are tuned for Hyprland to settle; shortening them races layout application.
func (i *Init) Execute() (string, error) {
	cfg := i.state.GetConfig()
	init := cfg.Init

	if err := EnsureBG(&cfg.Background); err != nil {
		return "", fmt.Errorf("background: %w", err)
	}
	fmt.Println("hyprd init: background ready")

	next := setupNext()
	online := true
	if init.NetworkTimeout > 0 {
		online = i.waitNetwork(init.NetworkTimeout)
		if online {
			fmt.Println("hyprd init: network ready")
		} else if next == "" && i.notify != nil {
			i.notify("attention", "critical", "Network", fmt.Sprintf("Failed to connect after %ds", init.NetworkTimeout))
		}
	}

	if i.lock.Locked() {
		fmt.Println("hyprd init: waiting for unlock")
		i.lock.Wait()
	}

	if next != "" {
		i.promptSetup(next, online)
	}

	if _, _, err := i.share.SyncDisplay(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd init: display: %v\n", err)
	}
	restoreEwwWidgets(true)

	layout := NewLayout(i.hypr, i.state)
	initSessions := cfg.Sessions.InitSessions()

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

	SyncGLava(i.state, i.lock.Locked())
	dispatchStartup(i.hypr, cfg.Bluetooth)

	if init.Workspace > 0 {
		if err := i.hypr.FocusWorkspace(init.Workspace); err != nil {
			return "", fmt.Errorf("focus initial workspace %d: %w", init.Workspace, err)
		}
	}

	fmt.Println("hyprd init: complete")
	return "init: complete", nil
}

func (l *Layout) restoreInitBrowsers(sessions []config.Session) error {
	var browserSessions []config.Session
	b := browser.NewBrowser(l.hypr, l.state)
	for _, session := range sessions {
		if session.Browser.Snapshot == "" || !b.UsesExactRestore(session.Browser) {
			continue
		}
		browserSessions = append(browserSessions, session)
	}
	if len(browserSessions) == 0 {
		return nil
	}

	configs := make([]config.BrowserConfig, 0, len(browserSessions))
	for _, session := range browserSessions {
		configs = append(configs, session.Browser)
	}
	if _, err := b.RestoreConfiguredSnapshots(configs, false); err != nil {
		return err
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
	return false
}

func setupNext() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "hyprd init: setup marker: %v\n", err)
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	data, err := os.ReadFile(filepath.Join(dir, "dctl", "next"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "hyprd init: setup marker: %v\n", err)
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (i *Init) promptSetup(next string, online bool) {
	body := next
	if !online {
		body = "Plug in Ethernet or join Wi-Fi: nmcli --ask device wifi connect <SSID>\n" + next
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd init: setup prompt: %v\n", err)
		return
	}
	cmd := "kitty --title terminalfloat --hold -e " + filepath.Join(home, ".local", "bin", "dctl") + " setup"
	notifier := notify.NewNotifier(i.hypr, i.state, i.state.GetConfig(), nil)
	args := append([]string{"hl.dsp.exec_cmd(" + hypr.LuaQuote(cmd) + ")"},
		notifier.PromptArgs("dctl", "critical", "Finish setting up", html.EscapeString(body), "Run setup")...)
	if err := detach.Unit(context.Background(), "setup-prompt", setupPrompt, args...); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd init: setup prompt: %v\n", err)
	}
}

const setupPrompt = `dispatch=$1
shift
[ "$(dunstify "$@")" = default ] || exit 0
exec hyprctl dispatch "$dispatch"`
