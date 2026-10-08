// Package execx wraps external command execution for production code and tests.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode"

	"dotfiles/cmds/internal/ui"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) error
	Output(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type OSRunner struct {
	Group       bool
	Interrupt   bool
	Stdin       []byte
	UI          *ui.UI
	Reason      string
	Interactive bool
}

func Reason(run Runner, reason string) Runner {
	if r, ok := run.(OSRunner); ok {
		r.Reason = reason
		return r
	}
	return run
}

func Interactive(run Runner) Runner {
	if r, ok := run.(OSRunner); ok {
		r.Interactive = true
		return r
	}
	return run
}

func Grouped(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 30 * time.Second
	return cmd
}

func Reap(ctx context.Context, cmd *exec.Cmd) error {
	err := cmd.Run()
	if ctx.Err() != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return err
}

type logKey struct{}

func Logged(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, logKey{}, w)
}

// Run renders command and result rows through UI, capturing output unless Interactive attaches the terminal.
// Without UI it streams both child output streams to stderr so JSON results can use stdout.
// Under Logged it writes both streams to the log writer and renders nothing.
// An Interactive child inherits the terminal, so it stays in the foreground process group even with Group set.
func (r OSRunner) Run(ctx context.Context, dir string, name string, args ...string) error {
	cmd := r.command(ctx, dir, name, args)
	if w, ok := ctx.Value(logKey{}).(io.Writer); ok {
		cmd.Stdout, cmd.Stderr = w, w
		return failed(cmd.Run(), name, args, "")
	}
	if cmd.Stdin == nil && (!r.Group || r.Interactive) {
		cmd.Stdin = os.Stdin
	}
	if r.UI == nil {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return failed(r.wait(ctx, cmd), name, args, "")
	}
	b := r.UI.Command(shell(name, args), r.Reason)
	if r.Interactive {
		cmd.Stdout, cmd.Stderr = b.Attach()
	} else {
		cmd.Stdout, cmd.Stderr = b, b
	}
	err := r.wait(ctx, cmd)
	shown := err
	if err != nil && ctx.Err() != nil {
		shown = ctx.Err()
	}
	b.End(shown)
	if r.UI.JSON() {
		return failed(err, name, args, "")
	}
	if err != nil {
		return rendered{err}
	}
	return nil
}

type rendered struct{ error }

func (r rendered) Unwrap() error { return r.error }

// Shown identifies a rendered command failure without added wrapper text, so callers can omit a duplicate row.
func Shown(err error) bool {
	r, ok := errors.AsType[rendered](err)
	return ok && r.Error() == err.Error()
}

// Output returns trimmed stdout even on failure; errors wrap the cause and include up to 20 trailing stderr lines.
func (r OSRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := r.command(ctx, dir, name, args)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := failed(r.wait(ctx, cmd), name, args, stderr.String())
	return strings.TrimSpace(stdout.String()), err
}

func (r OSRunner) command(ctx context.Context, dir string, name string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	switch {
	case ctx.Value(logKey{}) != nil:
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT) }
	case r.Interrupt:
		cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGINT) }
	case r.Group && !r.Interactive:
		cmd = Grouped(ctx, append([]string{name}, args...))
	}
	cmd.Dir = dir
	if r.Stdin != nil {
		cmd.Stdin = bytes.NewReader(r.Stdin)
	}
	return cmd
}

func (r OSRunner) wait(ctx context.Context, cmd *exec.Cmd) error {
	if r.Interrupt || ctx.Value(logKey{}) != nil {
		return cmd.Run()
	}
	return Reap(ctx, cmd)
}

func shell(name string, args []string) string {
	words := make([]string, 0, 1+len(args))
	for _, w := range append([]string{name}, args...) {
		if w == "" || strings.ContainsFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("@%+=:,./_-", r)
		}) {
			w = "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

func failed(err error, name string, args []string, stderr string) error {
	if err == nil {
		return nil
	}
	if len(args) > 8 {
		args = append(args[:8:8], "…")
	}
	err = fmt.Errorf("%s: %w", strings.Join(append([]string{name}, args...), " "), err)
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if tail := strings.Join(lines[max(0, len(lines)-20):], "\n"); tail != "" {
		err = fmt.Errorf("%w: %s", err, tail)
	}
	return err
}
