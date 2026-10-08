package setup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"
)

func Start(ctx context.Context, u *ui.UI, s Stage, force bool, home string) (wait func(context.Context) (Report, error), err error) {
	rel := filepath.Join(".local", "state", "dctl", s.Name+".log")
	path := filepath.Join(home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	log, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	validate := exec.CommandContext(ctx, "sudo", "-v")
	validate.Stdin, validate.Stdout, validate.Stderr = os.Stdin, os.Stderr, os.Stderr
	err = validate.Run()
	if ctx.Err() != nil {
		log.Close()
		return nil, ctx.Err()
	}
	if err != nil {
		log.Close()
		return nil, ErrRefused
	}
	run, stop := context.WithCancel(context.WithoutCancel(ctx))
	keep := keepalive(run)
	var r Report
	var applyErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		r, applyErr = s.Apply(execx.Logged(run, log), force)
	}()
	var draining atomic.Bool
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		defer signal.Stop(sig)
		for {
			select {
			case <-done:
				return
			case <-sig:
				if !draining.CompareAndSwap(false, true) {
					stop()
					return
				}
			}
		}
	}()
	shown := "~/" + rel
	u.Line(ui.Info, s.Name, s.About+" · log "+shown)
	return func(ctx context.Context) (Report, error) {
		defer stop()
		u.Begin(s.Name, s.About)
		spin := func(ctx context.Context) error {
			select {
			case <-done:
			case <-ctx.Done():
			}
			return nil
		}
		err := ctx.Err()
		if err == nil {
			err = u.Spin(ctx, "background install", spin)
		}
		if err != nil || ctx.Err() != nil {
			if errors.Is(context.Cause(ctx), ui.ErrCanceled) {
				draining.Store(true)
			}
			u.Warn("waiting for %s to finish · Ctrl+C again interrupts it", s.Name)
			_ = u.Spin(context.WithoutCancel(ctx), "background install", spin)
		}
		<-done
		keep()
		log.Close()
		for _, it := range r.Items {
			if it.State != Done {
				head, _, _ := strings.Cut(it.Detail, "\n")
				u.Row(level(it.State), short(r, it.Item)+": "+head)
			}
		}
		if r.State == Failed {
			u.Row(ui.Info, "last lines of "+shown)
			u.Detail("%s", tail(path, 20))
		}
		u.End(level(r.State), "")
		return r, applyErr
	}, nil
}

func keepalive(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				_ = exec.CommandContext(ctx, "sudo", "-n", "-v").Run()
			}
		}
	}()
	return cancel
}

func tail(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
