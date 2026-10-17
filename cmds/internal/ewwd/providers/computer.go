package providers

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const hwmonRoot = "/sys/class/hwmon"

type ComputerState struct {
	RAMAvailable   bool    `json:"ram_available"`
	RAMUsedPercent int     `json:"ram_used_percent"`
	TempAvailable  bool    `json:"temp_available"`
	TempCurrent    int     `json:"temp_current"`
	TempPercent    int     `json:"temp_percent"`
	TempMax        float64 `json:"temp_max"`
}

type Computer struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	tempBase  string
	tempMax   int64
	last      ComputerState
	published bool
}

func NewComputer(state StateSetter) Provider {
	return &Computer{state: state, done: make(chan struct{})}
}

func (c *Computer) Name() string {
	return "computer"
}

func (c *Computer) Start(ctx context.Context, notify func(data any)) error {
	c.active = true
	c.publish(notify)

	for waitForNextBoundary(ctx, c.done, time.Second) {
		c.publish(notify)
	}
	return nil
}

func (c *Computer) Stop() error {
	if c.active {
		close(c.done)
		c.active = false
	}
	return nil
}

func (c *Computer) publish(notify func(data any)) {
	state := c.read()
	if c.published && c.last == state {
		return
	}
	c.last = state
	c.published = true
	c.state.Set("computer", &state)
	notify(&state)
}

func (c *Computer) read() ComputerState {
	state := ComputerState{}
	state.RAMUsedPercent, state.RAMAvailable = readRAM()
	state.TempCurrent, state.TempPercent, state.TempMax, state.TempAvailable = c.readTemperature()
	return state
}

func readRAM() (int, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}

	var total, available uint64
	var totalFound, availableFound bool
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = value
			totalFound = true
		case "MemAvailable:":
			available = value
			availableFound = true
		}
	}
	if !totalFound || !availableFound || total == 0 || available > total {
		return 0, false
	}

	used := math.Round(float64(total-available) / float64(total) * 100)
	return int(used), true
}

func (c *Computer) readTemperature() (current, percent int, maxTemp float64, available bool) {
	if c.tempBase == "" {
		c.tempBase, c.tempMax = findCPUTemp()
	}
	if c.tempBase == "" {
		return 0, 0, 0, false
	}

	input, err := readMillidegrees(c.tempBase + "_input")
	if err != nil {
		c.tempBase = ""
		return 0, 0, 0, false
	}

	current = int(math.Round(float64(input) / 1000))
	percent = int(math.Round(float64(input) / float64(c.tempMax) * 100))
	maxTemp = float64(c.tempMax) / 1000
	return current, percent, maxTemp, true
}

// fallbackTjMax scales sensors that expose no limit, such as k10temp.
// TODO: set TjMax per machine; Ryzen desktops throttle at 90–95°C, Strix Halo at 100°C.
const fallbackTjMax = 95_000

func findCPUTemp() (base string, maximum int64) {
	dirs, err := os.ReadDir(hwmonRoot)
	if err != nil {
		return "", 0
	}
	for _, dir := range dirs {
		path := filepath.Join(hwmonRoot, dir.Name())
		switch strings.TrimSpace(readFile(filepath.Join(path, "name"))) {
		case "k10temp":
			if base := findTempLabel(path, "Tctl"); base != "" {
				return base, fallbackTjMax
			}
		case "coretemp":
			if base := findTempLabel(path, "Package id 0"); base != "" {
				if crit, err := readMillidegrees(base + "_crit"); err == nil && crit > 0 {
					return base, crit
				}
				return base, fallbackTjMax
			}
		}
	}
	return "", 0
}

func findTempLabel(hwmon, want string) string {
	labels, _ := filepath.Glob(filepath.Join(hwmon, "temp*_label"))
	for _, label := range labels {
		if strings.TrimSpace(readFile(label)) == want {
			return strings.TrimSuffix(label, "_label")
		}
	}
	return ""
}

func readMillidegrees(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}
