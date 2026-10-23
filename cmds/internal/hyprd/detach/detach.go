package detach

import (
	"os"
	"os/exec"
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
