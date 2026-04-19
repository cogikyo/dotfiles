package notify

// helpers.go provides shared helpers for sound selection, detached commands, env parsing, and tab icons.

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func pickSound(options []string) string {
	if len(options) == 0 {
		return ""
	}
	return options[time.Now().UnixNano()%int64(len(options))]
}

func runDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func envInt(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	n, _ := strconv.Atoi(value)
	return n
}

// tabIcon extracts the icon field from a kitty tab titled "<workspace> <icon> <name>".
func tabIcon(title string) string {
	fields := strings.Fields(title)
	if len(fields) == 3 {
		return fields[1]
	}
	return strings.TrimSpace(title)
}
