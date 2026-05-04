package notify

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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
