package providers

// canvas_client.go shares Spotify web access tokens between Canvas and the hidden Connect observer.
//
// Firefox's sp_dc cookie and Spotify's web-player TOTP supply access tokens; the shared cookie reader also supplies sp_t for Connect client tokens.
// Log into open.spotify.com in Firefox to refresh missing or expired cookies.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

const (
	canvasURL     = "https://spclient.wg.spotify.com/canvaz-cache/v0/canvases"
	totpVersion   = 61
	totpPeriod    = 30
	totpDigits    = 6
	totpRaw       = `,7/*F("rLJ2oxaKL^f+E1xvP@N`
	cookieTimeout = 3 * time.Second
	tokenTimeout  = 10 * time.Second
	tokenMinLife  = 30 * time.Second
)

type spotifyToken struct {
	AccessToken string `json:"accessToken"`
	ExpiresMs   int64  `json:"accessTokenExpirationTimestampMs"`
	ClientID    string `json:"clientId"`
}

// CanvasClient authenticates to Spotify's private Canvas endpoint and caches the web access token.
type CanvasClient struct {
	spDc  string
	token spotifyToken
	mu    sync.Mutex
}

// NewCanvasClient returns nil when no sp_dc cookie can be resolved.
func NewCanvasClient(spDc string) *CanvasClient {
	if spDc == "" {
		spDc = firefoxCookie(context.Background(), "sp_dc")
	}
	if spDc == "" {
		fmt.Fprintln(os.Stderr, "ewwd: canvas disabled — no sp_dc cookie found (log into open.spotify.com in Firefox)")
		return nil
	}
	return &CanvasClient{spDc: spDc}
}

func (c *CanvasClient) accessToken(ctx context.Context) (string, error) {
	token, err := c.webToken(ctx, tokenMinLife)
	return token.AccessToken, err
}

