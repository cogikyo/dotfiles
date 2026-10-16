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
	"unicode"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"

	"github.com/charmbracelet/x/ansi"
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
	if err := Sudo(ctx, u, "install "+s.Name+" in the background", "dctl setup "+s.Name); err != nil {
		log.Close()
		return nil, err
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
	waiting := "waiting for " + s.Name + " to finish"
	hints := make(chan string, 2)
	hint := func(keys string) {
		select {
		case hints <- waiting + " · " + keys:
		default:
		}
	}
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
				hint("Ctrl+C again interrupts it")
			}
		}
	}()
	shown := "~/" + rel
	note := s.About + " · log " + shown
	u.Line(ui.Info, s.Name, note)
	return func(ctx context.Context) (Report, error) {
		defer stop()
		u.Begin(s.Name, note)
		go func() {
			select {
			case <-done:
			case <-ctx.Done():
				if errors.Is(context.Cause(ctx), ui.ErrCanceled) && draining.CompareAndSwap(false, true) {
					hint("Ctrl+C interrupts it")
				}
			}
		}()
		u.Await(waiting, hints, done)
		keep()
		log.Close()
		r.show(u)
		if r.State == Failed {
			u.Row(ui.Info, "last lines of "+shown)
			u.Detail("%s", tail(path, 8))
		}
		u.End(level(r.State), "")
		return r, applyErr
	}, nil
}

type noTerminal struct{ rerun string }

func (e noTerminal) Error() string {
	return "sudo needs a password and no terminal can ask; run `" + e.rerun + "` in a terminal"
}

func (noTerminal) Unwrap() error { return ui.ErrNoTTY }

func Sudo(ctx context.Context, u *ui.UI, why, rerun string) error {
	if exec.CommandContext(ctx, "sudo", "-n", "-v").Run() == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return noTerminal{rerun}
	}
	tty.Close()
	u.Line(ui.Info, "sudo", why)
	cmd := exec.CommandContext(ctx, "sudo", "-v", "-p", "dctl: sudo password for %u: ")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	err = cmd.Run()
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case err != nil:
		return ErrRefused
	}
	return nil
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
	var lines []string
	for l := range strings.Lines(strings.ToValidUTF8(string(data), "\uFFFD")) {
		l = strings.TrimRight(l, "\r\n")
		l = l[strings.LastIndexByte(l, '\r')+1:]
		l = strings.Map(func(r rune) rune {
			switch {
			case r == '\t':
				return ' '
			case unicode.IsControl(r):
				return -1
			}
			return r
		}, ansi.Strip(l))
		if !progress(l) {
			lines = append(lines, strings.TrimRight(l, " "))
		}
	}
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

func progress(line string) bool {
	fields := strings.Fields(line)
	switch {
	case len(fields) == 0, fields[0] == "%", fields[0] == "Dload", strings.HasSuffix(line, " downloading..."):
		return true
	}
	return !strings.ContainsFunc(line, func(r rune) bool { return unicode.IsLetter(r) && !strings.ContainsRune("kMGTiB", r) })
}
