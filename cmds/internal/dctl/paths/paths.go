// Package paths discovers the dotfiles root and derives repo-relative paths.
//
// Responsibilities:
// - Honor DOTFILES and XDG_STATE_HOME overrides.
// - Keep command packages independent of hard-coded repo locations.
package paths

// paths.go defines root discovery and helpers for major dotfiles directories.

import (
	"fmt"
	"os"
	"path/filepath"
)

type Root struct {
	Dotfiles string
	Home     string
	State    string
}

func DiscoverRoot() (Root, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Root{}, err
	}
	dotfiles := os.Getenv("DOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(home, "dotfiles")
	}
	dotfiles, err = filepath.Abs(dotfiles)
	if err != nil {
		return Root{}, err
	}
	if _, err := os.Stat(filepath.Join(dotfiles, "AGENTS.md")); err != nil {
		return Root{}, fmt.Errorf("dotfiles root not found at %s", dotfiles)
	}
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		state = filepath.Join(home, ".local", "state")
	}
	return Root{Dotfiles: dotfiles, Home: home, State: filepath.Join(state, "dotfiles")}, nil
}

func (r Root) Etc(parts ...string) string {
	items := append([]string{r.Dotfiles, "etc"}, parts...)
	return filepath.Join(items...)
}

func (r Root) Config(parts ...string) string {
	items := append([]string{r.Dotfiles, "config"}, parts...)
	return filepath.Join(items...)
}

func (r Root) Bin(parts ...string) string {
	items := append([]string{r.Dotfiles, "bin"}, parts...)
	return filepath.Join(items...)
}

func ExpandHome(home string, p string) string {
	if p == "~" {
		return home
	}
	if len(p) >= 2 && p[:2] == "~/" {
		return filepath.Join(home, p[2:])
	}
	return p
}
