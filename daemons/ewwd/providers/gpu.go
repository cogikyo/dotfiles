package providers

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dotfiles/daemons/config"
)

// GPUState is an AMD GPU metrics snapshot. All numeric fields ship as pre-formatted strings.
type GPUState struct {
	GPUBusy   string `json:"gpu_busy"`   // utilization percent
	MemBusy   string `json:"mem_busy"`   // memory controller utilization percent
	MCLK      string `json:"mclk"`       // memory clock in MHz
	MCLKLevel string `json:"mclk_level"` // active memory clock level (0-N)
	VRAM      string `json:"vram"`       // VRAM usage percent
	Used      string `json:"used"`       // VRAM used in bytes
}

// GPU polls AMD GPU metrics out of sysfs device attributes (config.DevicePath).
type GPU struct {
	state  StateSetter
	config config.GPUConfig
	done   chan struct{}
	active bool
}

func NewGPU(state StateSetter, cfg config.GPUConfig) Provider {
	return &GPU{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
}

func (g *GPU) Name() string {
	return "gpu"
}

func (g *GPU) Start(ctx context.Context, notify func(data any)) error {
	g.active = true
	ticker := time.NewTicker(g.config.PollInterval)
	defer ticker.Stop()

	if state := g.read(); state != nil {
		g.state.Set("gpu", state)
		notify(state)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-g.done:
			return nil
		case <-ticker.C:
			if state := g.read(); state != nil {
				g.state.Set("gpu", state)
				notify(state)
			}
		}
	}
}

func (g *GPU) Stop() error {
	if g.active {
		close(g.done)
		g.active = false
	}
	return nil
}

func (g *GPU) read() *GPUState {
	path := g.config.DevicePath
	gpuBusy := readFile(path + "/gpu_busy_percent")
	memBusy := readFile(path + "/mem_busy_percent")

	// pp_dpm_mclk format: "<level>: <freq>Mhz[ *]" per line, '*' marks the active level.
	mclkData := readFile(path + "/pp_dpm_mclk")
	mclk, mclkLevel := parseMCLK(mclkData)

	totalStr := readFile(path + "/mem_info_vram_total")
	usedStr := readFile(path + "/mem_info_vram_used")
	vram := calculateVRAMPercent(totalStr, usedStr)

	return &GPUState{
		GPUBusy:   strings.TrimSpace(gpuBusy),
		MemBusy:   strings.TrimSpace(memBusy),
		MCLK:      mclk,
		MCLKLevel: mclkLevel,
		VRAM:      vram,
		Used:      strings.TrimSpace(usedStr),
	}
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

var mclkActiveRe = regexp.MustCompile(`^(\d):\s*(\d+)Mhz\s*\*`)

func parseMCLK(data string) (mclk, level string) {
	for line := range strings.SplitSeq(data, "\n") {
		if strings.Contains(line, "*") {
			matches := mclkActiveRe.FindStringSubmatch(line)
			if len(matches) >= 3 {
				return matches[2], matches[1]
			}
		}
	}
	return "", ""
}

func calculateVRAMPercent(totalStr, usedStr string) string {
	total, err1 := strconv.ParseFloat(strings.TrimSpace(totalStr), 64)
	used, err2 := strconv.ParseFloat(strings.TrimSpace(usedStr), 64)
	if err1 != nil || err2 != nil || total == 0 {
		return "0"
	}
	percent := (used / total) * 100
	return fmt.Sprintf("%d", int(percent+0.5))
}
