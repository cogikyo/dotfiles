package setup

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/machine"
	"dotfiles/cmds/internal/ui"
)

func Machine(u *ui.UI, root string) Stage {
	record := filepath.Join(root, machine.Path)
	return Stage{Name: "machine", Root: true, Items: []Item{{
		Name: "machine-profile",
		Check: func(context.Context) error {
			_, err := machine.Read(record)
			return err
		},
		Fix: func(context.Context) error {
			facts, err := machine.Detect(root)
			if err != nil {
				return err
			}
			p := facts.Profile()
			if old, err := machine.Read(record); err == nil {
				p.WiFi, p.YubiKeys = old.WiFi, old.YubiKeys
			}
			if p, err = AskMachine(u, facts, p); err != nil {
				return err
			}
			if err := machine.Write(record, p); err != nil {
				return err
			}
			u.OK("%s: %s", machine.Path, p)
			return nil
		},
	}}}
}

func AskMachine(u *ui.UI, f machine.Facts, p machine.Profile) (machine.Profile, error) {
	u.Info("detected %s", f)
	if slices.Contains(p.GPUs, machine.NVIDIA) {
		u.Warn("NVIDIA is not supported yet; the install falls back to the in-kernel driver")
	}
	if p.Preset == machine.Framework {
		ok, err := u.Confirm("Apply the Framework Desktop preset? " + p.String())
		if err != nil || ok {
			return p, err
		}
		p.Preset = machine.Custom
	}
	var err error
	if p.CPU == "" {
		if p.CPU, err = choose(u, "CPU vendor", []string{machine.AMD, machine.Intel}, machine.AMD); err != nil {
			return p, err
		}
	}
	if len(p.GPUs) == 0 {
		gpu, err := choose(u, "GPU vendor", []string{machine.AMD, machine.Intel, "none"}, p.CPU)
		if err != nil {
			return p, err
		}
		if gpu != "none" {
			p.GPUs = []string{gpu}
		}
	}
	if len(p.WLAN) > 0 {
		initial := "off"
		if p.WiFi {
			initial = "on"
		}
		wifi, err := choose(u, "Wi-Fi ("+strings.Join(p.WLAN, ", ")+")", []string{"off", "on"}, initial)
		if err != nil {
			return p, err
		}
		p.WiFi = wifi == "on"
	}
	keys, err := choose(u, "YubiKeys", []string{"2", "1", "0"}, strconv.Itoa(p.YubiKeys))
	if err != nil {
		return p, err
	}
	p.YubiKeys, _ = strconv.Atoi(keys)
	return p, p.Validate()
}

func choose(u *ui.UI, title string, options []string, initial string) (string, error) {
	i, err := u.Select(title, options, max(slices.Index(options, initial), 0))
	if err != nil {
		return "", err
	}
	return options[i], nil
}
