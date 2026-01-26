package providers

// ================================================================================
// Common types shared across providers
// ================================================================================

// StateSetter is the unified interface for updating daemon state.
// All providers use this single interface instead of provider-specific ones.
type StateSetter interface {
	Set(key string, value any)
}
