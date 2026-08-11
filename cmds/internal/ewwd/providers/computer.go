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
		c.tempBase = findNVMeComposite()
	}
	if c.tempBase == "" {
		return 0, 0, 0, false
	}

	input, inputErr := readMillidegrees(c.tempBase + "_input")
	minimum, minErr := readMillidegrees(c.tempBase + "_min")
	maximum, maxErr := readMillidegrees(c.tempBase + "_max")
	if inputErr != nil || minErr != nil || maxErr != nil || minimum+maximum == 0 {
		c.tempBase = ""
		return 0, 0, 0, false
	}

	current = int(math.Round(float64(input) / 1000))
	// Preserve the widget's established input / (minimum + maximum) ramp scale.
	percent = int(math.Round(float64(input) / float64(minimum+maximum) * 100))
	maxTemp = float64(maximum) / 1000
	return current, percent, maxTemp, true
}

func findNVMeComposite() string {
	dirs, err := os.ReadDir(hwmonRoot)
	if err != nil {
		return ""
	}
	for _, dir := range dirs {
		if !strings.HasPrefix(dir.Name(), "hwmon") {
			continue
		}
		path := filepath.Join(hwmonRoot, dir.Name())
		if strings.TrimSpace(readFile(filepath.Join(path, "name"))) != "nvme" {
			continue
		}
		labels, _ := filepath.Glob(filepath.Join(path, "temp*_label"))
		for _, label := range labels {
			if strings.TrimSpace(readFile(label)) == "Composite" {
				return strings.TrimSuffix(label, "_label")
			}
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
