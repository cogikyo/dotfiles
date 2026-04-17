package providers

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
	musicPollInterval  = 1 * time.Second
	musicPlayer        = "spotify"
	albumArtPath       = "/tmp/eww/album_art.png"
	defaultVolumeDelta = 0.05 // 5%
	defaultSeekDelta   = 10   // 10 seconds
)

// MusicState is the Spotify playback snapshot exported to the statusbar.
type MusicState struct {
	Status        string `json:"status"`         // "Playing" | "Paused" | "Stopped"
	Playing       bool   `json:"playing"`        // Status == "Playing"
	Volume        string `json:"volume"`         // raw playerctl volume, "0.0" to "1.0"
	VolumePercent int    `json:"volume_percent"` // Volume scaled to 0-100
	Artist        string `json:"artist"`
	Album         string `json:"album"`
	AlbumShort    string `json:"album_short"`  // truncated for widget width
	Title         string `json:"title"`
	TitleShort    string `json:"title_short"`  // truncated for widget width
	SingleTrack   bool   `json:"single_track"` // title == album; widget collapses to one line
	Progress      int    `json:"progress"`     // playback progress 0-100
	ArtPath       string `json:"art_path"`     // path to cached album art (fixed)
}

// Music monitors Spotify via playerctl and exposes playback-control actions.
type Music struct {
	state      StateSetter
	notify     func(data any)
	done       chan struct{}
	active     bool
	last       MusicState
	lastArtURL string
	mu         sync.Mutex     // guards last, lastArtURL
	wg         sync.WaitGroup // tracks in-flight album-art downloads
}

func NewMusic(state StateSetter) Provider {
	return &Music{
		state: state,
		done:  make(chan struct{}),
	}
}

func (m *Music) Name() string {
	return "music"
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ lifecycle                                                                    │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// Start runs playerctl --follow for event-driven metadata updates alongside a 1s poll
// that covers fields follow mode omits (progress, in particular).
func (m *Music) Start(ctx context.Context, notify func(data any)) error {
	m.active = true
	m.notify = notify

	if err := os.MkdirAll(filepath.Dir(albumArtPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "music: failed to create album art directory: %v\n", err)
	}

	m.updateAndNotify()

	go m.followMode(ctx)

	ticker := time.NewTicker(musicPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-m.done:
			return nil
		case <-ticker.C:
			m.updateAndNotify()
		}
	}
}

// Stop signals shutdown and waits for in-flight art downloads so we don't leak partial files.
func (m *Music) Stop() error {
	if m.active {
		close(m.done)
		m.active = false
		m.wg.Wait()
	}
	return nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ playerctl follow mode                                                        │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// followMode streams metadata changes from playerctl, reconnecting after any failure.
func (m *Music) followMode(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		default:
		}

		cmd := exec.CommandContext(ctx, "playerctl",
			"--player="+musicPlayer,
			"--follow",
			"metadata",
			"--format",
			`{{status}}	{{volume}}	{{artist}}	{{album}}	{{title}}	{{mpris:artUrl}}`,
		)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		if err := cmd.Start(); err != nil {
			time.Sleep(time.Second)
			continue
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			case <-m.done:
				cmd.Process.Kill()
				return
			default:
			}

			line := scanner.Text()
			m.parseFollowLine(line)
		}

		cmd.Wait()

		// Back off briefly before reconnecting.
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case <-time.After(time.Second):
		}
	}
}

