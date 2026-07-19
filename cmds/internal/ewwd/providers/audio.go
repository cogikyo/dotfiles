package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"dotfiles/cmds/internal/config"
)

const (
	audioCommandTimeout = 2 * time.Second
	audioDebounce       = 75 * time.Millisecond
	audioActionTimeout  = 750 * time.Millisecond
)

// AudioState is the authoritative WirePlumber snapshot exported to eww.
type AudioState struct {
	SinkAvailable   bool   `json:"sink_available"`
	Sink            int    `json:"sink"`
	SinkName        string `json:"sink_name"`
	SinkMuted       bool   `json:"sink_muted"`
	SourceAvailable bool   `json:"source_available"`
	Source          int    `json:"source"`
	SourceName      string `json:"source_name"`
	SourceMuted     bool   `json:"source_muted"`
}

type audioRequest struct {
	args  []string
	reply chan error
}

type pendingAudioAction struct {
	request audioRequest
	target  string
}

type Audio struct {
	state    StateSetter
	config   config.AudioConfig
	requests chan audioRequest
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
	notify   func(data any)
	last     AudioState
	hasLast  bool
}

func NewAudio(state StateSetter, cfg config.AudioConfig) Provider {
	return &Audio{
		state:    state,
		config:   cfg,
		requests: make(chan audioRequest),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

func (a *Audio) Name() string { return "audio" }

func (a *Audio) Start(ctx context.Context, notify func(data any)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(a.stopped)
	a.notify = notify

	events := make(chan string, 32)
	status := make(chan bool, 8)
	go subscribeAudio(ctx, events, status)

	var debounce *time.Timer
	var debounceC <-chan time.Time
	burstTargets := make(map[string]bool)
	var queue []audioRequest
	var pending *pendingAudioAction
	var actionTimer *time.Timer
	var actionTimerC <-chan time.Time

	startNext := func() {
		for pending == nil && len(queue) > 0 {
			request := queue[0]
			queue = queue[1:]
			if err := a.execute(ctx, request.args); err != nil {
				request.reply <- err
				continue
			}
			pending = &pendingAudioAction{request: request, target: audioActionTarget(request.args)}
			if actionTimer == nil {
				actionTimer = time.NewTimer(audioActionTimeout)
			} else {
				actionTimer.Reset(audioActionTimeout)
			}
			actionTimerC = actionTimer.C
		}
	}
	finishPending := func(err error) {
		if pending == nil {
			return
		}
		pending.request.reply <- err
		pending = nil
		if actionTimer != nil {
			actionTimer.Stop()
		}
		actionTimerC = nil
		startNext()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-a.stop:
			return nil
		case connected := <-status:
			if connected {
				a.refresh(ctx)
			} else {
				a.publish(AudioState{})
				finishPending(errors.New("audio event stream unavailable"))
			}
		case target := <-events:
			burstTargets[target] = true
			if debounce == nil {
				debounce = time.NewTimer(audioDebounce)
			} else {
				if !debounce.Stop() {
					select {
					case <-debounce.C:
					default:
					}
				}
				debounce.Reset(audioDebounce)
			}
			debounceC = debounce.C
		case <-debounceC:
			debounceC = nil
			a.refresh(ctx)
			if pending != nil && audioEventConfirms(pending.target, burstTargets) {
				finishPending(nil)
			}
			clear(burstTargets)
		case <-actionTimerC:
			actionTimerC = nil
			a.refresh(ctx)
			finishPending(nil)
		case req := <-a.requests:
			queue = append(queue, req)
			startNext()
		}
	}
}

func (a *Audio) Stop() error {
	a.stopOnce.Do(func() { close(a.stop) })
	return nil
}

func (a *Audio) refresh(ctx context.Context) {
	sink, sinkOK := a.readDevice(ctx, "sink")
	source, sourceOK := a.readDevice(ctx, "source")
	a.publish(AudioState{
		SinkAvailable:   sinkOK,
		Sink:            sink.volume,
		SinkName:        sink.name,
		SinkMuted:       sink.muted,
		SourceAvailable: sourceOK,
		Source:          source.volume,
		SourceName:      source.name,
		SourceMuted:     source.muted,
	})
}

func (a *Audio) publish(snapshot AudioState) {
	if a.hasLast && snapshot == a.last {
		return
	}
	a.last = snapshot
	a.hasLast = true
	a.state.Set("audio", &snapshot)
	if a.notify != nil {
		a.notify(&snapshot)
	}
}

type audioDevice struct {
	volume int
	name   string
	muted  bool
}

func (a *Audio) readDevice(ctx context.Context, deviceType string) (audioDevice, bool) {
	target := audioTarget(deviceType)
	inspect, err := audioCommand(ctx, "wpctl", "inspect", target)
	if err != nil {
		return audioDevice{}, false
	}
	volume, err := audioCommand(ctx, "wpctl", "get-volume", target)
	if err != nil {
		return audioDevice{}, false
	}

	stableName, displayName, ok := parseAudioIdentity(inspect)
	if !ok {
		return audioDevice{}, false
	}
	percent, muted, ok := parseAudioVolume(volume)
	if !ok {
		return audioDevice{}, false
	}
	if alias, exists := a.config.NameMappings[stableName]; exists {
		displayName = alias
	}
	return audioDevice{volume: percent, name: displayName, muted: muted}, true
}

func parseAudioIdentity(output string) (stableName, displayName string, ok bool) {
	properties := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		properties[key] = value
	}

	stableName = properties["node.name"]
	if stableName == "" {
		return "", "", false
	}
	for _, key := range []string{"node.description", "node.nick", "node.name"} {
		if properties[key] != "" {
			return stableName, properties[key], true
		}
	}
	return "", "", false
}

