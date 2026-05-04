package providers

// timer.go implements the timer/alarm provider and its user-triggered actions.

import (
	"context"
	"dotfiles/cmds/internal/config"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type TimerState struct {
	Timer       string `json:"timer"`        // HH:MM remaining
	Alarm       string `json:"alarm"`        // HH:MM remaining (running) or target time (idle)
	AlarmTarget string `json:"alarm_target"` // HH:MM absolute target
	TimerActive bool   `json:"timer_active"`
	AlarmActive bool   `json:"alarm_active"`
}

// Timer runs two independent minute-granularity countdowns with dunstify notifications.
//
// Action-driven: Start emits one snapshot and blocks; transitions flow through HandleAction.
type Timer struct {
	state  StateSetter
	config config.TimerConfig
	notify func(data any)
	done   chan struct{}
	active bool

	mu sync.Mutex

	timerHours   int
	timerMinutes int
	timerRunning bool
	timerStop    chan struct{}

	alarmTargetHour int
	alarmTargetMin  int
	alarmRunning    bool
	alarmStop       chan struct{}
}

func NewTimer(state StateSetter, cfg config.TimerConfig) Provider {
	t := &Timer{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
	t.resetTimerValues()
	t.resetAlarmValues()
	return t
}

func (t *Timer) Name() string {
	return "timer"
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ lifecycle                                                                    │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (t *Timer) Start(ctx context.Context, notify func(data any)) error {
	t.active = true
	t.notify = notify

	t.updateAndNotify()

	select {
	case <-ctx.Done():
		return nil
	case <-t.done:
		return nil
	}
}

func (t *Timer) Stop() error {
	if t.active {
		t.stopTimerCountdown()
		t.stopAlarmCountdown()
		close(t.done)
		t.active = false
	}
	return nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ state                                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (t *Timer) resetTimerValues() {
	t.timerHours = t.config.DefaultMinutes / 60
	t.timerMinutes = t.config.DefaultMinutes % 60
}

// resetAlarmValues seeds the alarm DefaultAlarmHours ahead, clipping to MinAlarmHours near the hour boundary.
func (t *Timer) resetAlarmValues() {
	now := time.Now()
	minutesUntilNextHour := 60 - now.Minute()
	offset := t.config.DefaultAlarmHours
	if minutesUntilNextHour < 30 {
		offset = t.config.MinAlarmHours
	}

	target := now.Add(time.Duration(offset) * time.Hour)
	t.alarmTargetHour = target.Hour()
	t.alarmTargetMin = 0
}

// getStateLocked builds the current TimerState; caller must hold t.mu.
func (t *Timer) getStateLocked() *TimerState {
	timerStr := fmt.Sprintf("%02d:%02d", t.timerHours, t.timerMinutes)

	var alarmStr string
	if t.alarmRunning {
		remaining := t.alarmRemaining()
		alarmStr = fmt.Sprintf("%02d:%02d", remaining/60, remaining%60)
	} else {
		alarmStr = fmt.Sprintf("%02d:%02d", t.alarmTargetHour, t.alarmTargetMin)
	}

	return &TimerState{
		Timer:       timerStr,
		Alarm:       alarmStr,
		AlarmTarget: fmt.Sprintf("%02d:%02d", t.alarmTargetHour, t.alarmTargetMin),
		TimerActive: t.timerRunning,
		AlarmActive: t.alarmRunning,
	}
}

func (t *Timer) getState() *TimerState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.getStateLocked()
}

// alarmRemaining returns minutes until alarmTarget, rolling to tomorrow if past; caller must hold t.mu.
func (t *Timer) alarmRemaining() int {
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(),
		t.alarmTargetHour, t.alarmTargetMin, 0, 0, now.Location())

	if target.Before(now) {
		target = target.Add(24 * time.Hour)
	}

	return max(0, int(target.Sub(now).Minutes()))
}

func (t *Timer) updateAndNotify() {
	state := t.getState()
	t.state.Set("timer", state)
	if t.notify != nil {
		t.notify(state)
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ actions                                                                      │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// HandleAction accepts "timer|alarm <start|reset|up|down>" and returns the new display value.
func (t *Timer) HandleAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("action required: timer/alarm with start/reset/up/down")
	}

	switch args[0] {
	case "timer":
		return t.handleTimerAction(args[1:])
	case "alarm":
		return t.handleAlarmAction(args[1:])
	default:
		return "", fmt.Errorf("unknown action: %s (use timer or alarm)", args[0])
	}
}

func (t *Timer) handleTimerAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("timer action required: start, reset, up, down")
	}

	t.mu.Lock()

	switch args[0] {
	case "start":
		if !t.timerRunning {
			t.timerRunning = true
			t.timerStop = make(chan struct{})
			go t.timerCountdownLoop()
			t.notifyDunst("attention", " timer started", 3000, "low")
		}

	case "reset":
		t.stopTimerCountdownLocked()
		t.resetTimerValues()
		t.notifyDunst("", " timer reset", 1000, "low")

	case "up", "down":
		if len(args) < 2 {
			t.mu.Unlock()
			return "", errors.New("up/down requires minutes value")
		}
		minutes, err := strconv.Atoi(args[1])
		if err != nil {
			t.mu.Unlock()
			return "", fmt.Errorf("invalid minutes: %s", args[1])
		}
		t.adjustTimer(args[0], minutes)

	default:
		t.mu.Unlock()
		return "", fmt.Errorf("unknown timer action: %s", args[0])
	}

	result := t.getStateLocked().Timer
	t.mu.Unlock()
	t.updateAndNotify()

	return result, nil
}

