package providers

// music.go owns the authoritative Spotify snapshot and reconciles playerctl feedback.
import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	musicPollInterval   = time.Second
	musicPlayer         = "spotify"
	albumArtPath        = "/tmp/eww/album_art.png"
	defaultVolumeDelta  = 0.05
	defaultSeekDelta    = 10
	musicCommandTimeout = 3 * time.Second
	artTimeout          = 15 * time.Second
	canvasTimeout       = 30 * time.Second
	followSeparator     = "\x1f"
	followFormat        = `{{status}}` + followSeparator + `{{volume}}` + followSeparator + `{{artist}}` + followSeparator + `{{album}}` + followSeparator + `{{title}}` + followSeparator + `{{mpris:artUrl}}` + followSeparator + `{{mpris:trackid}}` + followSeparator + `{{mpris:length}}`
)

type MusicState struct {
	Status        string `json:"status"`
	Playing       bool   `json:"playing"`
	Volume        string `json:"volume"`
	VolumePercent int    `json:"volume_percent"`
	Artist        string `json:"artist"`
	ArtistShort   string `json:"artist_short"`
	Album         string `json:"album"`
	AlbumShort    string `json:"album_short"`
	Title         string `json:"title"`
	TitleShort    string `json:"title_short"`
	SingleTrack   bool   `json:"single_track"`
	Progress      int    `json:"progress"`
	ArtPath       string `json:"art_path"`
	HasCanvas     bool   `json:"has_canvas"`
	CanvasFrame   string `json:"canvas_frame"`
}

// Music serializes every snapshot mutation through Start's event loop.
type Music struct {
	state  StateSetter
	canvas *CanvasClient
	player *CanvasPlayer

	events chan musicEvent
	wg     sync.WaitGroup

	lifecycle sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	active    bool
	actionMu  sync.Mutex
	notify    func(data any)
}

type musicEvent func(*musicOwner)

type musicOwner struct {
	music         *Music
	last          MusicState
	track         string
	artURL        string
	duration      float64
	trackRevision uint64
	seekRevision  uint64
	seekPending   bool
	pollPending   bool
}

type followState struct {
	status, volume, artist, album, title, artURL, track string
	duration                                            float64
}

type positionTicket struct {
	track, source               string
	trackRevision, seekRevision uint64
	duration                    float64
}

func NewMusic(state StateSetter, spDc string) Provider {
	m := &Music{
		state:  state,
		events: make(chan musicEvent, 64),
	}
	if client := NewCanvasClient(spDc); client != nil {
		m.canvas = client
		m.player = NewCanvasPlayer()
		fmt.Println("ewwd: canvas enabled (sp_dc resolved)")
	}
	return m
}

func (m *Music) Name() string { return "music" }

func (m *Music) Start(ctx context.Context, notify func(data any)) error {
	runCtx, cancel := context.WithCancel(ctx)
	m.lifecycle.Lock()
	if m.active {
		m.lifecycle.Unlock()
		cancel()
		return errors.New("music provider already running")
	}
	m.ctx, m.cancel, m.active, m.notify = runCtx, cancel, true, notify
	m.lifecycle.Unlock()

	defer func() {
		cancel()
		m.wg.Wait()
		if m.player != nil {
			m.player.Clear()
		}
		m.lifecycle.Lock()
		m.active, m.ctx, m.cancel, m.notify = false, nil, nil, nil
		m.lifecycle.Unlock()
	}()

	if err := os.MkdirAll(filepath.Dir(albumArtPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "music: create album art directory: %v\n", err)
	}

	owner := musicOwner{music: m, last: stoppedMusicState()}
	owner.publish(true)
	m.wg.Go(func() { m.follow(runCtx) })

	ticker := time.NewTicker(musicPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case event := <-m.events:
			event(&owner)
		case <-ticker.C:
			owner.poll(runCtx)
		}
	}
}

func (m *Music) Stop() error {
	m.lifecycle.Lock()
	cancel := m.cancel
	m.lifecycle.Unlock()
	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	return nil
}

