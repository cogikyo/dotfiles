package session

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"dotfiles/cmds/internal/hyprd/detach"
	"dotfiles/cmds/internal/hyprd/state"
)

const (
	powerSupplyRoot = "/sys/class/power_supply"
	powerPoll       = 2 * time.Second
)

// unplugged reports whether the machine runs on its own battery: a system battery is discharging and no Mains supply is online.
// USB supplies are ignored because a USB-C peripheral can show online at 5 V and 100 mA while the battery discharges.
// Machines without a battery are never unplugged.
func unplugged() bool {
	discharging, mains := false, false
	supplies, _ := filepath.Glob(filepath.Join(powerSupplyRoot, "*"))
	for _, supply := range supplies {
		read := func(name string) string {
			data, _ := os.ReadFile(filepath.Join(supply, name))
			return strings.TrimSpace(string(data))
		}
		switch read("type") {
		case "Battery":
			discharging = discharging || (read("scope") != "Device" && read("status") == "Discharging")
		case "Mains":
			mains = mains || read("online") == "1"
		}
	}
	return discharging && !mains
}

// WatchPower polls power and converges GLava when AC power changes and the wallpaper when holdVideo changes, starting with the first poll.
func WatchPower(done <-chan struct{}, s *state.State, lock *Lock) {
	ticker := time.NewTicker(powerPoll)
	defer ticker.Stop()
	first := true
	var battery, held bool
	for {
		if now := unplugged(); first || now != battery {
			battery = now
			SyncGLava(s, lock.Locked())
		}
		if now := holdVideo(); first || now != held {
			held = now
			cfg := s.GetConfig()
			if err := EnsureBG(&cfg.Background); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd power: background: %v\n", err)
			}
		}
		first = false
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}

var glavaRCs = []string{"bars_rc.glsl", "bars_r_rc.glsl", "radial_rc.glsl"}

var glavaMu sync.Mutex

// SyncGLava starts or stops the desktop visualizers to match the display profile, share mode, and lock.
// The lock screen keeps them only on AC power and never starts them, since a late window would miss the lock tag.
// The lock client's own GLava bar is not touched.
// Nothing changes until a display profile has been resolved, since the fallback may not be this screen's.
func SyncGLava(s *state.State, locked bool) {
	if s.GetDisplay().Monitor == "" {
		return
	}
	want := s.Profile().GLava && !s.GetScreenShare() && !(locked && unplugged())
	source := sync.OnceValue(glavaSource)
	glavaMu.Lock()
	defer glavaMu.Unlock()
	for _, rc := range glavaRCs {
		match := "^glava -e " + regexp.QuoteMeta(rc) + "( |$)"
		running := exec.Command("pgrep", "-f", match).Run() == nil
		switch {
		case want && !running && !locked:
			cmd := exec.Command("glava", "-e", rc)
			if src := source(); src != "" {
				cmd.Args = append(cmd.Args, "-r", fmt.Sprintf("setsource %q", src))
			}
			if err := detach.Start(cmd); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd glava: start %q: %v\n", strings.Join(cmd.Args, " "), err)
			}
		case !want && running:
			exec.Command("pkill", "-f", match).Run()
		}
	}
}

func glavaSource() string {
	source, err := snapclientMonitor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd glava: source: %v\n", err)
	}
	return source
}

type pactlSink struct {
	Index      int               `json:"index"`
	Name       string            `json:"name"`
	Monitor    string            `json:"monitor_source"`
	Properties map[string]string `json:"properties"`
}

type pactlSinkInput struct {
	Sink       int               `json:"sink"`
	Properties map[string]string `json:"properties"`
}

// snapclientMonitor selects audio after Snapserver's playback buffer.
// GLava's auto source reads the Snapcast default sink before that buffer.
func snapclientMonitor() (string, error) {
	out, err := exec.Command("pactl", "get-default-sink").Output()
	if err != nil {
		return "", fmt.Errorf("default sink: %w", err)
	}
	var sinks []pactlSink
	if err := pactlList("sinks", &sinks); err != nil {
		return "", err
	}
	def := strings.TrimSpace(string(out))
	if !slices.ContainsFunc(sinks, func(sink pactlSink) bool {
		return sink.Name == def && sink.Properties["application.name"] == "Snapcast"
	}) {
		return "", nil
	}
	var inputs []pactlSinkInput
	if err := pactlList("sink-inputs", &inputs); err != nil {
		return "", err
	}
	i := slices.IndexFunc(inputs, func(input pactlSinkInput) bool {
		return input.Properties["application.name"] == "Snapclient"
	})
	if i < 0 {
		return "", nil
	}
	j := slices.IndexFunc(sinks, func(sink pactlSink) bool { return sink.Index == inputs[i].Sink })
	if j < 0 {
		return "", fmt.Errorf("snapclient sink %d not listed", inputs[i].Sink)
	}
	return sinks[j].Monitor, nil
}

func pactlList(kind string, v any) error {
	out, err := exec.Command("pactl", "-f", "json", "list", kind).Output()
	if err != nil {
		return fmt.Errorf("list %s: %w", kind, err)
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("list %s: %w", kind, err)
	}
	return nil
}
