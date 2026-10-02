package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

const (
	privacyWorkspace = 6
	pseudoSubmap     = "pseudolock"
)

const (
	quickExit     = 3 * time.Second
	quickExits    = 3
	relaunchDelay = 2 * time.Second
	lockPoll      = 50 * time.Millisecond
	lockWait      = 10 * time.Second
)

// Lock owns privacy-screen and full-lock lifecycles: visual blackout, audio/notification pause, and restore.
//
// mu is held for each whole transition, restore included.
// saved != nil means the privacy screen or a full lock is active; inFull means hyprlock is supervised and is cleared only when hyprlock exits 0.
type Lock struct {
	hypr       *hypr.Client
	state      *state.State
	hyprlock   func() error
	running    func() bool
	endSession func() error
	mu         sync.Mutex
	saved      *lockState
	inFull     bool
}

type lockState struct {
	workspace    int
	musicPlaying bool
}

func NewLock(h *hypr.Client, s *state.State) *Lock {
	return &Lock{hypr: h, state: s, hyprlock: execHyprlock, running: hyprlockRunning, endSession: endSession}
}

// Execute routes privacy, idle privacy, unlock, and full lock.
func (l *Lock) Execute(arg string) (string, error) {
	switch strings.TrimSpace(arg) {
	case "pseudo":
		return l.enterPrivacy("pseudo")
	case "idle":
		return l.enterPrivacy("idle")
	case "-u", "unlock":
		return l.Unlock()
	case "full":
		return l.Full()
	default:
		return "", fmt.Errorf("usage: lock [pseudo|idle|unlock|full]")
	}
}

func (l *Lock) enterPrivacy(kind string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.saved != nil || l.inFull {
		return "lock: already active", nil
	}

	saved := l.capture()
	if err := l.hypr.FocusWorkspace(privacyWorkspace); err != nil {
		return "", fmt.Errorf("lock: switch to workspace %d: %w", privacyWorkspace, err)
	}
	if err := l.hypr.Submap(pseudoSubmap); err != nil {
		rollbackErr := errors.Join(
			l.hypr.Submap("reset"),
			l.hypr.FocusWorkspace(saved.workspace),
		)
		if rollbackErr != nil {
			return "", fmt.Errorf("lock: enter %s submap: %w; rollback: %w", pseudoSubmap, err, rollbackErr)
		}
		return "", fmt.Errorf("lock: enter %s submap: %w", pseudoSubmap, err)
	}

	l.saved = saved
	enterBlackout(saved)
	return "lock: " + kind, nil
}

// Unlock exits the privacy screen; refuses while hyprlock is active.
func (l *Lock) Unlock() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFull {
		return "lock: hyprlock active", nil
	}
	if err := l.hypr.Submap("reset"); err != nil {
		return "", fmt.Errorf("lock: reset submap: %w", err)
	}
	if l.saved == nil {
		return "lock: not active", nil
	}
	saved := l.saved
	l.saved = nil

	if err := l.exitBlackout(saved); err != nil {
		return "lock: unlocked", err
	}
	return "lock: unlocked", nil
}

// Locked reports whether a full lock is active.
func (l *Lock) Locked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inFull
}

// Full starts supervising hyprlock in the background.
func (l *Lock) Full() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFull {
		return "lock: hyprlock already running", nil
	}
	l.inFull = true
	go l.hold()
	return "lock: full", nil
}

// Adopt takes over a hyprlock left by a previous daemon: the full lock stays active until it exits, then hyprlock is relaunched.
func (l *Lock) Adopt() error {
	foreign := l.running()
	if !foreign {
		locked, err := l.hypr.Locked()
		if err != nil {
			return fmt.Errorf("lock: query session lock: %w", err)
		}
		if !locked {
			return nil
		}
		fmt.Fprintln(os.Stderr, "hyprd lock: compositor locked without hyprlock; relaunching hyprlock")
	}
	l.mu.Lock()
	l.inFull = true
	l.mu.Unlock()

	go func() {
		if foreign {
			l.awaitForeign()
		}
		l.hold()
	}()
	return nil
}

func (l *Lock) awaitForeign() {
	for l.running() {
		time.Sleep(relaunchDelay)
	}
}

func (l *Lock) hold() {
	quick := 0
	for launch := 0; ; launch++ {
		if launch > 1 {
			time.Sleep(relaunchDelay)
		}
		l.submap(pseudoSubmap)
		started := time.Now()
		exited := make(chan error, 1)
		go func() { exited <- l.hyprlock() }()
		if launch == 0 {
			l.cover()
		}

		err := l.awaitLock(exited)
		if err == nil {
			l.release()
			return
		}
		if l.running() {
			fmt.Fprintf(os.Stderr, "hyprd lock: hyprlock exited: %v; another hyprlock holds the lock, waiting for it\n", err)
			l.awaitForeign()
			quick = 0
			continue
		}
		fmt.Fprintf(os.Stderr, "hyprd lock: hyprlock exited: %v; relaunching\n", err)
		if time.Since(started) >= quickExit {
			quick = 0
			continue
		}
		quick++
		if quick < quickExits {
			continue
		}
		quick = 0
		if err := l.endSession(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd lock: hyprlock cannot hold the lock and the session did not end: %v\n", err)
			continue
		}
		fmt.Fprintln(os.Stderr, "hyprd lock: hyprlock cannot hold the lock; ending the session")
		return
	}
}

