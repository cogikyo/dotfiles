package main

import (
	"fmt"
	"os"

	srccmd "dotfiles/cmds/internal/src"
)

func main() {
	if err := srccmd.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "src: %v\n", err)
		os.Exit(1)
	}
}
