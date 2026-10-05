package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

const (
	coverWorkspace = 6
	barrierSubmap  = "lockbarrier"
	lockEnv        = "LOCK_MODE=lock"
	intentPending  = "pending"
	intentAcquired = "acquired"
	secureMarker   = "lock-secure: acquired"
	abortMarker    = "Aborting lock."
	logTail        = 4096
)

const (
	quickExit     = 3 * time.Second
	quickExits    = 3
	relaunchDelay = 2 * time.Second
	lockPoll      = 50 * time.Millisecond
	lockWait      = 5 * time.Second
	releasePoll   = time.Second
	releaseGrace  = 2 * time.Second
	coverTimeout  = 2 * time.Second
)

// Lock supervises the Quickshell session lock and restores session resources after release.
type Lock struct {
	hypr        *hypr.Client
	state       *state.State
	launch      func(context.Context, io.Writer) error
	running     func() bool
	stopForeign func()
	endSession  func() error
	notify      NotifyFunc
	intent      string
	mu          sync.Mutex
	saved       *lockState
	inFull      bool
	restarting  bool
	deferred    bool
}

type lockState struct {
	workspace    int
	musicPlaying bool
}

func NewLock(h *hypr.Client, s *state.State) *Lock {
	return &Lock{
		hypr: h, state: s,
		launch: execLock, running: lockRunning, stopForeign: stopLocks, endSession: endSession,
		intent: intentPath(),
	}
}

func (l *Lock) SetNotify(fn NotifyFunc) {
	l.notify = fn
}

func (l *Lock) Execute(arg string) (string, error) {
	if strings.TrimSpace(arg) != "full" {
		return "", errors.New("usage: lock full")
	}
	return l.Full()
}

// Locked reports whether a full lock is active.
func (l *Lock) Locked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inFull
}

// Full starts lock supervision asynchronously; returning does not mean the compositor is locked.
func (l *Lock) Full() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFull {
		return "lock: already running", nil
	}
	l.record(intentPending)
	if l.restarting {
		l.deferred = true
		return "lock: deferred to the restarting daemon", nil
	}
	l.inFull = true
	go l.hold(false)
	return "lock: full", nil
}

func (l *Lock) BeginRestart() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFull {
		return false
	}
	l.restarting = true
	return true
}

func (l *Lock) CancelRestart() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.restarting = false
	if l.deferred {
		l.deferred = false
		l.inFull = true
		go l.hold(false)
	}
}

// Adopt resumes supervision asynchronously, using compositor state because a surviving client's exit status is unavailable.
// It returns an error only if no lock client or pending lock exists and the compositor's lock state cannot be read.
func (l *Lock) Adopt() error {
	intent := l.readIntent()
	foreign := l.running()
	locked, err := l.hypr.Locked()
	switch {
	case foreign:
	case err != nil && intent == "":
		return fmt.Errorf("lock: query session lock: %w", err)
	case err != nil:
		fmt.Fprintf(os.Stderr, "hyprd lock: query session lock: %v; relaunching the pending lock\n", err)
	case locked:
		fmt.Fprintln(os.Stderr, "hyprd lock: compositor locked without a lock client; relaunching")
	case intent == intentAcquired:
		l.mu.Lock()
		l.inFull = true
		l.mu.Unlock()
		go l.release()
		return nil
	case intent != "":
		fmt.Fprintln(os.Stderr, "hyprd lock: pending lock was never acquired; relaunching")
	default:
		return nil
	}
	l.mu.Lock()
	l.inFull = true
	l.mu.Unlock()
	go l.hold(intent == intentAcquired)
	return nil
}

func (l *Lock) superviseForeign(acquired bool) bool {
	deadline := time.Now().Add(lockWait)
	var unlockedAt time.Time
	for l.running() {
		locked, err := l.hypr.Locked()
		switch {
		case err != nil:
		case locked:
			if !acquired {
				acquired = true
				l.record(intentAcquired)
			}
			unlockedAt = time.Time{}
		case acquired && unlockedAt.IsZero():
			unlockedAt = time.Now()
		case acquired && time.Since(unlockedAt) >= releaseGrace:
			fmt.Fprintf(os.Stderr, "hyprd lock: running lock client still alive %v after the compositor unlocked; stopping it\n", releaseGrace)
			l.stopForeign()
			return true
		case !acquired && !time.Now().Before(deadline):
			fmt.Fprintf(os.Stderr, "hyprd lock: running lock client did not lock within %v; stopping it\n", lockWait)
			l.stopForeign()
			return false
		}
		if acquired {
			time.Sleep(releasePoll)
		} else {
			time.Sleep(lockPoll)
		}
	}
	if !acquired {
		fmt.Fprintln(os.Stderr, "hyprd lock: running lock client exited without locking; relaunching")
		return false
	}
	locked, err := l.hypr.Locked()
	if err != nil || locked {
		fmt.Fprintln(os.Stderr, "hyprd lock: running lock client exited without releasing the session lock; relaunching")
		return false
	}
	return true
}

