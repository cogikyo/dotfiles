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

// MusicState represents current playback state for the eww statusbar.
type MusicState struct {
	Status        string `json:"status"`         // "Playing", "Paused", or "Stopped"
	Playing       bool   `json:"playing"`        // Convenience flag for active playback state
	Volume        string `json:"volume"`         // Range: "0.0" to "1.0"
	VolumePercent int    `json:"volume_percent"` // Playback volume scaled to 0-100 for widgets
	Artist        string `json:"artist"`         // Current track artist
	Album         string `json:"album"`          // Current track album
	AlbumShort    string `json:"album_short"`    // Widget-friendly truncated album name
	Title         string `json:"title"`          // Current track title
	TitleShort    string `json:"title_short"`    // Widget-friendly truncated song title
	SingleTrack   bool   `json:"single_track"`   // Album and title collapse to one display line
	Progress      int    `json:"progress"`       // Playback progress percentage (0-100)
	ArtPath       string `json:"art_path"`       // Local filesystem path to cached album art
}

// Music monitors Spotify playback via playerctl, caching album art and providing playback control actions.
type Music struct {
	state      StateSetter    // State storage backend
	notify     func(data any) // Callback for state changes
	done       chan struct{}  // Shutdown signal
	active     bool           // Whether provider is running
	last       MusicState     // Last known state for change detection
	lastArtURL string         // URL of currently cached album art
	mu         sync.Mutex     // Protects state and lastArtURL
	wg         sync.WaitGroup // Tracks background album art downloads
}

// NewMusic creates a Music provider for Spotify playback monitoring.
func NewMusic(state StateSetter) Provider {
	return &Music{
		state: state,
		done:  make(chan struct{}),
	}
}

func (m *Music) Name() string {
	return "music"
}

// Start monitors Spotify playback using playerctl follow mode for real-time updates, with a fallback polling mechanism.
func (m *Music) Start(ctx context.Context, notify func(data any)) error {
	m.active = true
	m.notify = notify

	// Ensure album art directory exists
	if err := os.MkdirAll(filepath.Dir(albumArtPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "music: failed to create album art directory: %v\n", err)
	}

	// Send initial state
	m.updateAndNotify()

	// Start playerctl follow mode in background
	go m.followMode(ctx)

	// Fallback poll ticker in case follow mode fails
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

// Stop shuts down the provider and waits for pending album art downloads to complete.
func (m *Music) Stop() error {
	if m.active {
		close(m.done)
		m.active = false
		m.wg.Wait() // Wait for album art downloads to complete
	}
	return nil
}

// followMode continuously streams metadata changes from playerctl, automatically restarting on failure.
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

		// playerctl died, wait before retry
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case <-time.After(time.Second):
		}
	}
}

// parseFollowLine parses tab-separated playerctl output and triggers album art downloads when URLs change.
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

	// Download album art if URL changed
	if artURL != "" && artURL != m.lastArtURL {
		m.wg.Go(func() { m.downloadAlbumArt(artURL) })
		m.lastArtURL = artURL
	}

	// Only notify if state changed (excluding art path comparison)
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

// updateAndNotify polls playerctl for current state and notifies only if state has changed.
func (m *Music) updateAndNotify() {
	state := m.getCurrentState()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Only notify if state changed
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

// getCurrentState queries playerctl for all metadata fields and returns a default "waiting" state if player is unavailable.
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

	// Download album art if available
	if artURL != "" && artURL != m.lastArtURL {
		m.wg.Go(func() { m.downloadAlbumArt(artURL) })
		m.lastArtURL = artURL
	}

	return musicState(status, volume, artist, album, title, progress)
}

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

// playerctl executes a playerctl command and returns trimmed stdout, or empty string on error.
func (m *Music) playerctl(cmd string) string {
	out, err := exec.Command("playerctl", "--player="+musicPlayer, cmd).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// playerctlMetadata retrieves a specific metadata field from playerctl, returning empty string on error.
func (m *Music) playerctlMetadata(field string) string {
	out, err := exec.Command("playerctl", "--player="+musicPlayer, "metadata", field).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// playerctlFormat retrieves formatted playerctl metadata output, returning empty string on error.
func (m *Music) playerctlFormat(format string) string {
	out, err := exec.Command("playerctl", "--player="+musicPlayer, "metadata", "--format", format).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getProgress returns playback progress as a percentage in the range 0-100.
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

// downloadAlbumArt fetches album art from URL and atomically writes it to the cache path.
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

	// Create temp file first, then rename for atomic write
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

// playerctlRun executes a playerctl command, logging errors to stderr.
func (m *Music) playerctlRun(args ...string) {
	fullArgs := append([]string{"--player=" + musicPlayer}, args...)
	if err := exec.Command("playerctl", fullArgs...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: playerctl %s error: %v\n", args[0], err)
	}
}

// HandleAction processes playback control commands (play, pause, toggle, next, previous, volume, seek).
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

// changeVolume adjusts player volume, or sets an absolute value when asked.
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

	// Get new volume and return
	newVol := m.playerctl("volume")
	m.updateAndNotify()
	return fmt.Sprintf("volume=%s", newVol), nil
}

// changeSeek adjusts playback position.
func (m *Music) changeSeek(direction string) (string, error) {
	sign := "+"
	if direction == "down" {
		sign = "-"
	}

	seekArg := fmt.Sprintf("%d%s", defaultSeekDelta, sign)
	m.playerctlRun("position", seekArg)

	return fmt.Sprintf("seek %s%ds", sign, defaultSeekDelta), nil
}
