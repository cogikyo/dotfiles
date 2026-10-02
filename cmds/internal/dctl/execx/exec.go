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
	IO bool
}

func (r OSRunner) Run(ctx context.Context, dir string, name string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if r.IO {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		res := &Result{}
		if err != nil {
			return res, commandErr(name, args, err)
		}
		return res, nil
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := &Result{Stdout: strings.TrimSpace(stdout.String()), Stderr: strings.TrimSpace(stderr.String())}
	if err != nil {
		return res, commandErr(name, args, err)
	}
	return res, nil
}

func (r OSRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	res, err := OSRunner{}.Run(ctx, dir, name, args...)
	return res.Stdout, err
}

func commandErr(name string, args []string, err error) error {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return fmt.Errorf("%s %s failed with exit %d", name, strings.Join(args, " "), exitErr.ExitCode())
	}
	return fmt.Errorf("run %s: %w", name, err)
}
