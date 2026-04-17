package notify

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// pickSound returns a time-indexed entry — cheap variety without math/rand seeding.
func pickSound(options []string) string {
	if len(options) == 0 {
		return ""
	}
	return options[time.Now().UnixNano()%int64(len(options))]
}

// runDetached starts a command and reaps it in a goroutine so the notifier doesn't block on exit.
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

// tabIcon extracts the middle field from a kitty tab titled "<workspace> <icon> <name>".
// Titles that don't match the 3-field convention are returned unchanged.
func tabIcon(title string) string {
	fields := strings.Fields(title)
	if len(fields) == 3 {
		return fields[1]
	}
	return strings.TrimSpace(title)
}