func (l *Lock) awaitLock(exited <-chan error) error {
	poll := time.NewTicker(lockPoll)
	defer poll.Stop()
	timeout := time.After(lockWait)
	for {
		select {
		case err := <-exited:
			return err
		case <-timeout:
			fmt.Fprintf(os.Stderr, "hyprd lock: compositor not locked after %v; keeping the %s submap\n", lockWait, pseudoSubmap)
			return <-exited
		case <-poll.C:
			if l.compositorLocked() {
				l.submap("reset")
				return <-exited
			}
		}
	}
}

func (l *Lock) compositorLocked() bool {
	locked, err := l.hypr.Locked()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: query session lock: %v\n", err)
		return false
	}
	return locked
}

func (l *Lock) active() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.saved != nil || l.inFull
}

func (l *Lock) resetSubmap() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.saved != nil || l.inFull {
		return nil
	}
	return l.hypr.Submap("reset")
}

func (l *Lock) submap(name string) {
	if err := l.hypr.Submap(name); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: submap %s: %v\n", name, err)
	}
}

func (l *Lock) cover() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.saved != nil {
		return
	}
	l.saved = l.capture()
	if err := l.hypr.FocusWorkspace(privacyWorkspace); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: switch to workspace %d: %v\n", privacyWorkspace, err)
	}
	enterBlackout(l.saved)
}

func (l *Lock) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	saved := l.saved
	l.saved = nil
	l.inFull = false
	l.submap("reset")
	if err := l.exitBlackout(saved); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: unlock after hyprlock: %v\n", err)
	}
}

func hyprlockCommand() *exec.Cmd {
	cmd := exec.Command("hyprlock", "--grace", "0")
	cmd.Stderr = os.Stderr
	return cmd
}

func execHyprlock() error {
	return hyprlockCommand().Run()
}

func hyprlockRunning() bool {
	return exec.Command("pidof", "-q", "hyprlock").Run() == nil
}

func endSession() error {
	err := terminateSession()
	if err == nil {
		return nil
	}
	if exitErr := exec.Command("hyprctl", "dispatch", "hl.dsp.exit()").Run(); exitErr != nil {
		return errors.Join(err, fmt.Errorf("hyprctl exit: %w", exitErr))
	}
	return nil
}

func terminateSession() error {
	out, err := exec.Command("loginctl", "show-user", strconv.Itoa(os.Getuid()), "-p", "Display", "--value").Output()
	if err != nil {
		return fmt.Errorf("find graphical session: %w", err)
	}
	session := strings.TrimSpace(string(out))
	if session == "" {
		return errors.New("no graphical session")
	}
	if err := exec.Command("loginctl", "terminate-session", session).Run(); err != nil {
		return fmt.Errorf("terminate session %s: %w", session, err)
	}
	return nil
}

// capture snapshots workspace for later restore. Called with l.mu held.
func (l *Lock) capture() *lockState {
	ws := l.state.GetWorkspace()
	if ws <= 0 {
		if activeWS, err := l.hypr.ActiveWorkspace(); err == nil && activeWS > 0 {
			ws = activeWS
		}
	}
	if ws <= 0 {
		ws = 1
	}
	return &lockState{workspace: ws}
}

func enterBlackout(saved *lockState) {
	saved.musicPlaying = playerctlStatus() == "Playing"
	exec.Command("killall", "glava").Run()
	exec.Command("dunstctl", "close-all").Run()
	exec.Command("dunstctl", "set-paused", "true").Run()
	exec.Command("playerctl", "--player=spotify", "pause").Run()
	closeEwwWidgets()
}

func closeEwwWidgets() {
	out, err := exec.Command("ewwd", "close").CombinedOutput()
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "hyprd lock: ewwd close unavailable: %v: %s\n", err, strings.TrimSpace(string(out)))
	if exec.Command("eww", "ping").Run() != nil {
		return
	}
	if out, err := exec.Command("eww", "close-all").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: eww close-all: %v: %s\n", err, strings.TrimSpace(string(out)))
	}
}

// exitBlackout restores workspace, reopens eww/glava, reconnects bluetooth, and unpauses dunst. Called with l.mu held.
func (l *Lock) exitBlackout(saved *lockState) error {
	cfg := l.state.GetConfig()
	if err := l.hypr.FocusWorkspace(saved.workspace); err != nil {
		return fmt.Errorf("lock: restore workspace %d: %w", saved.workspace, err)
	}
	if err := EnsureBG(&cfg.Background); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: background: %v\n", err)
	}

	dispatchStartup(l.hypr, cfg.Bluetooth)
	restoreEwwWidgets(false)

	if saved.musicPlaying {
		exec.Command("playerctl", "play").Run()
	}
	exec.Command("dunstctl", "set-paused", "false").Run()
	return nil
}

// restoreEwwWidgets reopens widgets through ewwd, starting ewwd.service and waiting up to a second when the daemon is down.
func restoreEwwWidgets(reload bool) {
	action := "restore"
	if reload {
		action = "open"
	}

	if !waitEwwdReady(0) {
		if err := exec.Command("systemctl", "--user", "--no-block", "start", "ewwd.service").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd lock: start ewwd.service: %v\n", err)
		}
		if !waitEwwdReady(time.Second) {
			fmt.Fprintln(os.Stderr, "hyprd lock: ewwd unavailable after service start")
			return
		}
	}
	startDetached("ewwd", action)
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
