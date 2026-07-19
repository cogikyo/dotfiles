package providers

// canvas_player.go converts Spotify Canvas MP4s into frames and cycles them for eww image widgets.
import (
	"bytes"
	"context"
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

// CanvasPlayer owns committed frame sets and the playback goroutine.
type CanvasPlayer struct {
	frames []string
	stop   chan struct{}
	frame  int
	mu     sync.Mutex
}

func NewCanvasPlayer() *CanvasPlayer { return &CanvasPlayer{} }

// Prepare renders a Canvas into an isolated directory without changing displayed frames.
func (p *CanvasPlayer) Prepare(ctx context.Context, data []byte) ([]string, error) {
	if err := os.MkdirAll(canvasFrameDir, 0o755); err != nil {
		return nil, fmt.Errorf("canvas dir: %w", err)
	}
	dir, err := os.MkdirTemp(canvasFrameDir, "canvas-*")
	if err != nil {
		return nil, err
	}
	fail := func(err error) ([]string, error) {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-vf", fmt.Sprintf("scale=%d:%d,fps=%d", canvasWidth, canvasHeight, canvasFPS), "-q:v", "2", filepath.Join(dir, "f_%04d.jpg"))
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fail(fmt.Errorf("ffmpeg: %w: %s", err, out))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fail(err)
	}
	frames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			frames = append(frames, filepath.Join(dir, entry.Name()))
		}
	}
	if len(frames) == 0 {
		return fail(fmt.Errorf("ffmpeg produced no frames"))
	}
	return frames, nil
}

// SetFrames commits prepared frames only after the music owner validates their track revision.
func (p *CanvasPlayer) SetFrames(frames []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	p.clearFramesLocked()
	p.frames = frames
	p.frame = 0
}

// Discard removes a prepared frame set that became obsolete before commit.
func (p *CanvasPlayer) Discard(frames []string) {
	if len(frames) != 0 {
		_ = os.RemoveAll(filepath.Dir(frames[0]))
	}
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
	p.clearFramesLocked()
	p.frames = nil
	p.frame = 0
}

func (p *CanvasPlayer) HasFrames() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.frames) > 0
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

func (p *CanvasPlayer) clearFramesLocked() {
	if len(p.frames) != 0 {
		_ = os.RemoveAll(filepath.Dir(p.frames[0]))
	}
}