func (m *Music) follow(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		m.send(ctx, func(owner *musicOwner) { owner.snapshotUnavailable() })
		m.readSnapshot(ctx)

		cmd := exec.CommandContext(ctx, "playerctl", "--player="+musicPlayer, "--follow", "metadata", "--format", followFormat)
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		if err == nil {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				state, parseErr := parseFollowState(scanner.Text())
				if parseErr != nil {
					continue
				}
				backoff = time.Second
				m.send(ctx, func(owner *musicOwner) { owner.applyFollow(state) })
			}
			err = errors.Join(scanner.Err(), cmd.Wait())
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: music follow: %v\n", err)
		}
		m.send(ctx, func(owner *musicOwner) { owner.snapshotUnavailable() })
		if !waitContext(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, 8*time.Second)
	}
}

func (m *Music) readSnapshot(ctx context.Context) {
	queryCtx, cancel := context.WithTimeout(ctx, musicCommandTimeout)
	defer cancel()
	line, err := m.playerctlOutput(queryCtx, "metadata", "--format", followFormat)
	if err != nil {
		return
	}
	state, err := parseFollowState(line)
	if err == nil {
		m.send(ctx, func(owner *musicOwner) { owner.applyFollow(state) })
	}
}

func parseFollowState(line string) (followState, error) {
	parts := strings.Split(line, followSeparator)
	if len(parts) != 8 {
		return followState{}, fmt.Errorf("expected 8 metadata fields, got %d", len(parts))
	}
	durationMicros, err := strconv.ParseFloat(parts[7], 64)
	if err != nil || durationMicros < 0 {
		durationMicros = 0
	}
	return followState{
		status: parts[0], volume: parts[1], artist: parts[2], album: parts[3], title: parts[4],
		artURL: parts[5], track: parts[6], duration: durationMicros / float64(time.Second/time.Microsecond),
	}, nil
}

func (owner *musicOwner) applyFollow(next followState) {
	previous := owner.last
	trackChanged := next.track != owner.track
	artChanged := next.artURL != owner.artURL
	if trackChanged {
		owner.track, owner.artURL = next.track, next.artURL
		owner.trackRevision++
		owner.seekRevision++
		owner.pollPending = false
		owner.clearVisuals()
	} else if artChanged {
		owner.artURL = next.artURL
		_ = os.Remove(albumArtPath)
	}

	progress := owner.last.Progress
	if trackChanged {
		progress = 0
	}
	owner.duration = next.duration
	state := musicState(next.status, next.volume, next.artist, next.album, next.title, progress)
	state.HasCanvas, state.CanvasFrame = owner.last.HasCanvas, owner.last.CanvasFrame
	if trackChanged {
		state.HasCanvas, state.CanvasFrame = false, ""
	}
	owner.last = state
	owner.syncCanvas()
	owner.publish(owner.last != previous)

	if trackChanged || artChanged {
		owner.downloadArt(next.artURL)
	}
	if trackChanged && next.track != "" && owner.music.canvas != nil {
		owner.fetchCanvas(next.track, owner.trackRevision)
	}
}

func (owner *musicOwner) snapshotUnavailable() {
	previous := owner.last
	if owner.track != "" {
		owner.track = ""
		owner.artURL = ""
		owner.trackRevision++
		owner.seekRevision++
		owner.pollPending = false
		owner.clearVisuals()
	}
	owner.duration = 0
	owner.last = stoppedMusicState()
	owner.publish(owner.last != previous)
}

func (owner *musicOwner) poll(ctx context.Context) {
	if owner.pollPending || owner.seekPending || owner.track == "" {
		return
	}
	owner.pollPending = true
	ticket := owner.positionTicket("poll")
	owner.music.wg.Go(func() { owner.music.readPosition(ctx, ticket, nil) })
}

