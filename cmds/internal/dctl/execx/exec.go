// Package execx wraps external command execution for production code and tests.
package execx

import (
	"bytes"
	"context"
	"fmt"
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
	Group bool
	Stdin []byte
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

// Run streams both child output streams to stderr so JSON results can use stdout.
func (r OSRunner) Run(ctx context.Context, dir string, name string, args ...string) error {
	cmd := r.command(ctx, dir, name, args)
	if cmd.Stdin == nil && !r.Group {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return failed(Reap(ctx, cmd), name, args, "")
}

// Output returns trimmed stdout even on failure; errors wrap the cause and include up to 20 trailing stderr lines.
func (r OSRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := r.command(ctx, dir, name, args)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := failed(Reap(ctx, cmd), name, args, stderr.String())
	return strings.TrimSpace(stdout.String()), err
}

func (r OSRunner) command(ctx context.Context, dir string, name string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.Group {
		cmd = Grouped(ctx, append([]string{name}, args...))
	}
	cmd.Dir = dir
	if r.Stdin != nil {
		cmd.Stdin = bytes.NewReader(r.Stdin)
	}
	return cmd
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
