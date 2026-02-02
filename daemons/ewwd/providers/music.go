package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	musicPollInterval  = 2 * time.Second
	musicPlayer        = "spotify"
	albumArtPath       = "/tmp/eww/album_art.png"
	defaultVolumeDelta = 0.05 // 5%
	defaultSeekDelta   = 10   // 10 seconds
)

// MusicState represents current playback state for the eww statusbar.
type MusicState struct {
	Status  string `json:"status"`   // "Playing", "Paused", or "Stopped"
	Volume  string `json:"volume"`   // Range: "0.0" to "1.0"
	Artist  string `json:"artist"`   // Current track artist
	Album   string `json:"album"`    // Current track album
	Title   string `json:"title"`    // Current track title
	ArtPath string `json:"art_path"` // Local filesystem path to cached album art
}

// Music monitors Spotify playback via playerctl, caching album art and providing playback control actions.
type Music struct {
	state      StateSetter      // State storage backend
	notify     func(data any)   // Callback for state changes
	done       chan struct{}    // Shutdown signal
	active     bool             // Whether provider is running
	last       MusicState       // Last known state for change detection
	lastArtURL string           // URL of currently cached album art
	mu         sync.Mutex       // Protects state and lastArtURL
	wg         sync.WaitGroup   // Tracks background album art downloads
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
		Status:  parts[0],
		Volume:  parts[1],
		Artist:  parts[2],
		Album:   parts[3],
		Title:   parts[4],
		ArtPath: albumArtPath,
	}

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
		state.Title != m.last.Title {

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
		state.Title != m.last.Title {

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
		return MusicState{
			Status:  "Stopped",
			Volume:  "0",
			Artist:  "Spotify",
			Album:   "Waiting for player to start...",
			Title:   "Waiting for player to start...",
			ArtPath: albumArtPath,
		}
	}

	volume := m.playerctl("volume")
	artist := m.playerctlMetadata("artist")
	album := m.playerctlMetadata("album")
	title := m.playerctlMetadata("title")
	artURL := m.playerctlMetadata("mpris:artUrl")

	// Download album art if available
	if artURL != "" && artURL != m.lastArtURL {
		m.wg.Go(func() { m.downloadAlbumArt(artURL) })
		m.lastArtURL = artURL
	}

	return MusicState{
		Status:  status,
		Volume:  volume,
		Artist:  artist,
		Album:   album,
		Title:   title,
		ArtPath: albumArtPath,
	}
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

// changeVolume adjusts player volume.
func (m *Music) changeVolume(args []string) (string, error) {
	direction := args[0]
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
