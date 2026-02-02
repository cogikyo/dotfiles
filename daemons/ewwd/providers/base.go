package providers

// StateSetter defines the interface for updating daemon state.
// All providers use this interface to publish their state changes.
type StateSetter interface {
	// Set stores a value under the given key in the daemon's state store.
	Set(key string, value any)
}
