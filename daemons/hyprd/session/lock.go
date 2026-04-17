package session

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

// Pseudo-lock parks the session on a workspace reserved for the visual blackout, so normal workspaces
// stay untouched while eww/glava are down.
const pseudoLockWorkspace = 6

// Grace window for hyprlock: if the user cancels before this elapses, the pre-lock music state resumes.
const fullLockGrace = 2 * time.Second

// Lock owns the pseudo-lock and full-lock lifecycles: visual blackout, audio/notification pause, and restore.
//
// Reentry is serialized by mu; saved != nil means a lock (pseudo or full) is active.
// inFull tracks whether a hyprlock process is currently blocking the screen so callers don't race it.
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

// Execute routes lock subcommands.
//
//	""|"pseudo"      → pseudo-lock (blackout + submap)
//	"-u"|"unlock"    → exit pseudo-lock
//	"full"           → run hyprlock with pre/post hooks
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

// Pseudo enters pseudo-lock: blackout + submap. No-op if any lock is already active.
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

// Unlock exits pseudo-lock. Refuses while hyprlock is up — hyprlock owns the unlock UX there.
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

// Full runs hyprlock with pre/post blackout hooks. Returns immediately; hyprlock and restore run in a
// goroutine so hyprd keeps serving commands while the screen is locked.
func (l *Lock) Full() (string, error) {
	l.mu.Lock()
	if l.inFull {
		l.mu.Unlock()
		return "lock: hyprlock already running", nil
	}
	// Clean transition from pseudo → full: drop submap, reuse blackout state if already applied.
	l.hypr.Dispatch("submap reset")
	if l.saved == nil {
		l.saved = l.capture()
		l.enterBlackout()
	}
	saved := l.saved
	l.inFull = true
	l.mu.Unlock()

	go l.runHyprlock(saved)
	return "lock: full", nil
}

func (l *Lock) runHyprlock(saved *lockState) {
	// Match the pre-migration script: give killall a beat before handing the display to hyprlock.
	time.Sleep(time.Second)

	start := time.Now()
	exec.Command("hyprlock", "--grace", strconv.Itoa(int(fullLockGrace/time.Second))).Run()
	elapsed := time.Since(start)

	l.mu.Lock()
	defer l.mu.Unlock()
	l.inFull = false
	if l.saved == nil {
		return
	}
	l.saved = nil
	// Only resume music if the user cancelled inside the grace window; a true idle-lock should wake silent.
	resumeMusic := saved.musicPlaying && elapsed <= fullLockGrace
	l.exitBlackout(saved, resumeMusic)
}

// capture snapshots the current workspace and music state so Unlock/Full can restore them.
// Called with l.mu held.
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

// enterBlackout hides visual surfaces, silences audio, and pauses notifications.
// Called with l.mu held.
func (l *Lock) enterBlackout() {
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", pseudoLockWorkspace))
	exec.Command("killall", "-9", "eww").Run()
	exec.Command("killall", "glava").Run()
	exec.Command("dunstctl", "close-all").Run()
	exec.Command("dunstctl", "set-paused", "true").Run()
	exec.Command("playerctl", "pause").Run()
}

// exitBlackout reverses enterBlackout: restores workspace, respawns eww + init.execs, unpauses dunst and
// optionally resumes music. Re-runs init.execs for the glava/bluetooth restart so those commands stay
// defined in one place.
// Called with l.mu held.
func (l *Lock) exitBlackout(saved *lockState, resumeMusic bool) {
	l.hypr.Dispatch(fmt.Sprintf("workspace %d", saved.workspace))

	cfg := l.state.GetConfig()
	for _, cmd := range cfg.Init.Execs {
		l.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
	}
	exec.Command("ewwd", "open").Start()

	if resumeMusic {
		exec.Command("playerctl", "play").Run()
	}
	// Unpause dunst after eww is back so a flood of queued notifications doesn't clobber startup.
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
