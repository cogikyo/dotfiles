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
	Remote      bool
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
	ctx, cancel := r.limit(ctx)
	defer cancel()
	cmd := r.command(ctx, dir, name, args)
	if w, ok := ctx.Value(logKey{}).(io.Writer); ok {
		cmd.Stdout, cmd.Stderr = w, w
		return failed(ctx, r.wait(ctx, cmd), name, args, "")
	}
	if cmd.Stdin == nil && (!r.Group || r.Interactive) {
		cmd.Stdin = os.Stdin
	}
	if r.UI == nil {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return failed(ctx, r.wait(ctx, cmd), name, args, "")
	}
	b := r.UI.Command(shell(name, args), r.Reason)
	if r.Interactive {
		cmd.Stdout, cmd.Stderr = b.Attach()
	} else {
		cmd.Stdout, cmd.Stderr = b, b
	}
	err := r.wait(ctx, cmd)
	if r.UI.JSON() {
		return failed(ctx, err, name, args, "")
	}
	if err != nil && errors.Is(ctx.Err(), context.Canceled) {
		err = interrupted{err}
	}
	b.End(err)
	if err != nil {
		return rendered{err}
	}
	return nil
}

type interrupted struct{ cause error }

func (interrupted) Error() string { return "interrupted" }

func (i interrupted) Unwrap() []error { return []error{context.Canceled, i.cause} }

type rendered struct{ error }

func (r rendered) Unwrap() error { return r.error }

// Shown identifies a rendered command failure without added wrapper text, so callers can omit a duplicate row.
func Shown(err error) bool {
	r, ok := errors.AsType[rendered](err)
	return ok && r.Error() == err.Error()
}

// Output returns trimmed stdout even on failure; errors wrap the cause and include up to 20 trailing stderr lines.
func (r OSRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	ctx, cancel := r.limit(ctx)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := r.command(ctx, dir, name, args)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := failed(ctx, r.wait(ctx, cmd), name, args, stderr.String())
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
	if r.Remote {
		cmd.Env = append(os.Environ(), remoteEnv()...)
	}
	if r.Stdin != nil {
		cmd.Stdin = bytes.NewReader(r.Stdin)
	}
	return cmd
}

func (r OSRunner) limit(ctx context.Context) (context.Context, context.CancelFunc) {
	if !r.Remote {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, RemoteTimeout)
}

func (r OSRunner) wait(ctx context.Context, cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	if r.Remote {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, &stderr)
	}
	start := time.Now()
	var err error
	if r.Interrupt || ctx.Value(logKey{}) != nil {
		err = cmd.Run()
	} else {
		err = Reap(ctx, cmd)
	}
	if r.Remote && err != nil {
		return remote(ctx, err, stderr.String(), time.Since(start))
	}
	return err
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

func failed(ctx context.Context, err error, name string, args []string, stderr string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(ctx.Err(), context.Canceled):
		return interrupted{err}
	}
	argv := strings.Join(append([]string{name}, args...), " ")
	if len(argv) > 40 {
		argv = argv[:max(len(name), strings.LastIndexByte(argv[:40], ' '))] + " …"
	}
	err = fmt.Errorf("%w (%s)", err, argv)
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if tail := strings.Join(lines[max(0, len(lines)-20):], "\n"); tail != "" {
		err = fmt.Errorf("%w: %s", err, tail)
	}
	return err
}