// parseFollowLine parses one tab-separated line and kicks off art downloads and change notifications.
// Fields: status, volume, artist, album, title, artUrl.
func (m *Music) parseFollowLine(line string) {
	parts := strings.Split(line, "\t")
	if len(parts) < 6 {
		return
	}

	state := MusicState{
		Status:   parts[0],
		Volume:   parts[1],
		Artist:   parts[2],
		Album:    parts[3],
		Title:    parts[4],
		Progress: m.getProgress(),
	}
	state = musicState(state.Status, state.Volume, state.Artist, state.Album, state.Title, state.Progress)

	artURL := parts[5]

	m.mu.Lock()
	defer m.mu.Unlock()

	if artURL != "" && artURL != m.lastArtURL {
		m.wg.Go(func() { m.downloadAlbumArt(artURL) })
		m.lastArtURL = artURL
	}

	// ArtPath is fixed (albumArtPath) so we skip it in change detection.
	if state.Status != m.last.Status ||
		state.Volume != m.last.Volume ||
		state.Artist != m.last.Artist ||
		state.Album != m.last.Album ||
		state.Title != m.last.Title ||
		state.Progress != m.last.Progress {

		m.last = state
		m.state.Set("music", &state)
		if m.notify != nil {
			m.notify(&state)
		}
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ polling fallback                                                             │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (m *Music) updateAndNotify() {
	state := m.getCurrentState()

	m.mu.Lock()
	defer m.mu.Unlock()

	if state.Status != m.last.Status ||
		state.Volume != m.last.Volume ||
		state.Artist != m.last.Artist ||
		state.Album != m.last.Album ||
		state.Title != m.last.Title ||
		state.Progress != m.last.Progress {

		m.last = state
		m.state.Set("music", &state)
		if m.notify != nil {
			m.notify(&state)
		}
	}
}

// getCurrentState builds a MusicState from per-field playerctl calls, with a "waiting" placeholder
// when no player is reachable.
func (m *Music) getCurrentState() MusicState {
	status := m.playerctl("status")
	if status == "" || status == "No players found" {
		return musicState(
			"Stopped",
			"0",
			"Spotify",
			"Waiting for player to start...",
			"Waiting for player to start...",
			0,
		)
	}

	volume := m.playerctl("volume")
	artist := m.playerctlMetadata("artist")
	album := m.playerctlMetadata("album")
	title := m.playerctlMetadata("title")
	artURL := m.playerctlMetadata("mpris:artUrl")
	progress := m.getProgress()

	if artURL != "" && artURL != m.lastArtURL {
		m.wg.Go(func() { m.downloadAlbumArt(artURL) })
		m.lastArtURL = artURL
	}

	return musicState(status, volume, artist, album, title, progress)
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ helpers and playerctl wrappers                                               │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func musicState(status, volume, artist, album, title string, progress int) MusicState {
	return MusicState{
		Status:        status,
		Playing:       status == "Playing",
		Volume:        volume,
		VolumePercent: musicVolumePercent(volume),
		Artist:        artist,
		Album:         album,
		AlbumShort:    truncateForWidget(album, 15),
		Title:         title,
		TitleShort:    truncateForWidget(title, 15),
		SingleTrack:   title == album,
		Progress:      progress,
		ArtPath:       albumArtPath,
	}
}

func truncateForWidget(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func musicVolumePercent(volume string) int {
	parsed, err := strconv.ParseFloat(volume, 64)
	if err != nil {
		return 0
	}

	percent := int(math.Round(parsed * 100))
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func (m *Music) playerctl(cmd string) string {
	out, err := exec.Command("playerctl", "--player="+musicPlayer, cmd).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Music) playerctlMetadata(field string) string {
	out, err := exec.Command("playerctl", "--player="+musicPlayer, "metadata", field).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Music) playerctlFormat(format string) string {
	out, err := exec.Command("playerctl", "--player="+musicPlayer, "metadata", "--format", format).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m *Music) getProgress() int {
	lengthStr := m.playerctlFormat("{{ mpris:length }}")
	positionStr := m.playerctlFormat("{{ position }}")

	length, err1 := strconv.ParseFloat(lengthStr, 64)
	position, err2 := strconv.ParseFloat(positionStr, 64)
	if err1 != nil || err2 != nil || length <= 0 {
		return 0
	}

	progress := int(math.Round(position / length * 100))
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

// downloadAlbumArt writes to albumArtPath atomically via tmp-file + rename so eww never sees a torn write.
func (m *Music) downloadAlbumArt(url string) {
	if url == "" {
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	tmpFile := albumArtPath + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return
	}

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpFile)
		return
	}

	os.Rename(tmpFile, albumArtPath)
}

func (m *Music) playerctlRun(args ...string) {
	fullArgs := append([]string{"--player=" + musicPlayer}, args...)
	if err := exec.Command("playerctl", fullArgs...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: playerctl %s error: %v\n", args[0], err)
	}
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ actions                                                                      │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// HandleAction supports: play, pause, toggle, next, previous, volume, seek.
func (m *Music) HandleAction(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("action required: play, pause, toggle, volume, seek")
	}

	switch args[0] {
	case "play":
		m.playerctlRun("play")
		return "playing", nil

	case "pause":
		m.playerctlRun("pause")
		return "paused", nil

	case "toggle":
		m.playerctlRun("play-pause")
		return "toggled", nil

	case "next":
		m.playerctlRun("next")
		return "next", nil

	case "previous":
		m.playerctlRun("previous")
		return "previous", nil

	case "volume":
		if len(args) < 2 {
			return "", errors.New("volume requires direction: up or down")
		}
		return m.changeVolume(args[1:])

	case "seek":
		if len(args) < 2 {
			return "", errors.New("seek requires direction: up or down")
		}
		return m.changeSeek(args[1])

	default:
		return "", fmt.Errorf("unknown action: %s", args[0])
	}
}

// changeVolume accepts "up|down [delta]" for relative steps, or "set <0.0-1.0>" for absolute.
func (m *Music) changeVolume(args []string) (string, error) {
	direction := args[0]

	if direction == "set" {
		if len(args) < 2 {
			return "", errors.New("volume set requires a value")
		}

		value, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return "", fmt.Errorf("invalid volume: %s", args[1])
		}

		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}

		m.playerctlRun("volume", fmt.Sprintf("%.2f", value))
		newVol := m.playerctl("volume")
		m.updateAndNotify()
		return fmt.Sprintf("volume=%s", newVol), nil
	}

	amount := defaultVolumeDelta

	if len(args) > 1 {
		parsed, err := strconv.ParseFloat(args[1], 64)
		if err == nil {
			amount = parsed
		}
	}

	sign := "+"
	if direction == "down" {
		sign = "-"
	}

	volArg := fmt.Sprintf("%s%.2f", sign, amount)
	m.playerctlRun("volume", volArg)

	newVol := m.playerctl("volume")
	m.updateAndNotify()
	return fmt.Sprintf("volume=%s", newVol), nil
}

func (m *Music) changeSeek(direction string) (string, error) {
	sign := "+"
	if direction == "down" {
		sign = "-"
	}

	seekArg := fmt.Sprintf("%d%s", defaultSeekDelta, sign)
	m.playerctlRun("position", seekArg)

	return fmt.Sprintf("seek %s%ds", sign, defaultSeekDelta), nil
}
