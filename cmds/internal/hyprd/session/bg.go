package session

// bg.go manages mpvpaper background process startup, health checks, and teardown.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
)

const (
	bgStartTimeout     = 2 * time.Second
	bgBootStartTimeout = 2 * time.Second
	bgFrameSettle      = 250 * time.Millisecond
)

var errNoActiveMonitors = errors.New("no active hyprland monitors")

// BG manages a single mpvpaper wallpaper process.
type BG struct {
	cfg *config.BackgroundConfig
}

const (
	backgroundCPUs        = "0-6,8-14,16-1023"
	backgroundOOMScoreAdj = "1000"
)

func NewBG(cfg *config.BackgroundConfig) *BG {
	return &BG{cfg: cfg}
}

// Execute runs "ensure" (spawn if dead) or "kill" (pkill all).
func (b *BG) Execute(mode string) (string, error) {
	switch mode {
	case "ensure":
		return b.ensure(bgStartTimeout)
	case "kill":
		b.killAll()
		return "bg: killed", nil
	default:
		return "", fmt.Errorf("unknown bg mode: %s (ensure|kill)", mode)
	}
}

func (b *BG) ensure(timeout time.Duration) (string, error) {
	if b.isAlive() {
		return "bg: running", nil
	}
	b.killAll()
	display, err := b.spawn()
	if err != nil {
		if errors.Is(err, errNoActiveMonitors) {
			return "bg: no active monitors", nil
		}
		return "", err
	}
	if !b.waitAlive(timeout) {
		return "", fmt.Errorf("bg: spawned on %s but IPC did not become ready", display)
	}
	time.Sleep(bgFrameSettle)
	return fmt.Sprintf("bg: spawned on %s", display), nil
}

func (b *BG) isAlive() bool {
	conn, err := net.DialTimeout("unix", b.cfg.Socket, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (b *BG) waitAlive(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b.isAlive() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return b.isAlive()
}

func (b *BG) spawn() (string, error) {
	v := &b.cfg.Wallpaper
	display, err := b.resolveDisplay()
	if err != nil {
		return "", err
	}
	videoPath := config.ExpandPath(b.cfg.VideoPath)
	fullPath := videoPath + "/" + v.File
	opts := fmt.Sprintf("--loop --input-ipc-server=%s --brightness=%d --contrast=%d --saturation=%d --hue=%d",
		b.cfg.Socket, v.Brightness, v.Contrast, v.Saturation, v.Hue)
	cmd := exec.Command("taskset", "-c", backgroundCPUs, "mpvpaper", "-p", "-o", opts, display, fullPath)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start mpvpaper: %w", err)
	}
	// Keep mpvpaper ahead of preferred development processes in OOM victim selection.
	scorePath := fmt.Sprintf("/proc/%d/oom_score_adj", cmd.Process.Pid)
	if err := os.WriteFile(scorePath, []byte(backgroundOOMScoreAdj), 0); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", fmt.Errorf("set mpvpaper OOM priority: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return "", fmt.Errorf("release mpvpaper: %w", err)
	}
	return display, nil
}

func (b *BG) resolveDisplay() (string, error) {
	display := strings.TrimSpace(b.cfg.Display)
	if display != "" && display != "auto" {
		return display, nil
	}

	data, err := exec.Command("hyprctl", "-j", "monitors").Output()
	if err != nil {
		return "", fmt.Errorf("query hyprland monitors: %w", err)
	}

	var monitors []struct {
		Name     string `json:"name"`
		Focused  bool   `json:"focused"`
		Disabled bool   `json:"disabled"`
	}
	if err := json.Unmarshal(data, &monitors); err != nil {
		return "", fmt.Errorf("parse hyprland monitors: %w", err)
	}

	for _, m := range monitors {
		if m.Focused && !m.Disabled && m.Name != "" {
			return m.Name, nil
		}
	}
	for _, m := range monitors {
		if !m.Disabled && m.Name != "" {
			return m.Name, nil
		}
	}
	return "", errNoActiveMonitors
}

func (b *BG) killAll() {
	exec.Command("pkill", "mpvpaper").Run()
	// settle so a following spawn does not race socket teardown
	time.Sleep(100 * time.Millisecond)
}

// EnsureBG spawns the wallpaper if not already running.
func EnsureBG(cfg *config.BackgroundConfig) error {
	bg := NewBG(cfg)
	_, err := bg.Execute("ensure")
	return err
}

// EnsureBGBoot waits long enough for cold-boot display and media startup before locking.
func EnsureBGBoot(cfg *config.BackgroundConfig) error {
	bg := NewBG(cfg)
	_, err := bg.ensure(bgBootStartTimeout)
	return err
}
