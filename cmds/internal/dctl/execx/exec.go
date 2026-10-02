// Package execx wraps external command execution for production code and tests.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Result struct {
	Stdout string
	Stderr string
}

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (*Result, error)
	Output(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type OSRunner struct {
	IO    bool
	Group bool
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

func (r OSRunner) Run(ctx context.Context, dir string, name string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if r.Group {
		cmd = Grouped(ctx, append([]string{name}, args...))
	}
	if dir != "" {
		cmd.Dir = dir
	}
	if r.IO {
		if !r.Group {
			cmd.Stdin = os.Stdin
		}
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		err := Reap(ctx, cmd)
		res := &Result{}
		if err != nil {
			return res, commandErr(name, args, err)
		}
		return res, nil
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := Reap(ctx, cmd)
	res := &Result{Stdout: strings.TrimSpace(stdout.String()), Stderr: strings.TrimSpace(stderr.String())}
	if err != nil {
		return res, commandErr(name, args, err)
	}
	return res, nil
}

func (r OSRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	res, err := OSRunner{}.Run(ctx, dir, name, args...)
	if err != nil && res.Stderr != "" {
		lines := strings.Split(res.Stderr, "\n")
		err = fmt.Errorf("%w: %s", err, strings.Join(lines[max(0, len(lines)-20):], "\n"))
	}
	return res.Stdout, err
}

func commandErr(name string, args []string, err error) error {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return fmt.Errorf("%s %s failed with exit %d", name, strings.Join(args, " "), exitErr.ExitCode())
	}
	return fmt.Errorf("run %s: %w", name, err)
}
