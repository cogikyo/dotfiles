package providers

// ================================================================================
// Screen brightness control via wlr-brightness
// ================================================================================

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// BrightnessState holds brightness level for eww.
type BrightnessState struct {
	Level int `json:"level"` // 2-10 (multiplied by 10 for percentage)
}

// Brightness provides screen brightness control.
type Brightness struct {
	state  StateSetter
	notify func(data any)
	done   chan struct{}
	active bool
	level  int // Current brightness level (2-10)
}

// NewBrightness creates a Brightness provider.
func NewBrightness(state StateSetter) Provider {
	return &Brightness{
		state: state,
		done:  make(chan struct{}),
		level: 10, // Default to max
	}
}

func (b *Brightness) Name() string {
	return "brightness"
}

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

func (b *Brightness) Stop() error {
	if b.active {
		close(b.done)
		b.active = false
	}
	return nil
}

// HandleAction processes brightness commands.
// Actions: reset, night, adjust <up|down>
func (b *Brightness) HandleAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("action required: reset, night, adjust")
	}

	switch args[0] {
	case "reset":
		b.level = 10
		b.setBrightness(1.0)

	case "night":
		b.level = 4
		b.setBrightness(0.4)

	case "adjust":
		if len(args) < 2 {
			return "", errors.New("adjust requires direction: up or down")
		}
		direction := args[1]
		if direction == "up" && b.level < 10 {
			b.level++
		} else if direction == "down" && b.level > 2 {
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
