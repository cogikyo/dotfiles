package hardware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
)

func Group(sys string, run execx.Runner) doctor.Group {
	return doctor.Group{Name: "hardware", Checks: []doctor.Check{
		{Name: "hardware-no-wlan", Check: func(context.Context) error { return noWLAN(sys) }},
		{Name: "hardware-bluetooth", Check: func(context.Context) error { return bluetooth(sys) }},
		{Name: "hardware-firmware", Check: func(ctx context.Context) error {
			res, err := run.Run(ctx, "", "journalctl", "-k", "-b", "--no-pager", "-o", "cat")
			if err != nil || strings.Contains(res.Stderr, "insufficient permissions") {
				return doctor.Block("cannot read the kernel log: %v %s", err, res.Stderr)
			}
			if failed := firmwareFailures(res.Stdout); len(failed) > 0 {
				return fmt.Errorf("%d firmware load failures: %s", len(failed), strings.Join(failed, "; "))
			}
			return nil
		}},
	}}
}

func noWLAN(sys string) error {
	dir := filepath.Join(sys, "class", "net")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return doctor.Block("cannot list %s: %v", dir, err)
	}
	var found []string
	for _, e := range entries {
		iface := filepath.Join(dir, e.Name())
		if !exists(filepath.Join(iface, "wireless")) && !exists(filepath.Join(iface, "phy80211")) {
			continue
		}
		driver, err := os.Readlink(filepath.Join(iface, "device", "driver"))
		if err != nil {
			driver = "unknown"
		}
		found = append(found, fmt.Sprintf("%s (driver %s)", e.Name(), filepath.Base(driver)))
	}
	if len(found) > 0 {
		return fmt.Errorf("wlan present: %s; blacklist the driver in system/etc/modprobe.d", strings.Join(found, ", "))
	}
	return nil
}

func bluetooth(sys string) error {
	entries, _ := os.ReadDir(filepath.Join(sys, "class", "bluetooth"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "hci") && !strings.Contains(e.Name(), ":") {
			return nil
		}
	}
	return fmt.Errorf("no Bluetooth controller in %s", filepath.Join(sys, "class", "bluetooth"))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func firmwareFailures(log string) []string {
	var out []string
	for line := range strings.Lines(log) {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "firmware") && strings.Contains(lower, "fail") {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}
