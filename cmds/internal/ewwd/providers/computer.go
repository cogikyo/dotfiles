package providers

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const hwmonRoot = "/sys/class/hwmon"

type ComputerState struct {
	RAMAvailable    bool    `json:"ram_available"`
	RAMUsedPercent  int     `json:"ram_used_percent"`
	RAMUsedGiB      float64 `json:"ram_used_gib"`
	DiskAvailable   bool    `json:"disk_available"`
	DiskUsedPercent int     `json:"disk_used_percent"`
	DiskUsedGiB     int     `json:"disk_used_gib"`
	TempAvailable   bool    `json:"temp_available"`
	TempCurrent     int     `json:"temp_current"`
	TempPercent     int     `json:"temp_percent"`
	TempMax         float64 `json:"temp_max"`
	Breakdown
}

type Breakdown struct {
	Available bool `json:"breakdown_available"`
	TmpGiB    int  `json:"tmp_gib"`
	TrashGiB  int  `json:"trash_gib"`
	CacheGiB  int  `json:"cache_gib"`
}

const scanInterval = 2 * time.Minute

type Computer struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	tempBase  string
	tempMax   int64
	breakdown Breakdown
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	scans := make(chan Breakdown, 1)
	go scanBreakdown(ctx, scans)
	c.publish(notify)

	for waitForNextBoundary(ctx, c.done, time.Second) {
		select {
		case c.breakdown = <-scans:
		default:
		}
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
	state.RAMUsedPercent, state.RAMUsedGiB, state.RAMAvailable = readRAM()
	state.DiskUsedPercent, state.DiskUsedGiB, state.DiskAvailable = readDisk("/")
	state.TempCurrent, state.TempPercent, state.TempMax, state.TempAvailable = c.readTemperature()
	state.Breakdown = c.breakdown
	return state
}

func scanBreakdown(ctx context.Context, out chan<- Breakdown) {
	for {
		if b, ok := readBreakdown(ctx); ok {
			select {
			case out <- b:
			case <-ctx.Done():
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(scanInterval):
		}
	}
}

// readBreakdown passes scratch to du first: one du call counts the /tmp/opencode bind mount of
// ~/.cache/opencode-tmp once, under tmp; a separate call would count it again under cache.
// /tmp itself is tmpfs, so only that mount holds disk space.
func readBreakdown(ctx context.Context) (Breakdown, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Breakdown{}, false
	}
	b := Breakdown{Available: true}
	targets := []struct {
		path string
		gib  *int
	}{
		{"/tmp/opencode", &b.TmpGiB},
		{filepath.Join(home, ".local/share/Trash"), &b.TrashGiB},
		{filepath.Join(home, ".cache"), &b.CacheGiB},
	}
	args := []string{"-c3", "nice", "-n19", "du", "-sk"}
	for _, t := range targets {
		args = append(args, t.path)
	}

	out, err := exec.CommandContext(ctx, "ionice", args...).Output()
	if ctx.Err() != nil {
		return Breakdown{}, false
	}
	// du exits 1 when it cannot read a subdirectory or a path is missing; the totals it prints still hold.
	if err != nil && !errors.As(err, new(*exec.ExitError)) {
		return Breakdown{}, false
	}
	for line := range strings.Lines(string(out)) {
		size, path, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		kib, err := strconv.ParseUint(size, 10, 64)
		if err != nil {
			continue
		}
		for _, t := range targets {
			if t.path == path {
				*t.gib = int(math.Ceil(float64(kib) * 1024 / gib))
			}
		}
	}
	return b, true
}

const gib = 1 << 30

func readRAM() (percent int, usedGiB float64, ok bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
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
		return 0, 0, false
	}

	used := total - available
	percent = int(math.Round(float64(used) / float64(total) * 100))
	usedGiB = math.Round(float64(used)*1024/gib*10) / 10
	return percent, usedGiB, true
}

// readDisk matches df's Use% and its rounded-up sizes: root-reserved blocks count as neither used nor available.
func readDisk(path string) (percent, usedGiB int, ok bool) {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) != nil {
		return 0, 0, false
	}
	used := st.Blocks - st.Bfree
	if used+st.Bavail == 0 {
		return 0, 0, false
	}
	percent = int(math.Ceil(float64(used) / float64(used+st.Bavail) * 100))
	usedGiB = int(math.Ceil(float64(used) * float64(st.Frsize) / gib))
	return percent, usedGiB, true
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