func (owner *musicOwner) positionTicket(source string) positionTicket {
	return positionTicket{
		track: owner.track, source: source, trackRevision: owner.trackRevision,
		seekRevision: owner.seekRevision, duration: owner.duration,
	}
}

func (owner *musicOwner) applyPosition(ticket positionTicket, progress int) bool {
	// A position result has no MPRIS event ordering, so it must still describe this track and seek.
	if ticket.source == "seek" {
		owner.seekPending = false
	}
	if ticket.trackRevision != owner.trackRevision || ticket.seekRevision != owner.seekRevision || ticket.track != owner.track {
		return false
	}
	if ticket.source == "poll" {
		owner.pollPending = false
	}
	if owner.last.Progress == progress {
		return true
	}
	owner.last.Progress = progress
	owner.publish(true)
	return true
}

func (owner *musicOwner) positionFailed(ticket positionTicket) {
	if ticket.source == "seek" {
		owner.seekPending = false
	}
	if ticket.source == "poll" && ticket.trackRevision == owner.trackRevision && ticket.seekRevision == owner.seekRevision && ticket.track == owner.track {
		owner.pollPending = false
	}
}

func (owner *musicOwner) clearVisuals() {
	if owner.music.player != nil {
		owner.music.player.Clear()
	}
	_ = os.Remove(albumArtPath)
	owner.last.HasCanvas, owner.last.CanvasFrame = false, ""
}

func (owner *musicOwner) syncCanvas() {
	player := owner.music.player
	if player == nil {
		return
	}
	if !owner.last.Playing || !owner.last.HasCanvas || !player.HasFrames() {
		player.Stop()
		return
	}
	track, trackRevision := owner.track, owner.trackRevision
	player.Play(func(frame string) {
		owner.music.send(owner.music.runningContext(), func(current *musicOwner) {
			if current.track != track || current.trackRevision != trackRevision || !current.last.HasCanvas {
				return
			}
			if current.last.CanvasFrame == frame {
				return
			}
			current.last.CanvasFrame = frame
			current.publish(true)
		})
	})
}

func (owner *musicOwner) publish(changed bool) {
	if !changed {
		return
	}
	snapshot := owner.last
	owner.music.state.Set("music", &snapshot)
	if owner.music.notify != nil {
		owner.music.notify(&snapshot)
	}
}

func (owner *musicOwner) downloadArt(url string) {
	if url == "" {
		return
	}
	track, revision := owner.track, owner.trackRevision
	ctx, cancel := context.WithTimeout(owner.music.runningContext(), artTimeout)
	owner.music.wg.Go(func() {
		defer cancel()
		tmp, err := downloadFile(ctx, url, filepath.Dir(albumArtPath), "album-art-*")
		if err != nil {
			return
		}
		owner.music.send(ctx, func(current *musicOwner) {
			defer os.Remove(tmp)
			if current.track != track || current.trackRevision != revision || current.artURL != url {
				return
			}
			if err := os.Rename(tmp, albumArtPath); err != nil {
				return
			}
			current.publish(true)
		})
	})
}

func (owner *musicOwner) fetchCanvas(track string, revision uint64) {
	ctx, cancel := context.WithTimeout(owner.music.runningContext(), canvasTimeout)
	owner.music.wg.Go(func() {
		defer cancel()
		uri := strings.Replace(track, "/com/spotify/track/", "spotify:track:", 1)
		cdnURL, err := owner.music.canvas.FetchCanvasURL(ctx, uri)
		if err != nil || cdnURL == "" {
			return
		}
		data, err := DownloadCanvasData(ctx, cdnURL)
		if err != nil {
			return
		}
		frames, err := owner.music.player.Prepare(ctx, data)
		if err != nil {
			return
		}
		owner.music.send(ctx, func(current *musicOwner) {
			if current.track != track || current.trackRevision != revision {
				owner.music.player.Discard(frames)
				return
			}
			current.music.player.SetFrames(frames)
			current.last.HasCanvas = true
			current.last.CanvasFrame = current.music.player.CurrentFrame()
			current.syncCanvas()
			current.publish(true)
		})
	})
}

