package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"dotfiles/daemons/ewwd/config"
)

// BrightnessState holds the current screen brightness level.
type BrightnessState struct {
	Level int `json:"level"` // 2-10 (multiplied by 10 for percentage)
}

// Brightness provides screen brightness control via wlr-brightness.
type Brightness struct {
	state  StateSetter
	config config.BrightnessConfig
	notify func(data any)
	done   chan struct{}
	active bool
	level  int // Current brightness level (2-10)
}

// NewBrightness creates a Brightness provider.
func NewBrightness(state StateSetter, cfg config.BrightnessConfig) Provider {
	return &Brightness{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
		level:  cfg.Default,
	}
}

// Name returns the provider identifier.
func (b *Brightness) Name() string {
	return "brightness"
}

// Start initializes the brightness provider and sends the initial state.
func (b *Brightness) Start(ctx context.Context, notify func(data any)) error {
	b.active = true
	b.notify = notify

	// Send initial state
	state := &BrightnessState{Level: b.level}
	b.state.Set("brightness", state)
	notify(state)

	// Brightness is action-driven, just wait for shutdown
	select {
	case <-ctx.Done():
		return nil
	case <-b.done:
		return nil
	}
}

// Stop gracefully shuts down the brightness provider.
func (b *Brightness) Stop() error {
	if b.active {
		close(b.done)
		b.active = false
	}
	return nil
}

// HandleAction processes brightness commands: reset, night, and adjust.
func (b *Brightness) HandleAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("action required: reset, night, adjust")
	}

	switch args[0] {
	case "reset":
		b.level = b.config.Max
		b.setBrightness(float64(b.config.Max) / 10.0)

	case "night":
		b.level = b.config.Night
		b.setBrightness(float64(b.config.Night) / 10.0)

	case "adjust":
		if len(args) < 2 {
			return "", errors.New("adjust requires direction: up or down")
		}
		direction := args[1]
		if direction == "up" && b.level < b.config.Max {
			b.level++
		} else if direction == "down" && b.level > b.config.Min {
			b.level--
		}
		b.setBrightness(float64(b.level) / 10.0)

	default:
		return "", fmt.Errorf("unknown action: %s", args[0])
	}

	// Update state and notify subscribers
	state := &BrightnessState{Level: b.level}
	b.state.Set("brightness", state)
	if b.notify != nil {
		b.notify(state)
	}

	return strconv.Itoa(b.level), nil
}

// setBrightness calls gdbus to set actual screen brightness.
func (b *Brightness) setBrightness(value float64) {
	// wlr-brightness-control via gdbus
	if err := exec.Command("gdbus", "call", "-e",
		"-d", "de.mherzberg",
		"-o", "/de/mherzberg/wlrbrightness",
		"-m", "de.mherzberg.wlrbrightness.set",
		fmt.Sprintf("%.1f", value),
	).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: brightness error: %v\n", err)
	}
}
