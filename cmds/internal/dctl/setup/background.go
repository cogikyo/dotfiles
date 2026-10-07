package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
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
	shown := "~/" + rel
	u.Node(ui.Info, fmt.Sprintf("%-10s", s.Name), "started in background · log "+shown)
	return func(ctx context.Context) (Report, error) {
		defer stop()
		spin := func(ctx context.Context) error {
			select {
			case <-done:
			case <-ctx.Done():
			}
			return nil
		}
		err := ctx.Err()
		if err == nil {
			err = u.Spin(ctx, s.Name+": waiting for the background run", spin)
		}
		if err != nil || ctx.Err() != nil {
			u.Warn("waiting for %s to finish safely · Ctrl+C again to interrupt", s.Name)
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt)
			select {
			case <-sig:
				stop()
			case <-done:
			}
			<-done
			signal.Stop(sig)
		}
		<-done
		keep()
		log.Close()
		Show(u, r)
		if r.State == Failed {
			u.Detail("last lines of %s:", shown)
			u.Detail("%s", tail(path, 20))
		}
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
