package notify

import (
	"os"
	"strconv"
	"strings"
)

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