func (l *Lock) hold(adopted bool) {
	strikes := 0
	alerted := false
	for launch := 0; ; launch++ {
		if launch > 1 {
			time.Sleep(relaunchDelay)
		}
		if l.running() {
			fmt.Fprintln(os.Stderr, "hyprd lock: another lock client is running; supervising it")
			if l.superviseForeign(adopted) {
				l.release()
				return
			}
		}
		adopted = false
		l.submap(barrierSubmap)
		l.record(intentPending)
		started := time.Now()
		deadline := time.After(lockWait)
		ctx, stop := context.WithCancel(context.Background())
		log := newLockLog(os.Stderr)
		exited := make(chan error, 1)
		go func() { exited <- l.launch(ctx, log) }()
		if launch == 0 {
			go l.cover()
		}

		acquired, err := l.awaitUnlock(exited, stop, deadline, log)
		stop()
		if err == nil {
			l.release()
			return
		}
		fmt.Fprintf(os.Stderr, "hyprd lock: %v; relaunching\n", err)
		if !alerted {
			alerted = true
			l.alert(err.Error() + "; relaunching")
		}
		if acquired && time.Since(started) >= quickExit {
			strikes = 0
			continue
		}
		strikes++
		if strikes < quickExits {
			continue
		}
		strikes = 0
		if err := l.endSession(); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd lock: lock client cannot hold the lock and the session did not end: %v\n", err)
			l.alert("lock client cannot hold the lock and the session did not end: " + err.Error())
			continue
		}
		fmt.Fprintln(os.Stderr, "hyprd lock: lock client cannot hold the lock; ending the session")
		return
	}
}

// awaitUnlock keeps the input barrier until this client logs secureMarker and stops a client that misses lockWait.
func (l *Lock) awaitUnlock(exited <-chan error, stop context.CancelFunc, deadline <-chan time.Time, log *lockLog) (bool, error) {
	select {
	case err := <-exited:
		if err != nil {
			return false, fmt.Errorf("lock client exited before the lock was secure: %w", err)
		}
		return false, errors.New("lock client exited 0 before the lock was secure")
	case <-log.abort:
		stop()
		return false, fmt.Errorf("lock client aborted the lock before it was secure: %v", <-exited)
	case <-deadline:
		stop()
		return false, fmt.Errorf("lock not secure after %v; stopped the lock client: %v", lockWait, <-exited)
	case <-log.secure:
		l.record(intentAcquired)
		l.submap("reset")
		return true, l.awaitRelease(exited, stop, log)
	}
}

// awaitRelease validates a clean exit against compositor state, but kills a lingering client after releaseGrace and accepts the observed unlock.
func (l *Lock) awaitRelease(exited <-chan error, stop context.CancelFunc, log *lockLog) error {
	watch := time.NewTicker(releasePoll)
	defer watch.Stop()
	var grace <-chan time.Time
	for {
		select {
		case err := <-exited:
			if log.done(log.abort) {
				return fmt.Errorf("lock client aborted the lock: %v", err)
			}
			return l.checkRelease(err)
		case <-log.abort:
			stop()
			return fmt.Errorf("lock client aborted the lock; stopped it: %v", <-exited)
		case <-grace:
			stop()
			err := <-exited
			if log.done(log.abort) {
				return fmt.Errorf("lock client aborted the lock: %v", err)
			}
			fmt.Fprintf(os.Stderr, "hyprd lock: lock client still running %v after the compositor unlocked; stopped it\n", releaseGrace)
			return nil
		case <-watch.C:
			if locked, err := l.hypr.Locked(); err == nil && !locked {
				watch.Stop()
				grace = time.After(releaseGrace)
			}
		}
	}
}

func (l *Lock) checkRelease(err error) error {
	if err != nil {
		return fmt.Errorf("lock client exited: %w", err)
	}
	locked, err := l.hypr.Locked()
	if err != nil {
		return fmt.Errorf("lock client exited 0; query session lock: %w", err)
	}
	if locked {
		return errors.New("lock client exited 0 without releasing the session lock")
	}
	return nil
}

func (l *Lock) alert(body string) {
	if l.notify != nil {
		go l.notify("hyprd", "critical", "Lock failed", body)
	}
}

func (l *Lock) resetSubmap() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFull {
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
	playing := playerctlStatus() == "Playing"
	l.mu.Lock()
	if !l.inFull || l.saved != nil {
		l.mu.Unlock()
		return
	}
	l.saved = l.capture()
	l.saved.musicPlaying = playing
	l.mu.Unlock()

	if err := l.hypr.FocusWorkspace(coverWorkspace); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: switch to workspace %d: %v\n", coverWorkspace, err)
	}
	cfg := l.state.GetConfig()
	enterBlackout(&cfg.Background)
}

func intentPath() string {
	dir, sig := os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if dir == "" || sig == "" || strings.ContainsRune(sig, '/') {
		return ""
	}
	return filepath.Join(dir, "hyprd-lock-"+sig)
}

