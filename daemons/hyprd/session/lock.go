package session

// lock.go implements pseudo-lock and full-lock lifecycles with restore of workspace, UI services, and audio.

import (
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
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
const fullLockGrace = 2 * time.Second // hyprlock cancel window that resumes music
const fullLockDelay = time.Second     // let killall settle before manual hyprlock takes the display
const pamLoadFlag = "hyprd-ssh-pam-load"

// Lock owns pseudo-lock and full-lock lifecycles: visual blackout, audio/notification pause, and restore.
//
// Serialized by mu; saved != nil means a lock is active, inFull means hyprlock is blocking.
type Lock struct {
	hypr   *hypr.Client
	state  *state.State
	mu     sync.Mutex
	saved  *lockState
	inFull bool
}

type lockState struct {
	workspace    int
	musicPlaying bool
}

func NewLock(h *hypr.Client, s *state.State) *Lock {
	return &Lock{hypr: h, state: s}
}

// Execute routes: "pseudo" (default), "unlock"/"-u", or "full".
func (l *Lock) Execute(arg string) (string, error) {
	switch strings.TrimSpace(arg) {
	case "", "pseudo":
		return l.Pseudo()
	case "-u", "unlock":
		return l.Unlock()
	case "full":
		return l.Full()
	default:
		return "", fmt.Errorf("usage: lock [pseudo|unlock|full]")
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

// Unlock exits pseudo-lock; refuses while hyprlock is active.
func (l *Lock) Unlock() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFull {
		return "lock: hyprlock active", nil
	}
	l.hypr.Dispatch("submap reset")
	if l.saved == nil {
		return "lock: not active", nil
	}
	saved := l.saved
	l.saved = nil
	l.exitBlackout(saved, saved.musicPlaying)
	return "lock: unlocked", nil
}

// Full runs hyprlock asynchronously with pre/post blackout hooks.
func (l *Lock) Full() (string, error) {
	// DEBUG: temporarily load SSH/keyring on plain Full() too, for testing without reboot.
	return l.full(fullLockDelay, fullLockGrace, true)
}

// FullImmediate runs hyprlock without startup delay or grace, for boot-time authentication.
func (l *Lock) FullImmediate() (string, error) {
	return l.full(0, 0, true)
}

func (l *Lock) full(delay, grace time.Duration, loadSSH bool) (string, error) {
	l.mu.Lock()
	if l.inFull {
		l.mu.Unlock()
		return "lock: hyprlock already running", nil
	}
	// Pseudo → full: drop submap, reuse existing blackout state.
	l.hypr.Dispatch("submap reset")
	if l.saved == nil {
		l.saved = l.capture()
		l.enterBlackout()
	}
	saved := l.saved
	l.inFull = true
	l.mu.Unlock()

	go l.runHyprlock(saved, delay, grace, loadSSH)
	return "lock: full", nil
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

	start := time.Now()
	exec.Command("hyprlock", "--grace", strconv.Itoa(int(grace/time.Second))).Run()
	elapsed := time.Since(start)

	l.mu.Lock()
	defer l.mu.Unlock()
	l.inFull = false
	if l.saved == nil {
		return
	}
	l.saved = nil
	// Only resume music if cancelled inside the grace window; idle-lock wakes silent.
	resumeMusic := saved.musicPlaying && grace > 0 && elapsed <= grace
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
		workspace:    ws,
		musicPlaying: playerctlStatus() == "Playing",
	}
}

func (l *Lock) enterBlackout() {
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", pseudoLockWorkspace))
	exec.Command("eww", "close-all").Run()
	exec.Command("killall", "glava").Run()
	exec.Command("dunstctl", "close-all").Run()
	exec.Command("dunstctl", "set-paused", "true").Run()
	exec.Command("playerctl", "pause").Run()
}

// exitBlackout restores workspace, reopens eww/glava, reconnects bluetooth, and unpauses dunst.
func (l *Lock) exitBlackout(saved *lockState, resumeMusic bool) {
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", saved.workspace))

	cfg := l.state.GetConfig()
	dispatchStartup(l.hypr, cfg.Bluetooth)
	// Reopen via running ewwd; if gone, respawn (fresh daemon auto-opens windows).
	if exec.Command("ewwd", "status").Run() == nil {
		exec.Command("ewwd", "open").Start()
	} else {
		cmd := exec.Command("setsid", "ewwd")
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Start()
	}

	if resumeMusic {
		exec.Command("playerctl", "play").Run()
	}
	// Delay dunst unpause so queued notifications don't clobber eww startup.
	time.AfterFunc(time.Second, func() {
		exec.Command("dunstctl", "set-paused", "false").Run()
	})
}

func playerctlStatus() string {
	out, err := exec.Command("playerctl", "status").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
