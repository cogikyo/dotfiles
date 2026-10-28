package detach

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Start runs cmd under setsid -f so the child is never hyprd's to reap.
// Waiting in a goroutine leaks zombies, since a rebuild re-execs hyprd.
func Start(cmd *exec.Cmd) error {
	if cmd.Err != nil {
		return cmd.Err
	}
	setsid := exec.Command("setsid", append([]string{"-f"}, cmd.Args...)...)
	setsid.Env, setsid.Dir, setsid.Stderr = cmd.Env, cmd.Dir, os.Stderr
	return setsid.Run()
}

// Unit runs sh -c script in a transient user unit that outlives hyprd.service.
// Stopping the service kills its whole cgroup, setsid -f children included.
// Args carry text any process controls, so systemd must not expand $ in them.
func Unit(ctx context.Context, name, script string, args ...string) error {
	return run(ctx, nil, name, script, args)
}

func NamedUnit(ctx context.Context, unit, script string, args ...string) error {
	return run(ctx, []string{"--unit=" + unit}, unit, script, args)
}

func Stop(ctx context.Context, patterns ...string) error {
	return systemd(ctx, "systemctl", append([]string{"--user", "stop", "--"}, patterns...)...)
}

func run(ctx context.Context, flags []string, name, script string, args []string) error {
	argv := append([]string{"--user", "--collect", "--quiet", "--expand-environment=no"}, flags...)
	argv = append(append(argv, "--", "sh", "-c", script, name), args...)
	return systemd(ctx, "systemd-run", argv...)
}

func systemd(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
