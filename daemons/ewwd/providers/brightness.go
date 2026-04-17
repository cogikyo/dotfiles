package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"dotfiles/daemons/config"
)

// BrightnessState holds the current screen brightness level.
type BrightnessState struct {
	Level int `json:"level"` // 2-10; times 10 for percent display
}

// Brightness drives wlr-brightness via gdbus. Action-only — no polling loop; Start just blocks.
type Brightness struct {
	state  StateSetter
	config config.BrightnessConfig
	notify func(data any)
	done   chan struct{}
	active bool
	level  int
}

func NewBrightness(state StateSetter, cfg config.BrightnessConfig) Provider {
	return &Brightness{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
		level:  cfg.Default,
	}
}

func (b *Brightness) Name() string {
	return "brightness"
}

func (b *Brightness) Start(ctx context.Context, notify func(data any)) error {
	b.active = true
	b.notify = notify

	state := &BrightnessState{Level: b.level}
	b.state.Set("brightness", state)
	notify(state)

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

// HandleAction supports: reset, night, adjust <up|down>. Returns the new level as a string.
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

	state := &BrightnessState{Level: b.level}
	b.state.Set("brightness", state)
	if b.notify != nil {
		b.notify(state)
	}

	return strconv.Itoa(b.level), nil
}

// setBrightness invokes wlr-brightness-control over gdbus, passing a 0.0-1.0 value.
func (b *Brightness) setBrightness(value float64) {
	if err := exec.Command("gdbus", "call", "-e",
		"-d", "de.mherzberg",
		"-o", "/de/mherzberg/wlrbrightness",
		"-m", "de.mherzberg.wlrbrightness.set",
		fmt.Sprintf("%.1f", value),
	).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: brightness error: %v\n", err)
	}
}
