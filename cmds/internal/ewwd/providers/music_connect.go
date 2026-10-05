package providers

// music_connect.go observes Spotify Connect as a hidden member and publishes the active device's previous and next tracks.
import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	connectStateURL       = "https://gue1-spclient.spotify.com/connect-state/v1/devices/"
	connectDealerURL      = "https://dealer.spotify.com/?access_token="
	clientTokenURL        = "https://clienttoken.spotify.com/v1/clienttoken"
	webPlayerURL          = "https://open.spotify.com/"
	trackMetadataURL      = "https://spclient.wg.spotify.com/metadata/4/track/"
	spotifyImageURL       = "https://i.scdn.co/image/"
	connectAppVersion     = "harmony:4.43.2-a61ecaf5"
	connectUserAgent      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	connectListLimit      = 5
	connectCacheLimit     = 512
	connectPingInterval   = 30 * time.Second
	connectPongTimeout    = 10 * time.Second
	connectRequestTimeout = 10 * time.Second
	connectMaxBackoff     = 5 * time.Minute
	hiddenMember          = `{"member_type":"CONNECT_STATE","device":{"device_info":{"capabilities":{"can_be_player":false,"hidden":true,"needs_full_player_state":true}}}}`
	base62                = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var appServerConfig = regexp.MustCompile(`<script id="appServerConfig" type="text/plain">([^<]+)</script>`)

type connectObserver struct {
	canvas  *CanvasClient
	device  string
	tracks  map[string]MusicTrack
	publish func(history, queue []MusicTrack)
}

type connectAuth struct {
	access, client string
}

type dealerMessage struct {
	Type     string            `json:"type"`
	URI      string            `json:"uri"`
	Headers  map[string]string `json:"headers"`
	Payloads []json.RawMessage `json:"payloads"`
}

type connectCluster struct {
	ActiveDeviceID string `json:"active_device_id"`
	PlayerState    struct {
		PrevTracks []connectTrack `json:"prev_tracks"`
		NextTracks []connectTrack `json:"next_tracks"`
	} `json:"player_state"`
}

type connectTrack struct {
	URI      string         `json:"uri"`
	Metadata map[string]any `json:"metadata"`
}

type coverImage struct {
	FileID string `json:"file_id"`
	Size   string `json:"size"`
}

type spotifyStatusError struct {
	op         string
	code       int
	reason     string
	retryAfter time.Duration
}

func newConnectObserver(canvas *CanvasClient, publish func(history, queue []MusicTrack)) *connectObserver {
	var id [16]byte
	_, _ = rand.Read(id[:])
	return &connectObserver{canvas: canvas, device: "hobs_" + hex.EncodeToString(id[:]), tracks: map[string]MusicTrack{}, publish: publish}
}

func (o *connectObserver) run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := o.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			o.publish([]MusicTrack{}, []MusicTrack{})
		}
		status, isStatus := errors.AsType[*spotifyStatusError](err)
		if isStatus && status.blocked() {
			fmt.Fprintf(os.Stderr, "ewwd: music connect disabled until restart: %v\n", err)
			return
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second
		}
		delay := backoff
		if isStatus {
			delay = max(delay, status.retryAfter)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: music connect: %v (retry in %s)\n", err, delay)
		}
		if !waitContext(ctx, delay) {
			return
		}
		backoff = min(backoff*2, connectMaxBackoff)
	}
}

