package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
)

var (
	errAbsent      = errors.New("OpenCode service is not running")
	errUnavailable = errors.New("OpenCode service is unavailable")
)

type apiClient struct {
	http *http.Client
}

type registration struct {
	URL      string `json:"url"`
	Password string `json:"password"`
}

func newAPIClient() *apiClient {
	return &apiClient{http: &http.Client{Timeout: 3 * time.Second}}
}

func registrationPath() (string, error) {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		state = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(state, "opencode", "service.json"), nil
}

func readRegistration() (registration, error) {
	path, err := registrationPath()
	if err != nil {
		return registration{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return registration{}, fmt.Errorf("%w: no %s", errAbsent, path)
	}
	if err != nil {
		return registration{}, fmt.Errorf("service registration: %w", err)
	}
	var found registration
	if err := json.Unmarshal(data, &found); err != nil {
		return registration{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if found.URL == "" {
		return registration{}, fmt.Errorf("%s has no url", path)
	}
	return found, nil
}

func (a *apiClient) get(ctx context.Context, path string, target any) error {
	service, err := readRegistration()
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(service.URL, "/")+path, nil)
	if err != nil {
		return err
	}
	if service.Password != "" {
		request.SetBasicAuth("opencode", service.Password)
	}
	response, err := a.http.Do(request)
	if errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("%w: %s refused the connection", errAbsent, service.URL)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %s timed out", errUnavailable, path)
	}
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusServiceUnavailable {
		return fmt.Errorf("%w: %s is starting or stopping", errUnavailable, service.URL)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %s", path, response.Status)
	}
	if err := decodeJSON(response.Body, target); err != nil {
		return fmt.Errorf("%s: invalid response: %w", path, err)
	}
	return nil
}

func (a *apiClient) health(ctx context.Context) error {
	var info struct {
		Version string `json:"version"`
	}
	return a.get(ctx, "/api/info", &info)
}

func (a *apiClient) waitHealthy(ctx context.Context, ceiling time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, ceiling)
	defer cancel()
	var last error
	for {
		last = a.health(ctx)
		if last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (a *apiClient) active(ctx context.Context) ([]string, error) {
	var active struct {
		Data map[string]struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if err := a.get(ctx, "/api/session/active", &active); err != nil {
		return nil, err
	}
	if active.Data == nil {
		return nil, errors.New("/api/session/active: invalid response: missing data")
	}
	return slices.Sorted(maps.Keys(active.Data)), nil
}

func decodeJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
