package providers

// base.go defines small shared abstractions used by multiple providers.
import "os"

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
