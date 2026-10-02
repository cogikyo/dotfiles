package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/ui"
)

type shell interface {
	run(ctx context.Context, stdin []byte, args ...string) error
	output(ctx context.Context, args ...string) ([]byte, error)
}

type host struct{ u *ui.UI }

func (h host) run(ctx context.Context, stdin []byte, args ...string) error {
	h.u.Dim("$ %s", strings.Join(args, " "))
	cmd := execx.Grouped(ctx, args)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := execx.Reap(ctx, cmd); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (host) output(ctx context.Context, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := execx.Grouped(ctx, args)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := execx.Reap(ctx, cmd); err != nil {
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