func (o *connectObserver) session(ctx context.Context) error {
	token, err := o.canvas.webToken(ctx)
	if err != nil {
		return err
	}
	auth := connectAuth{access: token.AccessToken}
	if auth.client, err = o.clientToken(ctx, token.ClientID); err != nil {
		return err
	}
	expiry := time.UnixMilli(token.ExpiresMs).Add(-time.Minute)
	if floor := time.Now().Add(time.Minute); expiry.Before(floor) {
		expiry = floor
	}
	sessionCtx, cancel := context.WithDeadline(ctx, expiry)
	defer cancel()

	socket, err := dialDealer(sessionCtx, connectDealerURL+url.QueryEscape(auth.access))
	if err != nil {
		return err
	}
	var lastRead atomic.Int64
	lastRead.Store(time.Now().UnixNano())
	var pinger sync.WaitGroup
	pinger.Go(func() { keepDealerAlive(sessionCtx, socket, &lastRead) })
	stopClose := context.AfterFunc(sessionCtx, socket.Close)

	connection := ""
	defer func() {
		cancel()
		stopClose()
		pinger.Wait()
		if connection != "" {
			o.unregister(ctx, auth, connection)
		}
		socket.Close()
	}()

	for {
		raw, err := socket.ReadMessage()
		if err != nil {
			if sessionCtx.Err() != nil {
				return nil
			}
			return fmt.Errorf("dealer read: %w", err)
		}
		lastRead.Store(time.Now().UnixNano())
		var msg dealerMessage
		if json.Unmarshal(raw, &msg) != nil || msg.Type != "message" {
			continue
		}
		switch {
		case strings.HasPrefix(msg.URI, "hm://pusher/v1/connections/"):
			id := msg.Headers["Spotify-Connection-Id"]
			if id == "" {
				continue
			}
			cluster, err := o.register(sessionCtx, auth, id)
			if err != nil {
				return err
			}
			connection = id
			o.apply(sessionCtx, auth, cluster)
		case strings.HasPrefix(msg.URI, "hm://connect-state/v1/cluster"):
			payload, err := dealerPayload(msg)
			if err != nil {
				continue
			}
			var update struct {
				Cluster connectCluster `json:"cluster"`
			}
			if json.Unmarshal(payload, &update) != nil {
				continue
			}
			o.apply(sessionCtx, auth, update.Cluster)
		}
	}
}

func keepDealerAlive(ctx context.Context, socket *dealerSocket, lastRead *atomic.Int64) {
	ticker := time.NewTicker(connectPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if time.Since(time.Unix(0, lastRead.Load())) > connectPingInterval+connectPongTimeout {
			socket.Close()
			return
		}
		if err := socket.WriteText([]byte(`{"type":"ping"}`)); err != nil {
			socket.Close()
			return
		}
	}
}

