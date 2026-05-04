// Package app carries process-level dependencies into command handlers.
//
// Responsibilities:
// - Bind discovered dotfiles paths, output mode, and global CLI flags.
// - Avoid package-level mutable state in subcommands.
package app

// context.go defines the command context passed through Kong Run methods.

import (
	"context"

	"dotfiles/cmds/internal/dctl/output"
	"dotfiles/cmds/internal/dctl/paths"
)

type Context struct {
	context.Context
	Root     paths.Root
	Output   *output.Printer
	Yes      bool
	Defaults bool
}

func NewContext(ctx context.Context, root paths.Root, out *output.Printer, yes bool, defaults bool) *Context {
	return &Context{Context: ctx, Root: root, Output: out, Yes: yes, Defaults: defaults}
}
