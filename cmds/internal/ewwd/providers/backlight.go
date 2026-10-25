package providers

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	backlightRoot = "/sys/class/backlight"
	backlightOSD  = "backlight-osd"
	osdLinger     = 1500 * time.Millisecond
	backlightStep = 2
)

type BacklightState struct {
	Available bool `json:"available"`
	Percent   int  `json:"percent"`
}

type Backlight struct {
	state  StateSetter
	done   chan struct{}
	active bool

	mu        sync.Mutex
	dir       string
	notify    func(data any)
	last      BacklightState
	published bool
	osdOpen   bool
	osdGen    int
	target    int
	pending   bool
	apply     chan struct{}
}

func NewBacklight(state StateSetter) Provider {
	return &Backlight{state: state, done: make(chan struct{}), apply: make(chan struct{}, 1)}
}

func (b *Backlight) Name() string {
	return "backlight"
}

func (b *Backlight) Start(ctx context.Context, notify func(data any)) error {
	b.active = true
	b.mu.Lock()
	b.notify = notify
	b.dir = findBacklight(backlightRoot)
	b.refresh(false)
	b.mu.Unlock()

	if b.dir == "" {
		select {
		case <-ctx.Done():
		case <-b.done:
		}
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	for _, name := range []string{"brightness", "actual_brightness"} {
		if err := watcher.Add(filepath.Join(b.dir, name)); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-b.done:
			return nil
		case <-b.apply:
			b.applyTarget()
		case <-watcher.Events:
			b.mu.Lock()
			b.refresh(true)
			b.mu.Unlock()
		case err := <-watcher.Errors:
			fmt.Fprintf(os.Stderr, "ewwd: backlight watch: %v\n", err)
		}
	}
}

func (b *Backlight) Stop() error {
	if b.active {
		close(b.done)
		b.active = false
	}
	return nil
}

func (b *Backlight) HandleAction(args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("usage: set <percent>%% | step <up|down>")
	}
	set, delta := -1, 0
	switch args[0] {
	case "set":
		percent, err := strconv.Atoi(strings.TrimSuffix(args[1], "%"))
		if err != nil {
			return "", fmt.Errorf("set takes a percent, got %q", args[1])
		}
		set = percent
	case "step":
		switch args[1] {
		case "up":
			delta = backlightStep
		case "down":
			delta = -backlightStep
		default:
			return "", fmt.Errorf("step takes up or down, got %q", args[1])
		}
	default:
		return "", fmt.Errorf("unknown action %q", args[0])
	}

	b.mu.Lock()
	if b.dir == "" {
		b.mu.Unlock()
		return "", fmt.Errorf("no backlight device")
	}
	if set < 0 {
		set = b.last.Percent
		if b.pending {
			set = b.target
		}
		set += delta
	}
	b.target = min(max(set, 1), 100)
	b.pending = true
	b.mu.Unlock()

	select {
	case b.apply <- struct{}{}:
	default:
	}
	return "ok", nil
}

func (b *Backlight) applyTarget() {
	b.mu.Lock()
	defer b.mu.Unlock()
	target := b.target
	out, err := exec.Command("brightnessctl", "--class=backlight", "set", fmt.Sprintf("%d%%", target)).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: brightnessctl: %v: %s\n", err, strings.TrimSpace(string(out)))
	}
	b.pending = false
	b.refresh(false)
}

func (b *Backlight) refresh(flash bool) {
	state := readBacklight(b.dir)
	if b.published && state == b.last {
		return
	}
	b.last = state
	b.published = true
	b.state.Set("backlight", &state)
	b.notify(&state)
	if flash {
		b.flashOSD()
	}
}

func (b *Backlight) flashOSD() {
	b.osdGen++
	gen := b.osdGen
	if !b.osdOpen {
		out, err := exec.Command("eww", "active-windows").Output()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			return
		}
		if out, err := exec.Command("eww", "--no-daemonize", "open", backlightOSD).CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: open %s: %v: %s\n", backlightOSD, err, strings.TrimSpace(string(out)))
			return
		}
		b.osdOpen = true
	}
	time.AfterFunc(osdLinger, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if gen != b.osdGen || !b.osdOpen {
			return
		}
		exec.Command("eww", "close", backlightOSD).Run()
		b.osdOpen = false
	})
}

func findBacklight(root string) string {
	entries, _ := os.ReadDir(root)
	if len(entries) == 0 {
		return ""
	}
	return filepath.Join(root, entries[0].Name())
}

func readBacklight(dir string) BacklightState {
	if dir == "" {
		return BacklightState{}
	}
	current, err := strconv.Atoi(strings.TrimSpace(readFile(filepath.Join(dir, "actual_brightness"))))
	if err != nil {
		return BacklightState{}
	}
	maximum, err := strconv.Atoi(strings.TrimSpace(readFile(filepath.Join(dir, "max_brightness"))))
	if err != nil || maximum <= 0 {
		return BacklightState{}
	}
	percent := int(math.Round(float64(current) * 100 / float64(maximum)))
	return BacklightState{Available: true, Percent: percent}
}