func dealerPayload(msg dealerMessage) ([]byte, error) {
	if len(msg.Payloads) == 0 {
		return nil, errors.New("dealer message has no payload")
	}
	var encoded string
	if json.Unmarshal(msg.Payloads[0], &encoded) != nil {
		return msg.Payloads[0], nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || msg.Headers["Transfer-Encoding"] != "gzip" {
		return data, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (o *connectObserver) clientToken(ctx context.Context, clientID string) (string, error) {
	if clientID == "" {
		return "", errors.New("clienttoken: access token response has no clientId")
	}
	device := firefoxCookie(ctx, "sp_t")
	if device == "" {
		return "", errors.New("clienttoken: no sp_t cookie found (log into open.spotify.com in Firefox)")
	}
	version, err := webPlayerVersion(ctx)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{"client_data": map[string]any{
		"client_version": version,
		"client_id":      clientID,
		"js_sdk_data": map[string]string{
			"device_brand": "unknown", "device_model": "unknown", "os": "linux",
			"os_version": "unknown", "device_id": device, "device_type": "computer",
		},
	}})
	if err != nil {
		return "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, connectRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, clientTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", connectUserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	var granted struct {
		Token struct {
			Token string `json:"token"`
		} `json:"granted_token"`
	}
	if err := doSpotifyJSON(req, "clienttoken", &granted); err != nil {
		return "", err
	}
	if granted.Token.Token == "" {
		return "", errors.New("clienttoken: no token granted")
	}
	return granted.Token.Token, nil
}

func webPlayerVersion(ctx context.Context) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, connectRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, webPlayerURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", connectUserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("web player config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", newSpotifyStatusError("web player config", resp)
	}
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("web player config: %w", err)
	}
	match := appServerConfig.FindSubmatch(page)
	if match == nil {
		return "", errors.New("web player config: no appServerConfig")
	}
	raw, err := base64.StdEncoding.DecodeString(string(match[1]))
	if err != nil {
		return "", fmt.Errorf("web player config: %w", err)
	}
	var config struct {
		ClientVersion string `json:"clientVersion"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", fmt.Errorf("web player config: %w", err)
	}
	version, _, _ := strings.Cut(config.ClientVersion, ".g")
	if version == "" {
		return "", errors.New("web player config: no clientVersion")
	}
	return version, nil
}

func (o *connectObserver) register(ctx context.Context, auth connectAuth, connection string) (connectCluster, error) {
	reqCtx, cancel := context.WithTimeout(ctx, connectRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, connectStateURL+o.device, strings.NewReader(hiddenMember))
	if err != nil {
		return connectCluster{}, err
	}
	auth.apply(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Spotify-Connection-Id", connection)
	var cluster connectCluster
	err = doSpotifyJSON(req, "connect-state register", &cluster)
	return cluster, err
}

func (o *connectObserver) unregister(ctx context.Context, auth connectAuth, connection string) {
	reqCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, connectStateURL+o.device, nil)
	if err != nil {
		return
	}
	auth.apply(req)
	req.Header.Set("X-Spotify-Connection-Id", connection)
	if resp, err := http.DefaultClient.Do(req); err == nil {
		resp.Body.Close()
	}
}

func (a connectAuth) apply(req *http.Request) {
	req.Header.Set("User-Agent", connectUserAgent)
	req.Header.Set("Authorization", "Bearer "+a.access)
	req.Header.Set("Client-Token", a.client)
	req.Header.Set("Spotify-App-Version", connectAppVersion)
	req.Header.Set("App-Platform", "WebPlayer")
}

func (o *connectObserver) apply(ctx context.Context, auth connectAuth, cluster connectCluster) {
	if cluster.ActiveDeviceID == "" {
		o.publish([]MusicTrack{}, []MusicTrack{})
		return
	}
	previous := slices.Clone(cluster.PlayerState.PrevTracks)
	slices.Reverse(previous)
	history, queue := playableTracks(previous), playableTracks(cluster.PlayerState.NextTracks)
	o.hydrate(ctx, auth, slices.Concat(history, queue))
	o.publish(o.resolve(history), o.resolve(queue))
}

func playableTracks(entries []connectTrack) []connectTrack {
	tracks := make([]connectTrack, 0, connectListLimit)
	for _, entry := range entries {
		if len(tracks) == connectListLimit {
			break
		}
		hidden := entry.Metadata["hidden"] == "true"
		if hidden || (!strings.HasPrefix(entry.URI, "spotify:track:") && !strings.HasPrefix(entry.URI, "spotify:episode:")) {
			continue
		}
		tracks = append(tracks, entry)
	}
	return tracks
}

func (entry connectTrack) metadataTrack() MusicTrack {
	text := func(key string) string {
		value, _ := entry.Metadata[key].(string)
		return value
	}
	art := text("image_url")
	if id, ok := strings.CutPrefix(art, "spotify:image:"); ok {
		art = spotifyImageURL + id
	} else if !strings.HasPrefix(art, "https://") {
		art = ""
	}
	return MusicTrack{Title: text("title"), Artist: text("artist_name"), ArtURL: art}
}

func (o *connectObserver) hydrate(ctx context.Context, auth connectAuth, entries []connectTrack) {
	var missing []string
	for _, entry := range entries {
		track := entry.metadataTrack()
		_, cached := o.tracks[entry.URI]
		complete := track.Title != "" && track.Artist != "" && track.ArtURL != ""
		if complete || cached || !strings.HasPrefix(entry.URI, "spotify:track:") || slices.Contains(missing, entry.URI) {
			continue
		}
		missing = append(missing, entry.URI)
	}
	if len(missing) == 0 {
		return
	}
	if len(o.tracks)+len(missing) > connectCacheLimit {
		clear(o.tracks)
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, uri := range missing {
		wg.Go(func() {
			track, err := trackMetadata(ctx, auth, uri)
			if err != nil {
				return
			}
			mu.Lock()
			o.tracks[uri] = track
			mu.Unlock()
		})
	}
	wg.Wait()
}

func (o *connectObserver) resolve(entries []connectTrack) []MusicTrack {
	tracks := make([]MusicTrack, 0, len(entries))
	for _, entry := range entries {
		track, cached := entry.metadataTrack(), o.tracks[entry.URI]
		track.Title = cmp.Or(track.Title, cached.Title)
		track.Artist = cmp.Or(track.Artist, cached.Artist)
		track.ArtURL = cmp.Or(track.ArtURL, cached.ArtURL)
		if track.Title != "" {
			tracks = append(tracks, track)
		}
	}
	return tracks
}

func trackMetadata(ctx context.Context, auth connectAuth, uri string) (MusicTrack, error) {
	gid, err := spotifyGID(strings.TrimPrefix(uri, "spotify:track:"))
	if err != nil {
		return MusicTrack{}, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, connectRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, trackMetadataURL+gid+"?market=from_token", nil)
	if err != nil {
		return MusicTrack{}, err
	}
	auth.apply(req)
	req.Header.Set("Accept", "application/json")
	var metadata struct {
		Name   string `json:"name"`
		Artist []struct {
			Name string `json:"name"`
		} `json:"artist"`
		Album struct {
			CoverGroup struct {
				Image []coverImage `json:"image"`
			} `json:"cover_group"`
		} `json:"album"`
	}
	if err := doSpotifyJSON(req, "track metadata", &metadata); err != nil {
		return MusicTrack{}, err
	}
	artists := make([]string, 0, len(metadata.Artist))
	for _, artist := range metadata.Artist {
		artists = append(artists, artist.Name)
	}
	track := MusicTrack{Title: metadata.Name, Artist: strings.Join(artists, ", ")}
	images := metadata.Album.CoverGroup.Image
	if i := slices.IndexFunc(images, func(image coverImage) bool { return image.Size == "DEFAULT" }); i >= 0 {
		track.ArtURL = spotifyImageURL + images[i].FileID
	} else if len(images) > 0 {
		track.ArtURL = spotifyImageURL + images[0].FileID
	}
	return track, nil
}

func spotifyGID(id string) (string, error) {
	n := new(big.Int)
	for _, ch := range id {
		digit := strings.IndexRune(base62, ch)
		if digit < 0 {
			return "", fmt.Errorf("invalid Spotify id %q", id)
		}
		n.Mul(n, big.NewInt(62)).Add(n, big.NewInt(int64(digit)))
	}
	if n.BitLen() > 128 {
		return "", fmt.Errorf("invalid Spotify id %q", id)
	}
	return fmt.Sprintf("%032x", n), nil
}

func doSpotifyJSON(req *http.Request, op string, out any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return newSpotifyStatusError(op, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s decode: %w", op, err)
	}
	return nil
}

func newSpotifyStatusError(op string, resp *http.Response) *spotifyStatusError {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var body struct {
		Error struct {
			Reason string `json:"reason"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &body)
	err := &spotifyStatusError{op: op, code: resp.StatusCode, reason: body.Error.Reason}
	if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
		err.retryAfter = time.Duration(seconds) * time.Second
	}
	return err
}

func (e *spotifyStatusError) Error() string {
	if e.reason == "" {
		return fmt.Sprintf("%s: status %d", e.op, e.code)
	}
	return fmt.Sprintf("%s: status %d %s", e.op, e.code, e.reason)
}

func (e *spotifyStatusError) blocked() bool {
	switch e.code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return true
	case http.StatusTooManyRequests:
		return e.reason == "QUOTA_EXCEEDED"
	default:
		return false
	}
}
