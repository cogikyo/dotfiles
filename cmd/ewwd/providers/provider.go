// Package providers defines the provider interface and common types for ewwd's
// modular system monitoring and control components.
package providers

// ================================================================================
// Provider interface and ActionProvider extension for command handling
// ================================================================================

import "context"

// Provider is the base interface for all ewwd providers.
type Provider interface {
	// Name returns the provider identifier (used for query/subscribe topics)
	Name() string
	// Start begins the provider's background work
	Start(ctx context.Context, notify func(data any)) error
	// Stop gracefully shuts down the provider
	Stop() error
}

// ActionProvider extends Provider with command handling.
type ActionProvider interface {
	Provider
	// HandleAction processes commands like "brightness adjust up 8"
	HandleAction(args []string) (string, error)
}
