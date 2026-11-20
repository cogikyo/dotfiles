package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

const glavaFeed = "glava-feed"

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
	feed := sync.OnceValue(hasGLavaFeed)
	glavaMu.Lock()
	defer glavaMu.Unlock()
	for _, rc := range glavaRCs {
		match := "^glava -e " + regexp.QuoteMeta(rc) + "( |$)"
		running := exec.Command("pgrep", "-f", match).Run() == nil
		switch {
		case want && !running && !locked:
			cmd := exec.Command("glava", "-e", rc)
			// PipeWire can silently route GLava to the default mic if the named source is missing.
			if feed() {
				cmd.Args = append(cmd.Args, "-r", fmt.Sprintf("setsource %q", glavaFeed))
			}
			if err := detach.Start(cmd); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd glava: start %q: %v\n", strings.Join(cmd.Args, " "), err)
			}
		case !want && running:
			exec.Command("pkill", "-f", match).Run()
		}
	}
}

func hasGLavaFeed() bool {
	out, err := exec.Command("pactl", "list", "short", "sources").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd glava: sources: %v\n", err)
		return false
	}
	for line := range strings.Lines(string(out)) {
		if f := strings.Fields(line); len(f) > 1 && f[1] == glavaFeed {
			return true
		}
	}
	return false
}
