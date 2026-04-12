package commands

import (
	"fmt"
	"net"
	"os/exec"
	"time"

	"dotfiles/daemons/config"
)

// BG manages a single mpvpaper wallpaper process.
// Ensures the configured video is running; respawns if it crashed or was killed.
type BG struct {
	cfg *config.BackgroundConfig
}

// NewBG creates a background manager from config.
func NewBG(cfg *config.BackgroundConfig) *BG {
	return &BG{cfg: cfg}
}

// Execute runs a background command.
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

// ensure checks if mpvpaper is responsive via IPC. If not, kills stale
// processes and spawns a fresh one.
func (b *BG) ensure() (string, error) {
	if b.isAlive() {
		return "bg: running", nil
	}

	b.killAll()
	b.spawn()
	return "bg: spawned", nil
}

// isAlive checks if mpvpaper responds on the IPC socket.
func (b *BG) isAlive() bool {
	conn, err := net.DialTimeout("unix", b.cfg.Socket, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// spawn starts mpvpaper with the configured video and IPC socket.
func (b *BG) spawn() {
	v := &b.cfg.Wallpaper
	videoPath := config.ExpandPath(b.cfg.VideoPath)
	fullPath := videoPath + "/" + v.File

	opts := fmt.Sprintf("--loop --input-ipc-server=%s --brightness=%d --contrast=%d --saturation=%d --hue=%d",
		b.cfg.Socket, v.Brightness, v.Contrast, v.Saturation, v.Hue)

	cmd := exec.Command("mpvpaper", "-o", opts, b.cfg.Display, fullPath)
	cmd.Start()
}

// killAll kills all mpvpaper processes.
func (b *BG) killAll() {
	exec.Command("pkill", "mpvpaper").Run()
	time.Sleep(100 * time.Millisecond)
}

// EnsureBG ensures the wallpaper is running. Used by ws command.
func EnsureBG(cfg *config.BackgroundConfig) {
	bg := NewBG(cfg)
	bg.Execute("ensure")
}
