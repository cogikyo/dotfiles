// Package providers defines ewwd subsystem monitors and action handlers.
//
// It:
//   - standardizes provider lifecycle and notify callbacks
//   - groups concrete monitors for system and service data
//   - optionally exposes command-style actions for interactive widgets
package providers

// provider.go defines provider interfaces used by ewwd runtime orchestration.
import "context"

// Provider monitors a subsystem and pushes state snapshots via notify.
// Start must block until ctx is done or Stop is called, emitting an initial snapshot first.
type Provider interface {
	Name() string
	Start(ctx context.Context, notify func(data any)) error
	Stop() error
}

// ActionProvider extends Provider with interactive commands (volume adjust, timer start, etc.).
type ActionProvider interface {
	Provider
	HandleAction(args []string) (string, error)
}
