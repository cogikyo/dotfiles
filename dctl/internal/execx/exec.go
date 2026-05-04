// Package execx wraps external command execution for production code and tests.
//
// Responsibilities:
// - Capture stdout/stderr for planning and checks.
// - Stream stdio for long-running interactive installs when requested.
// - Expose a narrow runner interface for dry-run and unit-test seams.
package execx

// exec.go defines command runner contracts and the OS-backed implementation.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (*Result, error)
	Output(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type OSRunner struct {
	Env map[string]string
	IO  bool
}

func (r OSRunner) Run(ctx context.Context, dir string, name string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(r.Env) > 0 {
		cmd.Env = mergeEnv(r.Env)
	}
	if r.IO {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		res := &Result{}
		if err != nil {
			return res, commandErr(name, args, err, res)
		}
		return res, nil
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := &Result{Stdout: strings.TrimSpace(stdout.String()), Stderr: strings.TrimSpace(stderr.String())}
	if err != nil {
		return res, commandErr(name, args, err, res)
	}
	return res, nil
}

func (r OSRunner) Output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	res, err := r.Run(ctx, dir, name, args...)
	return res.Stdout, err
}

func commandErr(name string, args []string, err error, res *Result) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return fmt.Errorf("%s %s failed with exit %d", name, strings.Join(args, " "), res.ExitCode)
	}
	return fmt.Errorf("run %s: %w", name, err)
}

func mergeEnv(overrides map[string]string) []string {
	base := map[string]string{}
	for _, item := range os.Environ() {
		if k, v, ok := strings.Cut(item, "="); ok {
			base[k] = v
		}
	}
	for k, v := range overrides {
		if strings.TrimSpace(k) != "" {
			base[k] = v
		}
	}
	out := make([]string, 0, len(base))
	for k, v := range base {
		out = append(out, k+"="+v)
	}
	return out
}

func ResolveDctlBinary() (string, error) {
	bin, err := os.Executable()
	if err == nil && bin != "" {
		return bin, nil
	}
	bin, err = exec.LookPath("dctl")
	if err != nil {
		return "", fmt.Errorf("resolve dctl binary: %w", err)
	}
	return bin, nil
}

func ExecDctl(argv []string) error {
	bin, err := ResolveDctlBinary()
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, argv...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}
