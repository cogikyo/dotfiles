package providers

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const powerSupplyRoot = "/sys/class/power_supply"

var (
	batteryIcons  = []string{"󰂎", "󰁺", "󰁻", "󰁼", "󰁽", "󰁾", "󰁿", "󰂀", "󰂁", "󰂂", "󰁹"}
	chargingIcons = []string{"󰢟", "󰢜", "󰂆", "󰂇", "󰂈", "󰢝", "󰂉", "󰢞", "󰂊", "󰂋", "󰂅"}
)

type BatteryState struct {
	Available bool   `json:"available"`
	Percent   int    `json:"percent"`
	Plugged   bool   `json:"plugged"`
	Status    string `json:"status"`
	Icon      string `json:"icon"`
	Remaining string `json:"remaining"`
}

type Battery struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	last      BatteryState
	published bool
}

func NewBattery(state StateSetter) Provider {
	return &Battery{state: state, done: make(chan struct{})}
}

func (b *Battery) Name() string {
	return "battery"
}

func (b *Battery) Start(ctx context.Context, notify func(data any)) error {
	b.active = true
	b.publish(notify)

	for waitForNextBoundary(ctx, b.done, 2*time.Second) {
		b.publish(notify)
	}
	return nil
}

func (b *Battery) Stop() error {
	if b.active {
		close(b.done)
		b.active = false
	}
	return nil
}

func (b *Battery) publish(notify func(data any)) {
	state := readBattery(powerSupplyRoot)
	if b.published && b.last == state {
		return
	}
	b.last = state
	b.published = true
	b.state.Set("battery", &state)
	notify(&state)
}

func readBattery(root string) BatteryState {
	paths, _ := filepath.Glob(filepath.Join(root, "*", "uevent"))
	var battery map[string]string
	mains := false
	for _, path := range paths {
		props := readUevent(path)
		switch props["TYPE"] {
		case "Battery":
			if battery == nil && props["SCOPE"] != "Device" && props["PRESENT"] != "0" {
				battery = props
			}
		case "Mains":
			mains = mains || props["ONLINE"] == "1"
		}
	}
	if battery == nil {
		return BatteryState{}
	}

	percent, err := strconv.Atoi(battery["CAPACITY"])
	if err != nil {
		return BatteryState{}
	}
	percent = min(max(percent, 0), 100)
	status := battery["STATUS"]
	plugged := mains || status == "Charging"

	icons := batteryIcons
	if plugged {
		icons = chargingIcons
	}
	return BatteryState{
		Available: true,
		Percent:   percent,
		Plugged:   plugged,
		Status:    strings.ToLower(status),
		Icon:      icons[percent/10],
		Remaining: remaining(battery, status),
	}
}

func remaining(props map[string]string, status string) string {
	now, full, rate := props["CHARGE_NOW"], props["CHARGE_FULL"], props["CURRENT_NOW"]
	if now == "" {
		now, full, rate = props["ENERGY_NOW"], props["ENERGY_FULL"], props["POWER_NOW"]
	}
	level, nowErr := strconv.ParseFloat(now, 64)
	capacity, fullErr := strconv.ParseFloat(full, 64)
	draw, rateErr := strconv.ParseFloat(rate, 64)
	if nowErr != nil || fullErr != nil || rateErr != nil || draw <= 0 {
		return ""
	}

	var hours float64
	switch status {
	case "Discharging":
		hours = level / draw
	case "Charging":
		hours = max(capacity-level, 0) / draw
	default:
		return ""
	}
	d := time.Duration(hours * float64(time.Hour)).Round(time.Minute)
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
}

func readUevent(path string) map[string]string {
	props := make(map[string]string)
	for line := range strings.SplitSeq(readFile(path), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			props[strings.TrimPrefix(key, "POWER_SUPPLY_")] = value
		}
	}
	return props
}
