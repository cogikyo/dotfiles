package providers

// base.go defines small shared abstractions used by multiple providers.

// StateSetter is the write-only view of the daemon's shared state store.
type StateSetter interface {
	Set(key string, value any)
}
