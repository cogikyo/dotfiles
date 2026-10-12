package opencode

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func stopService(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "opencode", "service", "stop").CombinedOutput()
	if err != nil {
		return fmt.Errorf("opencode service stop: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
