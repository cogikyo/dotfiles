package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// WatchPower polls AC power and converges GLava and the wallpaper on every change, starting with the first poll.
func WatchPower(done <-chan struct{}, s *state.State, lock *Lock) {
	ticker := time.NewTicker(powerPoll)
	defer ticker.Stop()
	first, last := true, false
	for {
		if now := unplugged(); first || now != last {
			first, last = false, now
			SyncGLava(s, lock.Locked())
			cfg := s.GetConfig()
			if err := EnsureBG(&cfg.Background); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd power: background: %v\n", err)
			}
		}
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}

var glavaExecs = [][]string{
	{"glava", "-e", "bars_rc.glsl"},
	{"glava", "-e", "bars_r_rc.glsl"},
	{"glava", "-e", "radial_rc.glsl"},
}

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
	glavaMu.Lock()
	defer glavaMu.Unlock()
	for _, argv := range glavaExecs {
		cmdline := strings.Join(argv, " ")
		running := exec.Command("pgrep", "-fx", cmdline).Run() == nil
		switch {
		case want && !running && !locked:
			cmd := exec.Command(argv[0], argv[1:]...)
			if err := cmd.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "hyprd glava: start %q: %v\n", cmdline, err)
				continue
			}
			go cmd.Wait()
		case !want && running:
			exec.Command("pkill", "-fx", cmdline).Run()
		}
	}
}