func parseAudioVolume(output string) (percent int, muted bool, ok bool) {
	fields := strings.Fields(output)
	if len(fields) < 2 || fields[0] != "Volume:" {
		return 0, false, false
	}
	volume, err := strconv.ParseFloat(fields[1], 64)
	if err != nil || volume < 0 || math.IsNaN(volume) || math.IsInf(volume, 0) {
		return 0, false, false
	}
	return int(math.Round(volume * 100)), strings.Contains(output, "[MUTED]"), true
}

func audioCommand(ctx context.Context, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, audioCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(commandCtx, name, args...).Output()
	return string(out), err
}

func subscribeAudio(ctx context.Context, events chan<- string, status chan<- bool) {
	backoff := 250 * time.Millisecond
	for ctx.Err() == nil {
		started := time.Now()
		cmd := exec.CommandContext(ctx, "pactl", "subscribe")
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		if err != nil {
			sendAudioStatus(ctx, status, false)
			if !waitAudioBackoff(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, 5*time.Second)
			continue
		}

		sendAudioStatus(ctx, status, true)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.ToLower(scanner.Text())
			target := ""
			if strings.Contains(line, " on sink ") {
				target = "sink"
			} else if strings.Contains(line, " on source ") {
				target = "source"
			} else if strings.Contains(line, " on server ") {
				target = "server"
			}
			if target != "" {
				select {
				case events <- target:
				default:
				}
			}
		}
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		sendAudioStatus(ctx, status, false)
		if time.Since(started) > 10*time.Second {
			backoff = 250 * time.Millisecond
		}
		if !waitAudioBackoff(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}

func audioActionTarget(args []string) string {
	if len(args) < 2 {
		return ""
	}
	return args[1]
}

func audioEventConfirms(target string, events map[string]bool) bool {
	if events["server"] {
		return true
	}
	if target == "both" {
		return events["sink"] && events["source"]
	}
	return events[target]
}

func sendAudioStatus(ctx context.Context, status chan<- bool, connected bool) {
	select {
	case status <- connected:
	case <-ctx.Done():
	}
}

func waitAudioBackoff(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (a *Audio) HandleAction(args []string) (string, error) {
	reply := make(chan error, 1)
	request := audioRequest{args: args, reply: reply}
	select {
	case a.requests <- request:
	case <-a.stopped:
		return "", errors.New("audio provider is stopped")
	}
	select {
	case err := <-reply:
		if err != nil {
			return "", err
		}
		return "ok", nil
	case <-a.stopped:
		return "", errors.New("audio provider is stopped")
	}
}

func (a *Audio) execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("action required: change_volume, toggle_mute, or reset_volume")
	}

	switch args[0] {
	case "change_volume":
		if len(args) != 3 {
			return errors.New("change_volume requires: <sink|source> <up|down>")
		}
		return a.changeVolume(ctx, args[1], args[2])
	case "toggle_mute":
		if len(args) != 2 {
			return errors.New("toggle_mute requires: sink or source")
		}
		if err := a.requireAvailable(args[1]); err != nil {
			return err
		}
		return a.runAction(ctx, "set-mute", audioTarget(args[1]), "toggle")
	case "reset_volume":
		if len(args) != 2 {
			return errors.New("reset_volume requires: sink, source, or both")
		}
		return a.resetVolume(ctx, args[1])
	default:
		return fmt.Errorf("unknown audio action: %s", args[0])
	}
}

