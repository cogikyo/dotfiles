package session

import (
	"errors"
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

const restoreRequest = "workspace = 1 }"

type fakeHypr struct {
	mu       sync.Mutex
	requests []string
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
	f.mu.Lock()
	f.requests = append(f.requests, string(buf[:n]))
	f.mu.Unlock()
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
	l.endSession = func() error {
		t.Error("endSession called")
		return nil
	}
	return l, fake
}

func TestHyprlockCommandHasNoGrace(t *testing.T) {
	args := hyprlockCommand().Args
	if !slices.Equal(args, []string{"hyprlock", "--grace", "0"}) {
		t.Fatalf("hyprlock args = %q, want zero grace", args)
	}
}

func TestFullLockRestoresOnceAfterCleanExit(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		l.hyprlock = func() error {
			launches++
			time.Sleep(time.Minute)
			if !l.Locked() || fake.count(restoreRequest) != 0 {
				t.Error("full lock released while hyprlock was running")
			}
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 1 || fake.count(restoreRequest) != 1 || l.Locked() {
			t.Errorf("launches = %d, restores = %d, locked = %v; want one launch, one restore, unlocked",
				launches, fake.count(restoreRequest), l.Locked())
		}
	})
}

func TestFullLockEndsSessionAfterQuickFailures(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches, ends := 0, 0
		l.hyprlock = func() error {
			launches++
			return errors.New("hyprlock crashed")
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

func TestFullLockWaitsForForeignHyprlock(t *testing.T) {
	l, fake := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		launches := 0
		foreignUntil := time.Now().Add(time.Minute)
		l.running = func() bool { return time.Now().Before(foreignUntil) }
		l.hyprlock = func() error {
			launches++
			if l.running() {
				return errors.New("session already locked")
			}
			return nil
		}

		if _, err := l.Full(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
		synctest.Wait()

		if launches != 2 || l.Locked() || fake.count(restoreRequest) != 1 {
			t.Errorf("launches = %d, locked = %v, restores = %d; want a relaunch after the foreign hyprlock, then one restore",
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
		l.hyprlock = func() error {
			starts = append(starts, time.Now())
			if fake.count(restoreRequest) != 0 {
				t.Error("restored before hyprlock exited cleanly")
			}
			if len(starts) > failures {
				return nil
			}
			return errors.New("hyprlock crashed")
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
		l.hyprlock = func() error {
			starts = append(starts, time.Now())
			defer func() { exits = append(exits, time.Now()) }()
			if len(starts) == 3 {
				return nil
			}
			time.Sleep(time.Minute)
			return errors.New("hyprlock crashed")
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

func TestConcurrentFullLockStartsOneHyprlock(t *testing.T) {
	l, _ := newTestLock(t)
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		launches, started := 0, 0
		l.hyprlock = func() error {
			mu.Lock()
			launches++
			mu.Unlock()
			time.Sleep(time.Minute)
			return nil
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
			t.Errorf("launches = %d, started = %d; want one hyprlock", launches, started)
		}
	})
}