func (c *CanvasClient) webToken(ctx context.Context, minLife time.Duration) (spotifyToken, error) {
	if err := c.refreshToken(ctx, minLife); err != nil {
		return spotifyToken{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token, nil
}

func (c *CanvasClient) refreshToken(ctx context.Context, minLife time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token.AccessToken != "" && time.Now().UnixMilli() < c.token.ExpiresMs-minLife.Milliseconds() {
		return nil
	}

	tok, err := c.fetchToken(ctx, c.spDc)
	if err != nil {
		if status, ok := errors.AsType[*spotifyStatusError](err); ok && status.blocked() && status.code != http.StatusUnauthorized {
			return err
		}
		// Cookie may have rotated; re-read from Firefox and retry once.
		fresh := firefoxCookie(ctx, "sp_dc")
		if fresh == "" || fresh == c.spDc {
			return err
		}
		c.spDc = fresh
		tok, err = c.fetchToken(ctx, fresh)
		if err != nil {
			return err
		}
	}

	c.token = tok
	return nil
}

func (c *CanvasClient) fetchToken(ctx context.Context, spDc string) (spotifyToken, error) {
	now := time.Now()
	params := url.Values{
		"reason":      {"transport"},
		"productType": {"web_player"},
		"totp":        {spotifyTOTP(now.UnixMilli())},
		"totpVer":     {strconv.Itoa(totpVersion)},
	}
	tokenURL := "https://open.spotify.com/api/token?" + params.Encode()

	ctx, cancel := context.WithTimeout(ctx, tokenTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return spotifyToken{}, err
	}
	req.Header.Set("Cookie", "sp_dc="+spDc)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return spotifyToken{}, fmt.Errorf("token fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return spotifyToken{}, newSpotifyStatusError("token fetch", resp)
	}

	var tok spotifyToken
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return spotifyToken{}, fmt.Errorf("token decode: %w", err)
	}
	if tok.AccessToken == "" {
		return spotifyToken{}, fmt.Errorf("token fetch: empty access token (sp_dc may be expired)")
	}

	return tok, nil
}

// spotifyTOTP generates the code Spotify's web player sends with token requests.
func spotifyTOTP(timestampMs int64) string {
	secret := deobfuscateTOTPSecret()
	counter := uint64(timestampMs/1000) / totpPeriod
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	mac := hmac.New(sha1.New, secret)
	mac.Write(counterBytes)
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0f
	code := binary.BigEndian.Uint32(h[offset:offset+4]) & 0x7fffffff
	otp := code % 1_000_000
	return fmt.Sprintf("%06d", otp)
}

func deobfuscateTOTPSecret() []byte {
	nums := make([]int, len(totpRaw))
	for i, ch := range totpRaw {
		nums[i] = int(ch) ^ (i%33 + 9)
	}
	var joined strings.Builder
	for _, n := range nums {
		joined.WriteString(strconv.Itoa(n))
	}
	hexStr := hex.EncodeToString([]byte(joined.String()))
	secret, _ := hex.DecodeString(hexStr)
	return secret
}

// firefoxCookie reads a Spotify cookie from Firefox's cookies.sqlite database.
func firefoxCookie(ctx context.Context, name string) string {
	cookiesDB := findFirefoxCookiesDB()
	if cookiesDB == "" {
		return ""
	}

	queryCtx, cancel := context.WithTimeout(ctx, cookieTimeout)
	defer cancel()
	dsn := fmt.Sprintf("file:%s?mode=ro&immutable=1", cookiesDB)
	out, err := exec.CommandContext(queryCtx, "sqlite3", dsn,
		"SELECT value FROM moz_cookies WHERE host = '.spotify.com' AND name = '"+name+"' LIMIT 1",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func findFirefoxCookiesDB() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	roots := []string{
		filepath.Join(home, ".mozilla", "firefox"),
		filepath.Join(home, ".config", "mozilla", "firefox"),
	}

	var fallback string
	for _, root := range roots {
		f, err := os.Open(filepath.Join(root, "profiles.ini"))
		if err != nil {
			continue
		}

		section, name, path := "", "", ""
		scanner := bufio.NewScanner(f)
		flush := func() {
			if !strings.HasPrefix(section, "Profile") || path == "" {
				return
			}
			db := filepath.Join(root, path, "cookies.sqlite")
			if _, err := os.Stat(db); err != nil {
				return
			}
			if name == "dev-edition-default" {
				fallback = db
				return
			}
			if fallback == "" {
				fallback = db
			}
		}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				flush()
				section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
				name, path = "", ""
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "Name":
				name = strings.TrimSpace(value)
			case "Path":
				path = strings.TrimSpace(value)
			}
		}
		flush()
		f.Close()
		if fallback != "" && strings.Contains(fallback, "dev-edition-default") {
			return fallback
		}
	}
	return fallback
}

// FetchCanvasURL returns the CDN URL for a track's Canvas MP4, or "" when none exists.
func (c *CanvasClient) FetchCanvasURL(ctx context.Context, trackURI string) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}

	reqMsg := &CanvasRequest{
		Tracks: []*CanvasRequest_Track{
			{TrackUri: trackURI},
		},
	}

	body, err := proto.Marshal(reqMsg)
	if err != nil {
		return "", fmt.Errorf("proto marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, canvasURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "application/protobuf")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("canvas fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("canvas fetch: status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("canvas read: %w", err)
	}

	var respMsg CanvasResponse
	if err := proto.Unmarshal(respBody, &respMsg); err != nil {
		return "", fmt.Errorf("proto unmarshal: %w", err)
	}

	for _, canvas := range respMsg.Canvases {
		if canvas.CanvasUrl != "" {
			return canvas.CanvasUrl, nil
		}
	}

	return "", nil
}

// DownloadCanvasData fetches the Canvas MP4 bytes from cdnURL.
func DownloadCanvasData(ctx context.Context, cdnURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdnURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("canvas download: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
