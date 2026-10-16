package execx

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const RemoteTimeout = 15 * time.Minute

var SSHOptions = []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2"}

var (
	ErrRejected    = errors.New("the remote rejected the SSH key; load it with `ssh-add`, or run `dctl setup ssh`")
	ErrUnreachable = errors.New("cannot reach the remote; check the network")
	ErrStalled     = errors.New("the remote gave no answer")
)

func Remote(run Runner) Runner {
	if r, ok := run.(OSRunner); ok {
		r.Remote, r.Group = true, true
		return r
	}
	return run
}

func remoteEnv() []string {
	return []string{"GIT_SSH_COMMAND=ssh " + strings.Join(SSHOptions, " "), "GIT_TERMINAL_PROMPT=0"}
}

func remote(ctx context.Context, err error, stderr string, waited time.Duration) error {
	has := func(subs ...string) bool {
		return slices.ContainsFunc(subs, func(s string) bool { return strings.Contains(stderr, s) })
	}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf("%w after %s; ssh may be waiting on the SSH agent for a passphrase prompt or a YubiKey touch: %w", ErrStalled, waited.Round(time.Second), err)
	case has("Permission denied (publickey"):
		return fmt.Errorf("%w: %w", ErrRejected, err)
	case has("ssh: connect to host", "ssh: Could not resolve hostname", "Timeout, server", "kex_exchange_identification", "banner exchange", "Connection closed by", "Connection reset by"):
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	return err
}
