package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dotfiles/daemons/ewwd/config"
)

var (
	audioIDRe   = regexp.MustCompile(`ID:\s*\w+-(\d+)`)
	audioNameRe = regexp.MustCompile(`Name:\s*([A-Za-z0-9-]+)`)
)

// AudioState holds audio volume levels and device names for the eww statusbar.
type AudioState struct {
	Sink       int    `json:"sink"`        // 0-100
	SinkName   string `json:"sink_name"`   // e.g., "pixel buds"
	Source     int    `json:"source"`      // 0-100 (display value with offset)
	SourceName string `json:"source_name"` // e.g., "pixel buds"
}

// Audio provides PulseAudio volume monitoring and control via pulsemixer.
type Audio struct {
	state  StateSetter
	config config.AudioConfig
	notify func(data any)
	done   chan struct{}
	active bool
	last   AudioState
}

// NewAudio creates an Audio provider.
func NewAudio(state StateSetter, cfg config.AudioConfig) Provider {
	return &Audio{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
}

// Name returns the provider identifier.
func (a *Audio) Name() string {
	return "audio"
}

// Start begins polling audio state and notifying on changes.
func (a *Audio) Start(ctx context.Context, notify func(data any)) error {
	a.active = true
	a.notify = notify

	// Send initial state
	a.updateAndNotify()

	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-a.done:
			return nil
		case <-ticker.C:
			a.updateAndNotify()
		}
	}
}

// Stop gracefully shuts down the audio provider.
func (a *Audio) Stop() error {
	if a.active {
		close(a.done)
		a.active = false
	}
	return nil
}

// updateAndNotify polls current audio state and notifies if changed.
func (a *Audio) updateAndNotify() {
	state := a.getCurrentState()

	// Only notify if state changed
	if state != a.last {
		a.last = state
		a.state.Set("audio", &state)
		if a.notify != nil {
			a.notify(&state)
		}
	}
}

// getCurrentState reads current audio levels from pulsemixer.
func (a *Audio) getCurrentState() AudioState {
	return AudioState{
		Sink:       a.getVolume("sink"),
		SinkName:   a.getName("sink"),
		Source:     a.sourceDisplayValue(a.getVolume("source")),
		SourceName: a.getName("source"),
	}
}

// getID returns the default device ID for sink/source.
func (a *Audio) getID(deviceType string) string {
	listArg := "--list-" + deviceType + "s"
	out, err := exec.Command("pulsemixer", listArg).Output()
	if err != nil {
		return ""
	}

	// Find line with "Default" and extract numeric ID
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "Default") {
			if m := audioIDRe.FindStringSubmatch(line); len(m) > 1 {
				return m[1]
			}
		}
	}
	return ""
}

// getVolume returns current volume for the device.
func (a *Audio) getVolume(deviceType string) int {
	id := a.getID(deviceType)
	if id == "" {
		return 0
	}

	deviceID := deviceType + "-" + id
	out, err := exec.Command("pulsemixer", "--get-volume", "--id", deviceID).Output()
	if err != nil {
		return 0
	}

	// Output is "X Y" for left/right channels, take first
	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return 0
	}

	vol, _ := strconv.Atoi(parts[0])
	return vol
}

// getName returns the device name, with custom mappings.
func (a *Audio) getName(deviceType string) string {
	listArg := "--list-" + deviceType + "s"
	out, err := exec.Command("pulsemixer", listArg).Output()
	if err != nil {
		return ""
	}

	// Find line with "Default" and extract Name
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "Default") {
			if m := audioNameRe.FindStringSubmatch(line); len(m) > 1 {
				name := m[1]
				// Custom name mappings from config
				if mapped, ok := a.config.NameMappings[name]; ok {
					return mapped
				}
				return name
			}
		}
	}
	return ""
}

// setVolume sets the volume for a device.
func (a *Audio) setVolume(deviceType string, volume int) {
	id := a.getID(deviceType)
	if id == "" {
		return
	}

	deviceID := deviceType + "-" + id
	if err := exec.Command("pulsemixer", "--id", deviceID, "--set-volume", strconv.Itoa(volume)).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: audio setVolume error: %v\n", err)
	}
}

// sourceDisplayValue converts actual source volume to display value (with offset).
func (a *Audio) sourceDisplayValue(actual int) int {
	if actual <= a.config.SourceOffset {
		return 0
	}
	return actual - a.config.SourceOffset
}

// HandleAction processes audio commands: mute, change_volume, and set_default.
func (a *Audio) HandleAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("action required: mute, change_volume, set_default")
	}

	switch args[0] {
	case "mute":
		if len(args) < 2 {
			return "", errors.New("mute requires device: sink or source")
		}
		a.mute(args[1])

	case "change_volume":
		if len(args) < 3 {
			return "", errors.New("change_volume requires: <sink|source> <up|down>")
		}
		a.changeVolume(args[1], args[2])

	case "set_default":
		if len(args) < 2 {
			return "", errors.New("set_default requires: sink, source, or both")
		}
		a.setDefault(args[1])

	default:
		return "", fmt.Errorf("unknown action: %s", args[0])
	}

	// Update state after action
	a.updateAndNotify()

	state := a.getCurrentState()
	return fmt.Sprintf("sink=%d source=%d", state.Sink, state.Source), nil
}

// mute sets volume to 0.
func (a *Audio) mute(deviceType string) {
	a.setVolume(deviceType, 0)
}

// changeVolume adjusts volume by step, respecting source offset logic.
func (a *Audio) changeVolume(deviceType, direction string) {
	current := a.getVolume(deviceType)
	maxVol := a.config.SinkMax

	if deviceType == "source" {
		maxVol = a.config.SourceMax

		// If source is at or below offset, jump to offset before adjusting
		if current <= a.config.SourceOffset && direction == "up" {
			a.setVolume(deviceType, a.config.SourceOffset)
			current = a.config.SourceOffset
		}
	}

	delta := a.config.VolumeStep
	if direction == "down" {
		delta = -a.config.VolumeStep
	}

	newVol := min(max(current+delta, 0), maxVol)

	// For source, if going below offset, snap to 0
	if deviceType == "source" && newVol < a.config.SourceOffset && direction == "down" {
		newVol = 0
	}

	a.setVolume(deviceType, newVol)
}

// setDefault sets device(s) to preset values.
func (a *Audio) setDefault(target string) {
	if target == "sink" || target == "both" {
		a.setVolume("sink", a.config.DefaultSinkVolume)
	}
	if target == "source" || target == "both" {
		a.setVolume("source", a.config.DefaultSourceVolume)
	}
}
