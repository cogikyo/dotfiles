package providers

// StateSetter allows providers to publish state updates to the daemon's shared store.
type StateSetter interface {
	Set(key string, value any) // Stores a value under the given key
}
