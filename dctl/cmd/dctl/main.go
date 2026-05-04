// Package main provides the dctl executable entry point.
package main

// main.go wires Kong parsing, root discovery, output mode selection, and command dispatch.

import (
	"context"
	"fmt"
	"os"

	"dotfiles/dctl/internal/app"
	"dotfiles/dctl/internal/cli"
	"dotfiles/dctl/internal/output"
	"dotfiles/dctl/internal/paths"

	"github.com/alecthomas/kong"
)

func main() {
	var rootCmd cli.CLI
	parser, err := kong.New(&rootCmd,
		kong.Name("dctl"),
		kong.Description("dotfiles control plane"),
		kong.UsageOnError(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dctl: %v\n", err)
		os.Exit(1)
	}

	args := cli.NormalizeArgs(os.Args[1:])
	if cli.ShouldLaunchNavigator(args) {
		argv, ok, err := cli.RunNavigator(os.Stdout, cli.BuildCommandCatalog(parser))
		if err != nil {
			fmt.Fprintf(os.Stderr, "dctl: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			return
		}
		if err := cli.ExecSubcommand(argv); err != nil {
			fmt.Fprintf(os.Stderr, "dctl: %v\n", err)
			os.Exit(1)
		}
		return
	}

	kctx, err := parser.Parse(args)
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	root, err := paths.DiscoverRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dctl: %v\n", err)
		os.Exit(1)
	}

	out := output.New(os.Stdout, os.Stderr, output.Options{
		JSON:  rootCmd.JSON,
		Plain: rootCmd.Plain,
	})
	ctx := app.NewContext(context.Background(), root, out, rootCmd.Yes, rootCmd.Defaults)

	if err := kctx.Run(ctx); err != nil {
		out.Error("%v", err)
		os.Exit(1)
	}
}
