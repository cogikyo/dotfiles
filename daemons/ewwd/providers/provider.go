// Package providers implements modular system monitoring and control components
// for the ewwd daemon.
//
// Each provider monitors a specific subsystem (audio, network, weather, etc.)
// and exposes its state through a unified interface. Providers run as background
// goroutines, pushing state updates to subscribers via the notify callback.
//
// Available providers:
//   - Audio: PulseAudio volume monitoring and control via pulsemixer
//   - Brightness: Screen brightness control via wlr-brightness
//   - Date: Date, time, and weeks-alive counter for statusbar
//   - GPU: AMD GPU metrics from /sys/class/drm/
//   - Music: Spotify playback monitoring and control via playerctl
//   - Network: Network speed monitoring via nmcli and /sys/class/net/
//   - Timer: Timer and alarm countdown with desktop notifications
//   - Weather: OpenWeatherMap integration for conditions and forecasts
//
// Providers that support user commands implement the ActionProvider interface.
package providers

import "context"

// Provider defines the base interface for all ewwd providers.
type Provider interface {
	// Name returns the provider identifier used for query and subscribe topics.
	Name() string
	// Start begins the provider's background work. The notify callback should
	// be called whenever the provider's state changes.
	Start(ctx context.Context, notify func(data any)) error
	// Stop gracefully shuts down the provider and releases resources.
	Stop() error
}

// ActionProvider extends Provider with command handling capability.
type ActionProvider interface {
	Provider
	// HandleAction processes commands such as "brightness adjust up 8".
	// It returns a result string and any error encountered.
	HandleAction(args []string) (string, error)
}
