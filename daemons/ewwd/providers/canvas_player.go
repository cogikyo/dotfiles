package providers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const (
	canvasFrameDir = "/dev/shm/eww/canvas"
	canvasFPS      = 24
	canvasWidth    = 94
	canvasHeight   = 95
)

type CanvasPlayer struct {
	frames []string
	stop   chan struct{}
	frame  int
	mu     sync.Mutex
}

func NewCanvasPlayer() *CanvasPlayer {
	return &CanvasPlayer{}
}

func (p *CanvasPlayer) Load(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopLocked()
	clearDir(canvasFrameDir)

	if err := os.MkdirAll(canvasFrameDir, 0o755); err != nil {
		return fmt.Errorf("canvas dir: %w", err)
	}

	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-vf", fmt.Sprintf("scale=%d:%d,fps=%d", canvasWidth, canvasHeight, canvasFPS),
		"-q:v", "2",
		filepath.Join(canvasFrameDir, "f_%04d.jpg"),
	)
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, out)
	}

	entries, err := os.ReadDir(canvasFrameDir)
	if err != nil {
		return err
	}
	p.frames = p.frames[:0]
	for _, e := range entries {
		if !e.IsDir() {
			p.frames = append(p.frames, filepath.Join(canvasFrameDir, e.Name()))
		}
	}
	if len(p.frames) == 0 {
		return fmt.Errorf("ffmpeg produced no frames")
	}
	p.frame = 0
	return nil
}

func (p *CanvasPlayer) Play(onTick func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.frames) == 0 || p.stop != nil {
		return
	}

	p.stop = make(chan struct{})
	go p.loop(p.stop, onTick)
}

func (p *CanvasPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *CanvasPlayer) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	p.frames = nil
	clearDir(canvasFrameDir)
}

func (p *CanvasPlayer) HasFrames() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.frames) > 0
}

func (p *CanvasPlayer) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stop != nil
}

func (p *CanvasPlayer) CurrentFrame() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.frames) == 0 {
		return ""
	}
	return p.frames[p.frame%len(p.frames)]
}

func (p *CanvasPlayer) loop(stop chan struct{}, onTick func(string)) {
	ticker := time.NewTicker(time.Second / canvasFPS)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.mu.Lock()
			if len(p.frames) == 0 {
				p.mu.Unlock()
				return
			}
			p.frame = (p.frame + 1) % len(p.frames)
			frame := p.frames[p.frame]
			p.mu.Unlock()
			onTick(frame)
		}
	}
}

func (p *CanvasPlayer) stopLocked() {
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
}

func clearDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		os.Remove(filepath.Join(dir, e.Name()))
	}
}
