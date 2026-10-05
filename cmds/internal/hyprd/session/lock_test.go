package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/hypr"
	"dotfiles/cmds/internal/hyprd/state"
)

const (
	restoreRequest = "workspace = 1 }"
	barrierRequest = `hl.dsp.submap("lockbarrier")`
	resetRequest   = `hl.dsp.submap("reset")`
)

type fakeHypr struct {
	mu       sync.Mutex
	requests []string
	locked   bool
	fail     string
}

func (f *fakeHypr) failing(substr string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = substr
}

func (f *fakeHypr) setLocked(locked bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locked = locked
}

func (f *fakeHypr) secure(w io.Writer) {
	f.setLocked(true)
	fmt.Fprintln(w, "  WARN js: "+secureMarker)
}

func (f *fakeHypr) hold(w io.Writer, d time.Duration) error {
	f.secure(w)
	time.Sleep(d)
	f.setLocked(false)
	return nil
}

func (f *fakeHypr) count(substr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.Contains(r, substr) {
			n++
		}
	}
	return n
}

func (f *fakeHypr) serve(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	req := string(buf[:n])
	f.mu.Lock()
	f.requests = append(f.requests, req)
	locked, fail := f.locked, f.fail
	f.mu.Unlock()
	if fail != "" && strings.Contains(req, fail) {
		conn.Write([]byte("error: request failed"))
		return
	}
	if req == "j/locked" {
		fmt.Fprintf(conn, `{"locked": %t}`, locked)
		return
	}
	conn.Write([]byte("ok"))
}

// newTestLock must run outside the synctest bubble so the fake Hyprland socket stays outside it.
func newTestLock(t *testing.T) (*Lock, *fakeHypr) {
	t.Setenv("PATH", t.TempDir())
	dir, err := os.MkdirTemp("", "lock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	sock := filepath.Join(dir, "hypr", "test", ".socket.sock")
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	fake := &fakeHypr{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go fake.serve(conn)
		}
	}()

	client, err := hypr.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	l := NewLock(client, state.NewState(&config.HyprConfig{}))
	l.running = func() bool { return false }
	l.stopForeign = func() {}
	l.endSession = func() error {
		t.Error("endSession called")
		return nil
	}
	return l, fake
}

func TestLockCommandRunsLockMode(t *testing.T) {
	log := newLockLog(io.Discard)
	cmd := lockCommand(context.Background(), log)
	if !slices.Equal(cmd.Args, []string{"qs", "-c", "lock"}) {
		t.Fatalf("lock args = %q, want qs -c lock", cmd.Args)
	}
	if cmd.Stdout != log || cmd.Stderr != log || cmd.WaitDelay <= 0 {
		t.Error("lock output is not scanned on both streams with a bounded wait")
	}
	for _, kv := range []string{lockEnv, "QS_DISABLE_CRASH_HANDLER=1"} {
		if !slices.Contains(cmd.Env, kv) {
			t.Errorf("lock env lacks %s", kv)
		}
	}
}

func TestFullLockRestoresOnceAfterCleanExit(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			fake.secure(w)
			time.Sleep(time.Minute)
			if !l.Locked() || fake.count(restoreRequest) != 0 {
				t.Error("full lock released while the lock client was running")
			}
			fake.setLocked(false)
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		if got := l.readIntent(); got != intentPending {
			t.Errorf("intent after Full = %q, want %q", got, intentPending)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 1 || fake.count(restoreRequest) != 1 || l.Locked() {
			t.Errorf("launches = %d, restores = %d, locked = %v; want one launch, one restore, unlocked",
				launches, fake.count(restoreRequest), l.Locked())
		}
		if got := l.readIntent(); got != "" {
			t.Errorf("intent after release = %q, want cleared", got)
		}
	})
}

func TestFullLockRejectsCleanExitWithoutRelease(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			switch launches {
			case 1:
				return nil
			case 2:
				fake.secure(w)
				time.Sleep(time.Minute)
				return nil
			default:
				return fake.hold(w, time.Minute)
			}
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 3 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want relaunches after an exit without locking and an exit without unlocking, then one restore",
				launches, l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestFullLockStopsClientThatNeverLocks(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		var starts []time.Time
		l.launch = func(ctx context.Context, w io.Writer) error {
			starts = append(starts, time.Now())
			if len(starts) > 1 {
				return fake.hold(w, time.Minute)
			}
			<-ctx.Done()
			return ctx.Err()
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if len(starts) != 2 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Fatalf("launches = %d, locked = %v, restores = %d; want a relaunch after the stuck client, then one restore",
				len(starts), l.Locked(), fake.count(restoreRequest))
		}
		if gap := starts[1].Sub(starts[0]); gap != lockWait {
			t.Errorf("stuck client stopped after %v, want %v", gap, lockWait)
		}
	})
}