func (l *Lock) record(intent string) {
	if l.intent == "" {
		fmt.Fprintln(os.Stderr, "hyprd lock: no XDG_RUNTIME_DIR or HYPRLAND_INSTANCE_SIGNATURE; lock intent not recorded")
		return
	}
	var err error
	if intent == "" {
		if err = os.Remove(l.intent); errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
	} else {
		err = os.WriteFile(l.intent, []byte(intent), 0o600)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: record intent: %v\n", err)
	}
}

func (l *Lock) readIntent() string {
	if l.intent == "" {
		return ""
	}
	data, err := os.ReadFile(l.intent)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err == nil && strings.TrimSpace(string(data)) == intentAcquired {
		return intentAcquired
	}
	return intentPending
}

func (l *Lock) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	saved := l.saved
	if saved == nil {
		saved = l.capture()
	}
	l.saved = nil
	l.inFull = false
	l.record("")
	l.submap("reset")
	if err := l.exitBlackout(saved); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: unlock: %v\n", err)
	}
}

func lockCommand(ctx context.Context, log *lockLog) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "qs", "-c", "lock")
	cmd.Env = append(os.Environ(), lockEnv, "QS_DISABLE_CRASH_HANDLER=1")
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.WaitDelay = time.Second
	return cmd
}

func execLock(ctx context.Context, out io.Writer) error {
	log, ok := out.(*lockLog)
	if !ok {
		log = newLockLog(out)
	}
	err := lockCommand(ctx, log).Run()
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	return err
}

type lockLog struct {
	out    io.Writer
	line   []byte
	secure chan struct{}
	abort  chan struct{}
}

func newLockLog(out io.Writer) *lockLog {
	return &lockLog{out: out, secure: make(chan struct{}), abort: make(chan struct{})}
}

func (g *lockLog) Write(p []byte) (int, error) {
	g.out.Write(p)
	g.line = append(g.line, p...)
	if !g.done(g.secure) && bytes.Contains(g.line, []byte(secureMarker)) {
		close(g.secure)
	}
	if !g.done(g.abort) && bytes.Contains(g.line, []byte(abortMarker)) {
		close(g.abort)
	}
	if i := bytes.LastIndexByte(g.line, '\n'); i >= 0 {
		g.line = g.line[i+1:]
	}
	if n := len(g.line); n > logTail {
		g.line = slices.Clone(g.line[n-logTail:])
	}
	return len(p), nil
}

func (g *lockLog) done(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func lockRunning() bool {
	return len(lockPids()) > 0
}

func stopLocks() {
	for _, pid := range lockPids() {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			fmt.Fprintf(os.Stderr, "hyprd lock: stop lock client %d: %v\n", pid, err)
		}
	}
}

func lockPids() []int {
	procs, err := os.ReadDir("/proc")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: list processes: %v\n", err)
		return nil
	}
	var pids []int
	for _, proc := range procs {
		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}
		dir := filepath.Join("/proc", proc.Name())
		exe, err := os.Readlink(filepath.Join(dir, "exe"))
		if err != nil || strings.TrimSuffix(filepath.Base(exe), " (deleted)") != "quickshell" {
			continue
		}
		env, err := os.ReadFile(filepath.Join(dir, "environ"))
		if err == nil && slices.Contains(strings.Split(string(env), "\x00"), lockEnv) {
			pids = append(pids, pid)
		}
	}
	return pids
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

// capture requires l.mu to be held.
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

func enterBlackout(bg *config.BackgroundConfig) {
	coverRun("killall", "glava")
	coverRun("dunstctl", "close-all")
	coverRun("dunstctl", "set-paused", "true")
	coverRun("playerctl", "--player=spotify", "pause")
	if err := NewBG(bg).SetPaused(true); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: pause background: %v\n", err)
	}
	closeEwwWidgets()
}

func coverRun(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), coverTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func closeEwwWidgets() {
	out, err := coverRun("ewwd", "close")
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "hyprd lock: ewwd close unavailable: %v: %s\n", err, strings.TrimSpace(string(out)))
	if _, err := coverRun("eww", "ping"); err != nil {
		return
	}
	if out, err := coverRun("eww", "close-all"); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: eww close-all: %v: %s\n", err, strings.TrimSpace(string(out)))
	}
}

// exitBlackout requires l.mu to be held.
func (l *Lock) exitBlackout(saved *lockState) error {
	cfg := l.state.GetConfig()
	if err := l.hypr.FocusWorkspace(saved.workspace); err != nil {
		return fmt.Errorf("lock: restore workspace %d: %w", saved.workspace, err)
	}
	if err := EnsureBG(&cfg.Background); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: background: %v\n", err)
	}
	if err := NewBG(&cfg.Background).SetPaused(false); err != nil {
		fmt.Fprintf(os.Stderr, "hyprd lock: resume background: %v\n", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), coverTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "playerctl", "status").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
