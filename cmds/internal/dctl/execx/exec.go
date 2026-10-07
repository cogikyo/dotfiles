// Package execx wraps external command execution for production code and tests.
package execx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) error
	Output(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type OSRunner struct {
	Group     bool
	Interrupt bool
	Stdin     []byte
	// Frame, when set, draws rules around streamed child output; it returns the closing call.
	Frame func() func()
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

// Run streams both child output streams to stderr so JSON results can use stdout, or to the writer set by Logged.
func (r OSRunner) Run(ctx context.Context, dir string, name string, args ...string) error {
	cmd := r.command(ctx, dir, name, args)
	if w, ok := ctx.Value(logKey{}).(io.Writer); ok {
		cmd.Stdout, cmd.Stderr = w, w
		return failed(cmd.Run(), name, args, "")
	}
	if cmd.Stdin == nil && !r.Group {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if r.Frame != nil {
		defer r.Frame()()
	}
	return failed(r.wait(ctx, cmd), name, args, "")
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
	case r.Group:
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
