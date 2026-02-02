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

// Provider monitors a subsystem and pushes state updates via notify callback.
type Provider interface {
	Name() string                                              // Returns provider identifier for query/subscribe topics
	Start(ctx context.Context, notify func(data any)) error   // Starts background monitoring; calls notify on state changes
	Stop() error                                               // Gracefully stops the provider and releases resources
}

// ActionProvider adds command handling for interactive control (e.g., volume adjust, brightness set).
type ActionProvider interface {
	Provider
	HandleAction(args []string) (string, error) // Processes command args, returns result or error
}