func TestFullLockEndsSessionAfterQuickFailures(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches, ends := 0, 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			return errors.New("lock client crashed")
		}
		l.endSession = func() error {
			ends++
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != quickExits || ends != 1 {
			t.Errorf("launches = %d, ends = %d; want %d launches and one session end", launches, ends, quickExits)
		}
		if !l.Locked() || fake.count(restoreRequest) != 0 {
			t.Errorf("locked = %v, restores = %d; want the lock kept without restore", l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestFullLockWaitsForForeignLock(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		foreignUntil := time.Now().Add(time.Minute)
		l.running = func() bool { return time.Now().Before(foreignUntil) }
		l.stopForeign = func() { foreignUntil = time.Now() }
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			if l.running() {
				t.Error("launched while a foreign lock client was running")
			}
			return fake.hold(w, time.Minute)
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 1 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want one launch after the foreign lock client, then one restore",
				launches, l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestFullLockKeepsRelaunchingWhenSessionCannotEnd(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		const failures = 7
		var starts []time.Time
		ends := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			starts = append(starts, time.Now())
			if fake.count(restoreRequest) != 0 {
				t.Error("restored before the lock client exited cleanly")
			}
			if len(starts) > failures {
				return fake.hold(w, time.Second)
			}
			return errors.New("lock client crashed")
		}
		l.endSession = func() error {
			ends++
			return errors.New("loginctl and hyprctl failed")
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if len(starts) != failures+1 || ends != failures/quickExits {
			t.Fatalf("launches = %d, ends = %d; want %d launches and %d end attempts", len(starts), ends, failures+1, failures/quickExits)
		}
		for i := 2; i < len(starts); i++ {
			if gap := starts[i].Sub(starts[i-1]); gap != relaunchDelay {
				t.Errorf("launch %d came %v after launch %d, want %v", i, gap, i-1, relaunchDelay)
			}
		}
		if fake.count(restoreRequest) != 1 {
			t.Errorf("restores = %d, want one after the clean exit", fake.count(restoreRequest))
		}
	})
}

func TestFullLockRelaunchesSlowCrashWithSpacing(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		var starts, exits []time.Time
		l.launch = func(_ context.Context, w io.Writer) error {
			starts = append(starts, time.Now())
			defer func() { exits = append(exits, time.Now()) }()
			if len(starts) == 3 {
				return fake.hold(w, time.Second)
			}
			fake.secure(w)
			time.Sleep(time.Minute)
			return errors.New("lock client crashed")
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if len(starts) != 3 {
			t.Fatalf("launches = %d, want 3", len(starts))
		}
		if gap := starts[1].Sub(exits[0]); gap != 0 {
			t.Errorf("first relaunch waited %v, want immediate", gap)
		}
		if gap := starts[2].Sub(exits[1]); gap != relaunchDelay {
			t.Errorf("second relaunch waited %v, want %v", gap, relaunchDelay)
		}
		if l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("locked = %v, restores = %d; want one restore after the clean exit", l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestConcurrentFullLockStartsOneClient(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		launches, started := 0, 0
		l.launch = func(_ context.Context, w io.Writer) error {
			mu.Lock()
			launches++
			mu.Unlock()
			return fake.hold(w, time.Minute)
		}

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				out, err := l.Full()
				if err != nil {
					t.Error(err)
				}
				if out == "lock: full" {
					mu.Lock()
					started++
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 1 || started != 1 {
			t.Errorf("launches = %d, started = %d; want one lock client", launches, started)
		}
	})
}

func TestFullLockKeepsBarrierUntilLocked(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		var beforeLock, whileLocked int
		l.launch = func(_ context.Context, w io.Writer) error {
			time.Sleep(time.Second)
			beforeLock = fake.count(resetRequest)
			fake.secure(w)
			time.Sleep(time.Minute)
			whileLocked = fake.count(resetRequest)
			fake.setLocked(false)
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if beforeLock != 0 {
			t.Errorf("submap reset %d times before the compositor locked, want 0", beforeLock)
		}
		if whileLocked != 1 {
			t.Errorf("submap reset %d times while locked, want 1 after the lock was acquired", whileLocked)
		}
		if fake.count(barrierRequest) != 1 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("barriers = %d, locked = %v, restores = %d; want one barrier, then one restore",
				fake.count(barrierRequest), l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestLockDyingBeforeLockKeepsBarrier(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches, ends := 0, 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			if got := fake.count(barrierRequest); got != launches {
				t.Errorf("launch %d started after %d barriers, want one per launch", launches, got)
			}
			time.Sleep(time.Second)
			return errors.New("lock client crashed before acquiring the lock")
		}
		l.endSession = func() error {
			ends++
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != quickExits || ends != 1 {
			t.Errorf("launches = %d, ends = %d; want %d launches and one session end", launches, ends, quickExits)
		}
		if fake.count(resetRequest) != 0 || !l.Locked() {
			t.Errorf("resets = %d, locked = %v; want the barrier kept and the lock held", fake.count(resetRequest), l.Locked())
		}
	})
}

func TestAdoptRelaunchesOnAbandonedLock(t *testing.T) {
	l, fake := newTestLock(t)
	fake.setLocked(true)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			if fake.count(barrierRequest) != 1 {
				t.Error("lock client relaunched without the barrier")
			}
			fmt.Fprintln(w, secureMarker)
			time.Sleep(time.Minute)
			fake.setLocked(false)
			return nil
		}

		if err := l.Adopt(); err != nil {
			t.Fatal(err)
		}
		if !l.Locked() {
			t.Fatal("Adopt left the abandoned lock unsupervised")
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 1 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want one relaunch, then one restore",
				launches, l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestAdoptReleasesForeignUnlock(t *testing.T) {
	for _, unlocks := range []bool{true, false} {
		l, fake := newTestLock(t)
		fake.setLocked(true)
		synctest.Test(t, func(t *testing.T) {
			foreignUntil := time.Now().Add(time.Minute)
			l.running = func() bool { return time.Now().Before(foreignUntil) }
			launches := 0
			l.launch = func(_ context.Context, w io.Writer) error {
				launches++
				return fake.hold(w, time.Minute)
			}

			if err := l.Adopt(); err != nil {
				t.Fatal(err)
			}
			if unlocks {
				time.Sleep(30 * time.Second)
				fake.setLocked(false)
			}
			time.Sleep(time.Hour)
			synctest.Wait()

			want := 1
			if unlocks {
				want = 0
			}
			if launches != want || l.Locked() || fake.count(restoreRequest) != 1 {
				t.Errorf("unlocks = %v: launches = %d, locked = %v, restores = %d; want %d launches, then one restore",
					unlocks, launches, l.Locked(), fake.count(restoreRequest), want)
			}
		})
	}
}

func TestAdoptHonorsRecordedIntent(t *testing.T) {
	for _, intent := range []string{intentPending, intentAcquired, "other"} {
		l, fake := newTestLock(t)
		if intent == "other" {
			stale := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "hyprd-lock-other")
			if err := os.WriteFile(stale, []byte(intentPending), 0o600); err != nil {
				t.Fatal(err)
			}
		} else {
			l.record(intent)
		}
		synctest.Test(t, func(t *testing.T) {
			launches := 0
			l.launch = func(_ context.Context, w io.Writer) error {
				launches++
				return fake.hold(w, time.Minute)
			}

			if err := l.Adopt(); err != nil {
				t.Fatal(err)
			}
			if intent != "other" && !l.Locked() {
				t.Errorf("intent %s: Adopt left the recorded lock unsupervised", intent)
			}
			time.Sleep(time.Hour)
			synctest.Wait()

			wantLaunches, wantRestores := 0, 0
			switch intent {
			case intentPending:
				wantLaunches, wantRestores = 1, 1
			case intentAcquired:
				wantRestores = 1
			}
			if launches != wantLaunches || fake.count(restoreRequest) != wantRestores || l.Locked() || l.readIntent() != "" {
				t.Errorf("intent %s: launches = %d, restores = %d, locked = %v, intent = %q; want %d launches and %d restores, then cleared",
					intent, launches, fake.count(restoreRequest), l.Locked(), l.readIntent(), wantLaunches, wantRestores)
			}
		})
	}
}

func TestRestartDefersLock(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			return fake.hold(w, time.Minute)
		}

		if !l.BeginRestart() {
			t.Fatal("BeginRestart refused without a lock")
		}
		out, err := l.Full()
		if err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if l.Locked() || launches != 0 || l.readIntent() != intentPending {
			t.Errorf("%s: locked = %v, launches = %d, intent = %q; want a recorded intent and no launch during restart",
				out, l.Locked(), launches, l.readIntent())
		}

		l.CancelRestart()
		if !l.Locked() || l.BeginRestart() {
			t.Error("canceled restart left the deferred lock unsupervised or allowed a new restart")
		}
		time.Sleep(time.Hour)
		synctest.Wait()
		if launches != 1 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want the deferred lock launched once, then one restore",
				launches, l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestNeverAcquiredEndsSession(t *testing.T) {
	l, _ := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches, ends := 0, 0
		l.launch = func(ctx context.Context, w io.Writer) error {
			launches++
			<-ctx.Done()
			return ctx.Err()
		}
		l.endSession = func() error {
			ends++
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != quickExits || ends != 1 || !l.Locked() {
			t.Errorf("launches = %d, ends = %d, locked = %v; want %d slow never-acquired launches, then one session end",
				launches, ends, l.Locked(), quickExits)
		}
	})
}

func TestAdoptStopsForeignThatNeverLocks(t *testing.T) {
	l, fake := newTestLock(t)
	l.record(intentPending)
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var stopped time.Time
		l.running = func() bool { return stopped.IsZero() }
		l.stopForeign = func() { stopped = time.Now() }
		launches := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			return fake.hold(w, time.Minute)
		}

		if err := l.Adopt(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if gap := stopped.Sub(start); gap < lockWait || gap > lockWait+lockPoll {
			t.Errorf("foreign client stopped after %v, want about %v", gap, lockWait)
		}
		if launches != 1 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want our own lock launched once, then one restore",
				launches, l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestWatchdogStopsHungClientAfterUnlock(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		var unlocked, stopped time.Time
		l.launch = func(ctx context.Context, w io.Writer) error {
			fake.secure(w)
			time.Sleep(time.Minute)
			fake.setLocked(false)
			unlocked = time.Now()
			<-ctx.Done()
			stopped = time.Now()
			return ctx.Err()
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if gap := stopped.Sub(unlocked); gap < releaseGrace || gap > releasePoll+releaseGrace {
			t.Errorf("hung client stopped %v after unlock, want between %v and %v", gap, releaseGrace, releasePoll+releaseGrace)
		}
		if l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("locked = %v, restores = %d; want one restore after the hung client was stopped", l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestWatchdogIgnoresQueryErrors(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		l.launch = func(ctx context.Context, w io.Writer) error {
			fake.secure(w)
			time.Sleep(time.Second)
			fake.failing("j/locked")
			fake.setLocked(false)
			time.Sleep(time.Minute)
			if ctx.Err() != nil {
				t.Error("watchdog stopped the client on a failed query")
			}
			fake.failing("")
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("locked = %v, restores = %d; want one restore after the clean exit", l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestStaleLockedWithoutMarkerHitsDeadline(t *testing.T) {
	l, fake := newTestLock(t)
	fake.setLocked(true)
	synctest.Test(t, func(t *testing.T) {
		var starts []time.Time
		l.launch = func(ctx context.Context, w io.Writer) error {
			starts = append(starts, time.Now())
			if len(starts) > 1 {
				return fake.hold(w, time.Minute)
			}
			<-ctx.Done()
			return ctx.Err()
		}

		if err := l.Adopt(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if len(starts) != 2 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Fatalf("launches = %d, locked = %v, restores = %d; want a relaunch after the unmarked client, then one restore",
				len(starts), l.Locked(), fake.count(restoreRequest))
		}
		if gap := starts[1].Sub(starts[0]); gap != lockWait {
			t.Errorf("unmarked client stopped after %v despite stale j/locked, want %v", gap, lockWait)
		}
	})
}

func TestAbortMarkerForcesRelaunch(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			if launches > 1 {
				return fake.hold(w, time.Minute)
			}
			fake.secure(w)
			time.Sleep(time.Minute)
			fake.setLocked(false)
			fmt.Fprintln(w, "  WARN: WlSessionLock.surface does not create a WlSessionLockSurface. "+abortMarker)
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 2 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want a relaunch after the aborted lock, then one restore",
				launches, l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestLockLogFindsMarkersAfterLongLines(t *testing.T) {
	var out strings.Builder
	log := newLockLog(&out)
	long := strings.Repeat("x", 1<<20)
	fmt.Fprintf(log, "%s\n%s", long, secureMarker[:5])
	fmt.Fprintf(log, "%s\n%s %s\n", secureMarker[5:], long, abortMarker)
	if !log.done(log.secure) || !log.done(log.abort) {
		t.Errorf("secure = %v, abort = %v; want both markers found", log.done(log.secure), log.done(log.abort))
	}
	if out.Len() != 2*len(long)+len(secureMarker)+len(abortMarker)+4 {
		t.Errorf("forwarded %d bytes, want every byte forwarded", out.Len())
	}
	if len(log.line) > logTail {
		t.Errorf("buffered %d bytes, want at most %d", len(log.line), logTail)
	}
}

func TestBarrierFailureStillLaunchesLock(t *testing.T) {
	l, fake := newTestLock(t)
	fake.failing(barrierRequest)
	synctest.Test(t, func(t *testing.T) {
		launches, beforeLock := 0, -1
		l.launch = func(_ context.Context, w io.Writer) error {
			launches++
			beforeLock = fake.count(resetRequest)
			return fake.hold(w, time.Minute)
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 1 || beforeLock != 0 || fake.count(barrierRequest) != 1 {
			t.Errorf("launches = %d, resets before lock = %d, barriers = %d; want one launch after one failed barrier and no early reset",
				launches, beforeLock, fake.count(barrierRequest))
		}
		if l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("locked = %v, restores = %d; want one restore after the clean exit", l.Locked(), fake.count(restoreRequest))
		}
	})
}

func TestAdoptFailsWhenLockQueryFails(t *testing.T) {
	l, fake := newTestLock(t)
	fake.failing("j/locked")
	l.launch = func(_ context.Context, w io.Writer) error {
		t.Error("lock client launched without a lock state")
		return nil
	}
	if err := l.Adopt(); err == nil {
		t.Fatal("Adopt succeeded without a session lock state")
	}
	if l.Locked() {
		t.Error("Adopt started supervision after a failed query")
	}
}

func TestPickerKeepsLockBarrierAndWorkspace(t *testing.T) {
	const focusSelected = "workspace = 2 }"
	l, fake := newTestLock(t)
	st := state.NewState(&config.HyprConfig{Sessions: config.SessionsConfig{
		"work": {Name: "work", Workspace: 2, Command: "true"},
	}})
	p := NewPicker(l.hypr, st, l)
	synctest.Test(t, func(t *testing.T) {
		reopen := func() {
			p.mu.Lock()
			p.active, p.ws, p.cache = true, 2, map[int][]string{2: {"work"}}
			p.mu.Unlock()
		}
		confirm := func() {
			reopen()
			if _, err := p.Execute("confirm"); err != nil {
				t.Fatal(err)
			}
		}

		confirm()
		time.Sleep(time.Second)
		synctest.Wait()
		if resets, focuses := fake.count(resetRequest), fake.count(focusSelected); resets != 1 || focuses != 1 {
			t.Fatalf("unlocked confirm: resets = %d, focuses = %d; want 1 and 1", resets, focuses)
		}

		confirm()
		l.mu.Lock()
		l.inFull = true
		l.mu.Unlock()
		time.Sleep(time.Second)
		synctest.Wait()
		if resets, focuses := fake.count(resetRequest), fake.count(focusSelected); resets != 1 || focuses != 1 {
			t.Errorf("confirm during full lock: resets = %d, focuses = %d; want the barrier and blackout kept", resets, focuses)
		}

		reopen()
		if _, err := p.Execute("close"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if got := fake.count(resetRequest); got != 1 {
			t.Errorf("close during full lock: resets = %d, want the barrier kept", got)
		}
	})
}
