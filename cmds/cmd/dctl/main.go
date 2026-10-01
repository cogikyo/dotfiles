package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"dotfiles/cmds/internal/dctl/cli"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"

	"github.com/alecthomas/kong"
)

func main() {
	var root cli.CLI
	parser, err := kong.New(&root,
		kong.Name("dctl"),
		kong.Description("dotfiles control plane"),
		kong.UsageOnError(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dctl: %v\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"--help"}
	}
	kctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	u := ui.New(ui.Options{JSON: root.JSON, Plain: root.Plain, Yes: root.Yes})
	dotfiles, err := paths.DiscoverRoot()
	if err != nil {
		u.Error("%v", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	kctx.BindTo(ctx, (*context.Context)(nil))
	err = kctx.Run(u, dotfiles)
	stop()
	switch {
	case errors.Is(err, context.Canceled):
		u.Error("interrupted")
		os.Exit(130)
	case err != nil:
		u.Error("%v", err)
		os.Exit(1)
	}
}
