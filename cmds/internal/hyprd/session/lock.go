package session

// lock.go implements pseudo-lock and full-lock lifecycles with restore of workspace, UI services, and audio.

import (
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const pseudoLockWorkspace = 6         // workspace reserved for the visual blackout
const fullLockGrace = 2 * time.Second // hyprlock cancel window
const fullLockDelay = time.Second     // let killall settle before manual hyprlock takes the display
const idleUnlockSuppress = 2 * time.Second

// pamLoadFlag is the runtime handshake consumed by `hyprd ssh pam-load` from pam_exec.
const pamLoadFlag = "hyprd-ssh-pam-load"

// Lock owns pseudo-lock and full-lock lifecycles: visual blackout, audio/notification pause, and restore.
//
// Serialized by mu; saved != nil means a lock is active, inFull means hyprlock is blocking.
type Lock struct {
	hypr            *hypr.Client
	state           *state.State
	mu              sync.Mutex
	saved           *lockState
	inFull          bool
	idleUnlockAfter time.Time
}

type lockState struct {
	workspace      int
	musicPlaying   bool
	restoreWidgets bool
}

func NewLock(h *hypr.Client, s *state.State) *Lock {
	return &Lock{hypr: h, state: s}
}

// Execute routes pseudo, idle pseudo-lock, unlock, idle unlock, and full lock.
func (l *Lock) Execute(arg string) (string, error) {
	switch strings.TrimSpace(arg) {
	case "", "pseudo":
		return l.Pseudo()
	case "idle":
		return l.Idle()
	case "-u", "unlock":
		return l.Unlock()
	case "idle-unlock":
		return l.IdleUnlock()
	case "full":
		return l.Full()
	default:
		return "", fmt.Errorf("usage: lock [pseudo|idle|unlock|idle-unlock|full]")
	}
}

// Pseudo enters pseudo-lock (blackout + submap); no-op if any lock is already active.
func (l *Lock) Pseudo() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.saved != nil {
		return "lock: already active", nil
	}
	l.saved = l.capture()
	l.enterBlackout()
	l.hypr.Dispatch("submap pseudolock")
	return "lock: pseudo", nil
}

// Idle enters pseudo-lock from hypridle. Entering blackout/submap can itself
// create a synthetic resume event, so idle resume briefly ignores unlocks.
func (l *Lock) Idle() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.saved != nil {
		return "lock: already active", nil
	}
	l.saved = l.capture()
	l.idleUnlockAfter = time.Now().Add(idleUnlockSuppress)
	l.enterBlackout()
	l.hypr.Dispatch("submap pseudolock")
	return "lock: idle", nil
}

// Unlock exits pseudo-lock; refuses while hyprlock is active.
func (l *Lock) Unlock() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.unlockLocked()
}

// IdleUnlock exits an idle pseudo-lock unless this is hypridle's synthetic
// resume caused by entering the pseudo-lock itself.
func (l *Lock) IdleUnlock() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.idleUnlockAfter.IsZero() && time.Now().Before(l.idleUnlockAfter) {
		return "lock: idle unlock suppressed", nil
	}
	return l.unlockLocked()
}

func (l *Lock) unlockLocked() (string, error) {
	if l.inFull {
		return "lock: hyprlock active", nil
	}
	l.hypr.Dispatch("submap reset")
	if l.saved == nil {
		l.idleUnlockAfter = time.Time{}
		return "lock: not active", nil
	}
	saved := l.saved
	l.saved = nil
	l.idleUnlockAfter = time.Time{}
	l.exitBlackout(saved, saved.musicPlaying)
	return "lock: unlocked", nil
}

// Full runs hyprlock asynchronously with pre/post blackout hooks.
func (l *Lock) Full() (string, error) {
	return l.full(fullLockDelay, fullLockGrace, true, true)
}

// FullImmediate runs hyprlock without startup delay or grace, for boot-time authentication.
func (l *Lock) FullImmediate() (string, error) {
	return l.full(0, 0, true, true)
}

// FullImmediateWait runs the boot-time full lock synchronously so startup work
// does not open private workspace layouts behind the lock screen.
func (l *Lock) FullImmediateWait() (string, error) {
	return l.fullBlocking(0, 0, true, false)
}

func (l *Lock) full(delay, grace time.Duration, loadSSH, restoreWidgets bool) (string, error) {
	saved, result, err := l.startFull(restoreWidgets)
	if saved == nil || err != nil {
		return result, err
	}

	go l.runHyprlock(saved, delay, grace, loadSSH)
	return "lock: full", nil
}

func (l *Lock) fullBlocking(delay, grace time.Duration, loadSSH, restoreWidgets bool) (string, error) {
	saved, result, err := l.startFull(restoreWidgets)
	if saved == nil || err != nil {
		return result, err
	}

	l.runHyprlock(saved, delay, grace, loadSSH)
	return "lock: full", nil
}

