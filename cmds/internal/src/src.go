package src

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type app struct {
	stdout io.Writer
	stderr io.Writer
	home   string
	cache  string
}

func Run(args []string, stdout, stderr io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	a := app{
		stdout: stdout,
		stderr: stderr,
		home:   home,
		cache:  cacheRoot(home),
	}

	if len(args) == 0 {
		return errors.New("usage: src find|get|ls|prune")
	}

	switch args[0] {
	case "find":
		return a.find(args[1:])
	case "get":
		return a.get(args[1:])
	case "ls":
		return a.ls(args[1:])
	case "prune":
		return a.prune(args[1:])
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, "usage: src find|get|ls|prune")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func cacheRoot(home string) string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "src")
	}
	return filepath.Join(home, ".cache", "src")
}

func command(dir, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd
}

func outputLine(w io.Writer, s string) error {
	_, err := fmt.Fprintln(w, s)
	return err
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func trimCommandError(err error) string {
	return strings.TrimSpace(strings.TrimPrefix(err.Error(), "exit status 1"))
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
