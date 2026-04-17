package providers

// StateSetter is the daemon's shared store from a provider's point of view (write-only).
type StateSetter interface {
	Set(key string, value any)
}
