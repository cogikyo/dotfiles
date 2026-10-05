package providers

// canvas_player.go retains each Canvas MP4 beside the frames cycled for eww image widgets.
import (
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

type canvasSet struct {
	dir    string
	video  string
	frames []string
}

func (s canvasSet) remove() {
	if s.dir != "" {
		_ = os.RemoveAll(s.dir)
	}
}

// CanvasPlayer owns committed MP4s, their frame sets, and the playback goroutine.
type CanvasPlayer struct {
	set   canvasSet
	stop  chan struct{}
	frame int
	mu    sync.Mutex
}

func NewCanvasPlayer() *CanvasPlayer { return &CanvasPlayer{} }

// Prepare retains a Canvas MP4 and renders its frames in an isolated directory without changing the committed set.
func (p *CanvasPlayer) Prepare(ctx context.Context, data []byte) (canvasSet, error) {
	if err := os.MkdirAll(canvasFrameDir, 0o755); err != nil {
		return canvasSet{}, fmt.Errorf("canvas dir: %w", err)
	}
	dir, err := os.MkdirTemp(canvasFrameDir, "canvas-*")
	if err != nil {
		return canvasSet{}, err
	}
	set := canvasSet{dir: dir, video: filepath.Join(dir, "canvas.mp4")}
	fail := func(err error) (canvasSet, error) {
		set.remove()
		return canvasSet{}, err
	}
	if err := os.WriteFile(set.video, data, 0o644); err != nil {
		return fail(fmt.Errorf("canvas video: %w", err))
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-i", set.video, "-vf", fmt.Sprintf("scale=%d:%d,fps=%d", canvasWidth, canvasHeight, canvasFPS), "-q:v", "2", filepath.Join(dir, "f_%04d.jpg"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fail(fmt.Errorf("ffmpeg: %w: %s", err, out))
	}
	frames, err := filepath.Glob(filepath.Join(dir, "f_*.jpg"))
	if err != nil {
		return fail(err)
	}
	if len(frames) == 0 {
		return fail(fmt.Errorf("ffmpeg produced no frames"))
	}
	set.frames = frames
	return set, nil
}

// Commit installs a prepared set only after the music owner validates its track revision.
func (p *CanvasPlayer) Commit(set canvasSet) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	p.set.remove()
	p.set = set
	p.frame = 0
}

// Discard removes a prepared set that became obsolete before commit.
func (p *CanvasPlayer) Discard(set canvasSet) {
	set.remove()
}

func (p *CanvasPlayer) Play(onTick func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.set.frames) == 0 || p.stop != nil {
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
	p.set.remove()
	p.set = canvasSet{}
	p.frame = 0
}

func (p *CanvasPlayer) HasFrames() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.set.frames) > 0
}

func (p *CanvasPlayer) CurrentFrame() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.set.frames) == 0 {
		return ""
	}
	return p.set.frames[p.frame%len(p.set.frames)]
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
			if len(p.set.frames) == 0 {
				p.mu.Unlock()
				return
			}
			p.frame = (p.frame + 1) % len(p.set.frames)
			frame := p.set.frames[p.frame]
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