func (t *Timer) handleAlarmAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("alarm action required: start, reset, up, down")
	}

	t.mu.Lock()

	switch args[0] {
	case "start":
		if !t.alarmRunning {
			remaining := t.alarmRemaining()
			if remaining <= 0 {
				t.mu.Unlock()
				t.resetAlarmValues()
				t.updateAndNotify()
				return "", errors.New("alarm time already passed, resetting")
			}

			t.alarmRunning = true
			t.alarmStop = make(chan struct{})
			go t.alarmCountdownLoop()
			t.notifyDunst("attention", "󰀠 alarm started", 3000, "low")
		}

	case "reset":
		t.stopAlarmCountdownLocked()
		t.resetAlarmValues()
		t.notifyDunst("", "󰹱 alarm reset", 1000, "low")

	case "up", "down":
		if len(args) < 2 {
			t.mu.Unlock()
			return "", errors.New("up/down requires minutes value")
		}
		minutes, err := strconv.Atoi(args[1])
		if err != nil {
			t.mu.Unlock()
			return "", fmt.Errorf("invalid minutes: %s", args[1])
		}
		t.adjustAlarm(args[0], minutes)

	default:
		t.mu.Unlock()
		return "", fmt.Errorf("unknown alarm action: %s", args[0])
	}

	result := t.getStateLocked().Alarm
	t.mu.Unlock()
	t.updateAndNotify()

	return result, nil
}

func (t *Timer) adjustTimer(direction string, minutes int) {
	totalMinutes := t.timerHours*60 + t.timerMinutes

	if direction == "up" {
		totalMinutes += minutes
	} else {
		totalMinutes -= minutes
	}

	if totalMinutes < 0 {
		totalMinutes = 0
	}
	if totalMinutes > 99*60+59 {
		totalMinutes = 99*60 + 59
	}

	t.timerHours = totalMinutes / 60
	t.timerMinutes = totalMinutes % 60
}

func (t *Timer) adjustAlarm(direction string, minutes int) {
	totalMinutes := t.alarmTargetHour*60 + t.alarmTargetMin

	if direction == "up" {
		totalMinutes += minutes
	} else {
		totalMinutes -= minutes
	}

	if totalMinutes < 0 {
		totalMinutes += 24 * 60
	}
	totalMinutes = totalMinutes % (24 * 60)

	t.alarmTargetHour = totalMinutes / 60
	t.alarmTargetMin = totalMinutes % 60
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ countdown loops                                                              │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// countdownLoop fires onTick each minute until it returns true, then calls onComplete.
func (t *Timer) countdownLoop(stopChan chan struct{}, onTick func() bool, onComplete func()) {
	for {
		now := time.Now()
		nextMin := now.Add(time.Minute).Truncate(time.Minute)

		select {
		case <-stopChan:
			return
		case <-time.After(nextMin.Sub(now)):
		}

		if done := onTick(); done {
			onComplete()
			return
		}
		t.updateAndNotify()
	}
}

func (t *Timer) timerCountdownLoop() {
	t.countdownLoop(t.timerStop,
		func() bool {
			t.mu.Lock()
			defer t.mu.Unlock()

			totalMinutes := t.timerHours*60 + t.timerMinutes - 1
			if totalMinutes <= 0 {
				t.timerHours = 0
				t.timerMinutes = 0
				t.timerRunning = false
				return true
			}
			t.timerHours = totalMinutes / 60
			t.timerMinutes = totalMinutes % 60
			return false
		},
		func() {
			t.notifyDunst("timer", " timer done!", 0, "normal")
			t.updateAndNotify()
		},
	)
}

func (t *Timer) alarmCountdownLoop() {
	t.countdownLoop(t.alarmStop,
		func() bool {
			t.mu.Lock()
			defer t.mu.Unlock()

			if t.alarmRemaining() <= 0 {
				t.alarmRunning = false
				return true
			}
			return false
		},
		func() {
			t.notifyDunst("timer", "󰀠 alarm done!", 0, "normal")
			t.resetAlarmValues()
			t.updateAndNotify()
		},
	)
}

func (t *Timer) stopTimerCountdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopTimerCountdownLocked()
}

// stopTimerCountdownLocked halts the timer goroutine; caller must hold t.mu.
func (t *Timer) stopTimerCountdownLocked() {
	if t.timerRunning && t.timerStop != nil {
		close(t.timerStop)
		t.timerRunning = false
		t.timerStop = nil
	}
}

func (t *Timer) stopAlarmCountdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopAlarmCountdownLocked()
}

// stopAlarmCountdownLocked halts the alarm goroutine; caller must hold t.mu.
func (t *Timer) stopAlarmCountdownLocked() {
	if t.alarmRunning && t.alarmStop != nil {
		close(t.alarmStop)
		t.alarmRunning = false
		t.alarmStop = nil
	}
}

// notifyDunst posts a dunstify notification.
func (t *Timer) notifyDunst(appName, message string, timeoutMs int, urgency string) {
	args := []string{}
	if appName != "" {
		args = append(args, "-a", appName)
	}
	if timeoutMs > 0 {
		args = append(args, "-t", strconv.Itoa(timeoutMs))
	}
	if urgency != "" {
		args = append(args, "-u", urgency)
	}
	args = append(args, message)

	if err := exec.Command("dunstify", args...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: dunstify error: %v\n", err)
	}
}
