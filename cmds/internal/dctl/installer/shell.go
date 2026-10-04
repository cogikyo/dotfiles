package installer

import (
	"context"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/ui"
)

type shell interface {
	run(ctx context.Context, stdin []byte, args ...string) error
	output(ctx context.Context, args ...string) ([]byte, error)
}

type host struct{ u *ui.UI }

func (h host) run(ctx context.Context, stdin []byte, args ...string) error {
	h.u.Dim("$ %s", strings.Join(args, " "))
	return execx.OSRunner{Group: true, Stdin: stdin}.Run(ctx, "", args[0], args[1:]...)
}

func (host) output(ctx context.Context, args ...string) ([]byte, error) {
	out, err := execx.OSRunner{Group: true}.Output(ctx, "", args[0], args[1:]...)
	return []byte(out), err
}
