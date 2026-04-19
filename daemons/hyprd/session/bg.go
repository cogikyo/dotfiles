package session

// bg.go manages mpvpaper background process startup, health checks, and teardown.

import (
	"fmt"
	"net"
	"os/exec"
	"time"

	"dotfiles/daemons/config"
)

// BG manages a single mpvpaper wallpaper process.
type BG struct {
	cfg *config.BackgroundConfig
}

func NewBG(cfg *config.BackgroundConfig) *BG {
	return &BG{cfg: cfg}
}

// Execute runs "ensure" (spawn if dead) or "kill" (pkill all).
func (b *BG) Execute(mode string) (string, error) {
	switch mode {
	case "ensure":
		return b.ensure()
	case "kill":
		b.killAll()
		return "bg: killed", nil
	default:
		return "", fmt.Errorf("unknown bg mode: %s (ensure|kill)", mode)
	}
}

func (b *BG) ensure() (string, error) {
	if b.isAlive() {
		return "bg: running", nil
	}
	b.killAll()
	b.spawn()
	return "bg: spawned", nil
}

func (b *BG) isAlive() bool {
	conn, err := net.DialTimeout("unix", b.cfg.Socket, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (b *BG) spawn() {
	v := &b.cfg.Wallpaper
	videoPath := config.ExpandPath(b.cfg.VideoPath)
	fullPath := videoPath + "/" + v.File
	opts := fmt.Sprintf("--loop --input-ipc-server=%s --brightness=%d --contrast=%d --saturation=%d --hue=%d",
		b.cfg.Socket, v.Brightness, v.Contrast, v.Saturation, v.Hue)
	cmd := exec.Command("mpvpaper", "-o", opts, b.cfg.Display, fullPath)
	cmd.Start()
}

func (b *BG) killAll() {
	exec.Command("pkill", "mpvpaper").Run()
	// settle so a following spawn does not race socket teardown
	time.Sleep(100 * time.Millisecond)
}

// EnsureBG spawns the wallpaper if not already running.
func EnsureBG(cfg *config.BackgroundConfig) {
	bg := NewBG(cfg)
	bg.Execute("ensure")
}
