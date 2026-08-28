package opencode

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func restartService(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "--user", "restart", "opencode.service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
