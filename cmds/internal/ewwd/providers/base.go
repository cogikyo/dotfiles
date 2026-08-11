package providers

// base.go defines small shared abstractions used by multiple providers.
import (
	"context"
	"os"
	"time"
)

// StateSetter is the write-only view of the daemon's shared state store.
type StateSetter interface {
	Set(key string, value any)
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func waitForNextBoundary(ctx context.Context, done <-chan struct{}, interval time.Duration) bool {
	now := time.Now()
	timer := time.NewTimer(now.Truncate(interval).Add(interval).Sub(now))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-done:
		return false
	case <-timer.C:
		return true
	}
}