func (a *Audio) changeVolume(ctx context.Context, deviceType, direction string) error {
	if err := a.requireAvailable(deviceType); err != nil {
		return err
	}
	suffix := "+"
	if direction == "down" {
		suffix = "-"
	} else if direction != "up" {
		return fmt.Errorf("invalid volume direction %q", direction)
	}
	ceiling := a.config.SinkMax
	if deviceType == "source" {
		ceiling = a.config.SourceMax
	}
	return a.runAction(ctx, "set-volume", "--limit", formatVolume(ceiling), audioTarget(deviceType), fmt.Sprintf("%d%%%s", a.config.VolumeStep, suffix))
}

func (a *Audio) resetVolume(ctx context.Context, target string) error {
	if target != "sink" && target != "source" && target != "both" {
		return fmt.Errorf("invalid audio target %q", target)
	}
	if target == "both" {
		if err := a.requireAvailable("sink"); err != nil {
			return err
		}
		if err := a.requireAvailable("source"); err != nil {
			return err
		}
	} else if err := a.requireAvailable(target); err != nil {
		return err
	}
	if target == "sink" || target == "both" {
		if err := a.setVolume(ctx, "sink", a.config.DefaultSinkVolume, a.config.SinkMax); err != nil {
			return err
		}
	}
	if target == "source" || target == "both" {
		return a.setVolume(ctx, "source", a.config.DefaultSourceVolume, a.config.SourceMax)
	}
	return nil
}

func (a *Audio) setVolume(ctx context.Context, deviceType string, volume, ceiling int) error {
	return a.runAction(ctx, "set-volume", "--limit", formatVolume(ceiling), audioTarget(deviceType), formatVolume(volume))
}

func (a *Audio) requireAvailable(deviceType string) error {
	if deviceType != "sink" && deviceType != "source" {
		return fmt.Errorf("invalid audio target %q", deviceType)
	}
	available := a.last.SinkAvailable
	if deviceType == "source" {
		available = a.last.SourceAvailable
	}
	if !a.hasLast || !available {
		return fmt.Errorf("default %s is unavailable", deviceType)
	}
	return nil
}

func (a *Audio) runAction(ctx context.Context, args ...string) error {
	commandCtx, cancel := context.WithTimeout(ctx, audioCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(commandCtx, "wpctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wpctl %s: %w%s", args[0], err, commandOutput(out))
	}
	return nil
}

func audioTarget(deviceType string) string {
	if deviceType == "source" {
		return "@DEFAULT_AUDIO_SOURCE@"
	}
	return "@DEFAULT_AUDIO_SINK@"
}

func formatVolume(percent int) string {
	return strconv.FormatFloat(float64(percent)/100, 'f', 2, 64)
}

func commandOutput(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	return ": " + trimmed
}
