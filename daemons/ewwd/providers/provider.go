// Package providers implements the subsystem monitors feeding ewwd's shared state.
//
// Each provider owns one external data source (pulsemixer, nmcli, sysfs, OpenWeatherMap, etc.)
// and runs as a background goroutine, pushing snapshots via the notify callback.
// Providers with user-driven side effects also implement ActionProvider.
package providers

import "context"

// Provider monitors a subsystem and pushes state snapshots via notify.
//
// Name doubles as the query/subscribe topic string. Start must block until ctx is done or Stop
// is called; it should emit an initial snapshot before entering its poll/event loop.
type Provider interface {
	Name() string
	Start(ctx context.Context, notify func(data any)) error
	Stop() error
}

// ActionProvider accepts interactive commands (volume adjust, timer start, etc.) alongside monitoring.
type ActionProvider interface {
	Provider
	HandleAction(args []string) (string, error)
}