func (m *Music) readPosition(ctx context.Context, ticket positionTicket, done chan<- error) {
	queryCtx, cancel := context.WithTimeout(ctx, musicCommandTimeout)
	defer cancel()
	value, err := m.playerctlOutput(queryCtx, "position")
	if err != nil {
		m.finishPosition(ctx, ticket, 0, err, done)
		return
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		m.finishPosition(ctx, ticket, 0, fmt.Errorf("parse position: %w", err), done)
		return
	}
	progress := progressFor(seconds, ticket.duration)
	m.finishPosition(ctx, ticket, progress, nil, done)
}

func (m *Music) finishPosition(ctx context.Context, ticket positionTicket, progress int, result error, done chan<- error) {
	m.send(ctx, func(owner *musicOwner) {
		if result == nil {
			owner.applyPosition(ticket, progress)
		} else {
			owner.positionFailed(ticket)
		}
		if done != nil {
			done <- result
		}
	})
}

func (m *Music) send(ctx context.Context, event musicEvent) {
	if ctx == nil {
		return
	}
	select {
	case m.events <- event:
	case <-ctx.Done():
	}
}

func (m *Music) runningContext() context.Context {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	return m.ctx
}

func (m *Music) playerctlOutput(ctx context.Context, args ...string) (string, error) {
	args = append([]string{"--player=" + musicPlayer}, args...)
	out, err := exec.CommandContext(ctx, "playerctl", args...).Output()
	if err != nil {
		return "", fmt.Errorf("playerctl %s: %w", args[len(args)-1], err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (m *Music) runPlayerctl(ctx context.Context, args ...string) error {
	args = append([]string{"--player=" + musicPlayer}, args...)
	if err := exec.CommandContext(ctx, "playerctl", args...).Run(); err != nil {
		return fmt.Errorf("playerctl %s: %w", args[len(args)-1], err)
	}
	return nil
}

func (m *Music) actionContext() (context.Context, context.CancelFunc, error) {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if !m.active || m.ctx == nil {
		return nil, nil, errors.New("music provider is not running")
	}
	ctx, cancel := context.WithTimeout(m.ctx, musicCommandTimeout)
	return ctx, cancel, nil
}

func musicState(status, volume, artist, album, title string, progress int) MusicState {
	return MusicState{
		Status: status, Playing: status == "Playing", Volume: volume, VolumePercent: musicVolumePercent(volume),
		Artist: artist, ArtistShort: compactLabel(artist, shortInfoLimit),
		Album: album, AlbumShort: compactLabel(album, shortInfoLimit), Title: title,
		TitleShort: compactLabel(title, shortTitleLimit), SingleTrack: title == album, Progress: progress, ArtPath: albumArtPath,
	}
}

const (
	shortInfoLimit  = 24
	shortTitleLimit = 40
)

// compactLabel strips balanced parenthesized detail (e.g. "(feat. X)") before truncating;
// full metadata stays in Artist/Album/Title for tooltips.
func compactLabel(value string, limit int) string {
	return truncateForWidget(stripParenGroups(value), limit)
}

// stripParenGroups removes complete balanced "(" ... ")" groups, including nested ones.
// Unbalanced parens are left untouched so unrelated content survives.
func stripParenGroups(value string) string {
	runes := []rune(value)
	removed := make([]bool, len(runes))
	var open []int
	for i, r := range runes {
		switch r {
		case '(':
			open = append(open, i)
		case ')':
			if len(open) == 0 {
				continue
			}
			start := open[len(open)-1]
			open = open[:len(open)-1]
			if len(open) == 0 {
				for j := start; j <= i; j++ {
					removed[j] = true
				}
			}
		}
	}
	var kept []rune
	for i, r := range runes {
		if !removed[i] {
			kept = append(kept, r)
		}
	}
	return strings.Join(strings.Fields(string(kept)), " ")
}

func stoppedMusicState() MusicState {
	return musicState("Stopped", "0", "Spotify", "Waiting for player to start...", "Waiting for player to start...", 0)
}

func progressFor(position, duration float64) int {
	if duration <= 0 {
		return 0
	}
	return min(max(int(math.Round(position/duration*100)), 0), 100)
}

func truncateForWidget(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return strings.TrimRight(string(runes[:limit]), " ") + "..."
}

func musicVolumePercent(volume string) int {
	parsed, err := strconv.ParseFloat(volume, 64)
	if err != nil {
		return 0
	}
	return min(max(int(math.Round(parsed*100)), 0), 100)
}

func downloadFile(ctx context.Context, url, dir, pattern string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	defer func() {
		if tmp != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	path := tmp.Name()
	tmp = nil
	return path, nil
}

func waitContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (m *Music) HandleAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("action required: play, pause, toggle, volume, seek")
	}
	m.actionMu.Lock()
	defer m.actionMu.Unlock()

	ctx, cancel, err := m.actionContext()
	if err != nil {
		return "", err
	}
	defer cancel()

	switch args[0] {
	case "play", "pause", "next", "previous":
		if err := m.runPlayerctl(ctx, args[0]); err != nil {
			return "", err
		}
		return args[0], nil
	case "toggle":
		if err := m.runPlayerctl(ctx, "play-pause"); err != nil {
			return "", err
		}
		return "toggle", nil
	case "volume":
		return m.changeVolume(ctx, args[1:])
	case "seek":
		if len(args) != 2 || (args[1] != "up" && args[1] != "down") {
			return "", errors.New("seek requires direction: up or down")
		}
		return m.changeSeek(ctx, args[1])
	default:
		return "", fmt.Errorf("unknown action: %s", args[0])
	}
}

func (m *Music) changeVolume(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("volume requires direction: up, down, or set")
	}
	var value string
	switch args[0] {
	case "set":
		if len(args) != 2 {
			return "", errors.New("volume set requires a value")
		}
		parsed, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("invalid volume: %s", args[1])
		}
		value = fmt.Sprintf("%.2f", min(max(parsed, 0), 1))
	case "up", "down":
		if len(args) > 2 {
			return "", errors.New("volume accepts at most one delta")
		}
		amount := defaultVolumeDelta
		if len(args) == 2 {
			parsed, err := strconv.ParseFloat(args[1], 64)
			if err != nil || parsed < 0 {
				return "", fmt.Errorf("invalid volume delta: %s", args[1])
			}
			amount = parsed
		}
		suffix := "+"
		if args[0] == "down" {
			suffix = "-"
		}
		value = fmt.Sprintf("%.2f%s", amount, suffix)
	default:
		return "", errors.New("volume requires direction: up, down, or set")
	}
	if err := m.runPlayerctl(ctx, "volume", value); err != nil {
		return "", err
	}
	return "volume changed", nil
}

func (m *Music) changeSeek(ctx context.Context, direction string) (string, error) {
	tickets := make(chan positionTicket, 1)
	m.send(ctx, func(owner *musicOwner) {
		owner.seekRevision++
		owner.seekPending = true
		owner.pollPending = false
		tickets <- owner.positionTicket("seek")
	})
	var ticket positionTicket
	select {
	case ticket = <-tickets:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	sign := "+"
	if direction == "down" {
		sign = "-"
	}
	if err := m.runPlayerctl(ctx, "position", fmt.Sprintf("%d%s", defaultSeekDelta, sign)); err != nil {
		m.send(m.runningContext(), func(owner *musicOwner) { owner.seekPending = false })
		return "", err
	}
	result := make(chan error, 1)
	m.readPosition(ctx, ticket, result)
	select {
	case err := <-result:
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("seek %s%ds", sign, defaultSeekDelta), nil
	case <-ctx.Done():
		m.send(m.runningContext(), func(owner *musicOwner) { owner.seekPending = false })
		return "", ctx.Err()
	}
}
