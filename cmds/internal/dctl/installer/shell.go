package installer

import (
	"context"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"
)

type shell interface {
	run(ctx context.Context, stdin []byte, c cmd) error
	output(ctx context.Context, args ...string) ([]byte, error)
}

type host struct{ u *ui.UI }

func (h host) run(ctx context.Context, stdin []byte, c cmd) error {
	r := execx.OSRunner{Group: true, Stdin: stdin, Reason: c.Why}
	if !c.Quiet {
		r.UI = h.u
	}
	return r.Run(ctx, "", c.Args[0], c.Args[1:]...)
}

func (host) output(ctx context.Context, args ...string) ([]byte, error) {
	out, err := execx.OSRunner{Group: true}.Output(ctx, "", args[0], args[1:]...)
	return []byte(out), err
}
