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
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	args = append([]string{"--user", "--collect", "--quiet", "--expand-environment=no", "--", "sh", "-c", script, name}, args...)
	if out, err := exec.CommandContext(ctx, "systemd-run", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("systemd-run: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