func (l *Lock) startFull(restoreWidgets bool) (*lockState, string, error) {
	l.mu.Lock()
	if l.inFull {
		l.mu.Unlock()
		return nil, "lock: hyprlock already running", nil
	}
	// Pseudo → full: drop submap, reuse existing blackout state.
	l.hypr.Dispatch("submap reset")
	if l.saved == nil {
		l.saved = l.capture()
		l.saved.restoreWidgets = restoreWidgets
		l.enterBlackout()
	}
	saved := l.saved
	l.inFull = true
	l.mu.Unlock()
	return saved, "lock: full", nil
}

func (l *Lock) runHyprlock(saved *lockState, delay, grace time.Duration, loadSSH bool) {
	if delay > 0 {
		time.Sleep(delay)
	}
	if loadSSH {
		flag := filepath.Join(runtimeDir(), pamLoadFlag)
		if f, err := os.Create(flag); err == nil {
			f.Close()
			defer os.Remove(flag)
		}
	}

	cmd := exec.Command("hyprlock", "--grace", strconv.Itoa(int(grace/time.Second)))
	if err := cmd.Start(); err == nil {
		cmd.Wait()
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.inFull = false
	if l.saved == nil {
		return
	}
	l.saved = nil
	resumeMusic := saved.musicPlaying
	l.exitBlackout(saved, resumeMusic)
}

func runtimeDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return fmt.Sprintf("/run/user/%d", os.Getuid())
}

// capture snapshots workspace and music state for later restore. Called with l.mu held.
func (l *Lock) capture() *lockState {
	ws := l.state.GetWorkspace()
	if ws <= 0 {
		if data, err := l.hypr.Request("j/activeworkspace"); err == nil {
			var active struct {
				ID int `json:"id"`
			}
			if json.Unmarshal(data, &active) == nil && active.ID > 0 {
				ws = active.ID
			}
		}
	}
	if ws <= 0 {
		ws = 1
	}
	return &lockState{
		workspace:      ws,
		musicPlaying:   playerctlStatus() == "Playing",
		restoreWidgets: true,
	}
}

func (l *Lock) enterBlackout() {
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", pseudoLockWorkspace))
	closeEwwWidgets()
	exec.Command("killall", "glava").Run()
	exec.Command("dunstctl", "close-all").Run()
	exec.Command("dunstctl", "set-paused", "true").Run()
	exec.Command("playerctl", "pause").Run()
}

func closeEwwWidgets() {
	if out, err := exec.Command("ewwd", "close").CombinedOutput(); err == nil {
		return
	} else {
		fmt.Fprintf(os.Stderr, "hyprd lock: ewwd close unavailable: %v%s; falling back to eww close-all\n", err, commandOutput(out))
	}
	if out, err := exec.Command("eww", "close-all").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: eww close-all: %v%s\n", err, commandOutput(out))
	}
}

func commandOutput(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}

// exitBlackout restores workspace, reopens eww/glava, reconnects bluetooth, and unpauses dunst.
func (l *Lock) exitBlackout(saved *lockState, resumeMusic bool) {
	cfg := l.state.GetConfig()
	if err := EnsureBG(&cfg.Background); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: background: %v\n", err)
	}
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", saved.workspace))

	dispatchStartup(l.hypr, cfg.Bluetooth)
	if saved.restoreWidgets {
		restoreEwwWidgets(false)
	}

	if resumeMusic {
		exec.Command("playerctl", "play").Run()
	}
	// Delay dunst unpause so queued notifications don't clobber eww startup.
	time.AfterFunc(time.Second, func() {
		exec.Command("dunstctl", "set-paused", "false").Run()
	})
}

// restoreEwwWidgets reopens widgets through ewwd once the daemon socket is ready.
func restoreEwwWidgets(reload bool) {
	if !waitEwwdReady(7 * time.Second) {
		if err := exec.Command("systemctl", "--user", "start", "ewwd.service").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd lock: start ewwd.service: %v\n", err)
		}
		if !waitEwwdReady(7 * time.Second) {
			fmt.Fprintln(os.Stderr, "hyprd lock: ewwd unavailable after service start")
			return
		}
	}
	if reload {
		startDetached("ewwd", "open")
	} else {
		startDetached("ewwd", "restore")
	}
}

func waitEwwdReady(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if exec.Command("ewwd", "status").Run() == nil {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func startDetached(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: start %s %s: %v\n", name, strings.Join(args, " "), err)
		return
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd lock: %s %s: %v\n", name, strings.Join(args, " "), err)
		}
	}()
}

func playerctlStatus() string {
	out, err := exec.Command("playerctl", "status").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
