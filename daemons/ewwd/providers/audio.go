package providers

// audio.go monitors PulseAudio state and handles volume/mute actions via pulsemixer.
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

	"dotfiles/daemons/config"
)

var (
	audioIDRe   = regexp.MustCompile(`ID:\s*\w+-(\d+)`)
	audioNameRe = regexp.MustCompile(`Name:\s*([A-Za-z0-9-]+)`)
)

// AudioState is the PulseAudio snapshot exported to the statusbar.
type AudioState struct {
	Sink          int    `json:"sink"`
	SinkName      string `json:"sink_name"`
	SinkBluetooth bool   `json:"sink_bluetooth"`
	Source        int    `json:"source"` // post-SourceOffset display value
	SourceName    string `json:"source_name"`
}

type Audio struct {
	state  StateSetter
	config config.AudioConfig
	notify func(data any)
	done   chan struct{}
	active bool
	last   AudioState
}

func NewAudio(state StateSetter, cfg config.AudioConfig) Provider {
	return &Audio{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
}

func (a *Audio) Name() string {
	return "audio"
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ lifecycle                                                                    │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (a *Audio) Start(ctx context.Context, notify func(data any)) error {
	a.active = true
	a.notify = notify

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

func (a *Audio) Stop() error {
	if a.active {
		close(a.done)
		a.active = false
	}
	return nil
}

// updateAndNotify publishes a snapshot only when it differs from the last.
func (a *Audio) updateAndNotify() {
	state := a.getCurrentState()

	if state != a.last {
		a.last = state
		a.state.Set("audio", &state)
		if a.notify != nil {
			a.notify(&state)
		}
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ pulsemixer queries                                                           │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (a *Audio) getCurrentState() AudioState {
	sinkName := a.getName("sink")

	return AudioState{
		Sink:          a.getVolume("sink"),
		SinkName:      sinkName,
		SinkBluetooth: a.isBluetoothDevice(sinkName),
		Source:        a.sourceDisplayValue(a.getVolume("source")),
		SourceName:    a.getName("source"),
	}
}

// getID parses the numeric ID of the default sink/source from pulsemixer --list output.
func (a *Audio) getID(deviceType string) string {
	listArg := "--list-" + deviceType + "s"
	out, err := exec.Command("pulsemixer", listArg).Output()
	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "Default") {
			if m := audioIDRe.FindStringSubmatch(line); len(m) > 1 {
				return m[1]
			}
		}
	}
	return ""
}

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

	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return 0
	}

	vol, _ := strconv.Atoi(parts[0])
	return vol
}

// getName returns the default device name with NameMappings aliases applied.
func (a *Audio) getName(deviceType string) string {
	listArg := "--list-" + deviceType + "s"
	out, err := exec.Command("pulsemixer", listArg).Output()
	if err != nil {
		return ""
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(line, "Default") {
			if m := audioNameRe.FindStringSubmatch(line); len(m) > 1 {
				name := m[1]
				if mapped, ok := a.config.NameMappings[name]; ok {
					return mapped
				}
				return name
			}
		}
	}
	return ""
}

func (a *Audio) isBluetoothDevice(name string) bool {
	for _, candidate := range a.config.BluetoothNames {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}

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

// sourceDisplayValue collapses volumes at or below SourceOffset to zero (dead zone).
func (a *Audio) sourceDisplayValue(actual int) int {
	if actual <= a.config.SourceOffset {
		return 0
	}
	return actual - a.config.SourceOffset
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ actions                                                                      │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// HandleAction supports: mute, change_volume, set_default.
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

	a.updateAndNotify()

	state := a.getCurrentState()
	return fmt.Sprintf("sink=%d source=%d", state.Sink, state.Source), nil
}

func (a *Audio) mute(deviceType string) {
	a.setVolume(deviceType, 0)
}

// changeVolume steps by VolumeStep, snapping source volume through the dead zone.
func (a *Audio) changeVolume(deviceType, direction string) {
	current := a.getVolume(deviceType)
	maxVol := a.config.SinkMax

	if deviceType == "source" {
		maxVol = a.config.SourceMax

		// Seed at SourceOffset so the first step up from the dead zone is audible.
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

	// Snap straight to 0 when stepping down through SourceOffset.
	if deviceType == "source" && newVol < a.config.SourceOffset && direction == "down" {
		newVol = 0
	}

	a.setVolume(deviceType, newVol)
}

func (a *Audio) setDefault(target string) {
	if target == "sink" || target == "both" {
		a.setVolume("sink", a.config.DefaultSinkVolume)
	}
	if target == "source" || target == "both" {
		a.setVolume("source", a.config.DefaultSourceVolume)
	}
}
