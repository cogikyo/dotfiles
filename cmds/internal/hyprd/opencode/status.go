package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const serverURL = "http://127.0.0.1:4096"

type apiClient struct {
	http *http.Client
}

type project struct {
	ID        string   `json:"id"`
	Worktree  string   `json:"worktree"`
	Sandboxes []string `json:"sandboxes"`
}

type sessionStatus struct {
	Type string `json:"type"`
}

func newAPIClient() *apiClient {
	return &apiClient{http: &http.Client{Timeout: 3 * time.Second}}
}

func (a *apiClient) health(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/global/health", nil)
	if err != nil {
		return err
	}
	response, err := a.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", response.Status)
	}
	var health struct {
		Healthy bool `json:"healthy"`
	}
	if err := decodeJSON(response.Body, &health); err != nil {
		return err
	}
	if !health.Healthy {
		return fmt.Errorf("backend reported unhealthy")
	}
	return nil
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

func (a *apiClient) busy(ctx context.Context, base []string) (bool, string, error) {
	if err := a.health(ctx); err != nil {
		return false, "", err
	}

	directories := make(map[string]struct{}, len(base))
	for _, directory := range base {
		addDirectory(directories, directory)
	}

	projects, err := a.projects(ctx)
	if err != nil {
		return false, "", err
	}
	for _, project := range projects {
		if project.ID != "global" && cleanDirectory(project.Worktree) != string(filepath.Separator) {
			addDirectory(directories, project.Worktree)
		}
		for _, sandbox := range project.Sandboxes {
			addDirectory(directories, sandbox)
		}
	}
	if len(directories) == 0 {
		return false, "", nil
	}

	type result struct {
		directory string
		idle      bool
		reason    string
	}
	results := make(chan result, len(directories))
	var group sync.WaitGroup
	for directory := range directories {
		group.Go(func() {
			idle, reason := a.directoryIdle(ctx, directory)
			results <- result{directory: directory, idle: idle, reason: reason}
		})
	}
	group.Wait()
	close(results)

	var blocked []string
	for result := range results {
		if !result.idle {
			blocked = append(blocked, fmt.Sprintf("%s: %s", result.directory, result.reason))
		}
	}
	sort.Strings(blocked)
	return len(blocked) > 0, strings.Join(blocked, "; "), nil
}

func (a *apiClient) projects(ctx context.Context) ([]project, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/project", nil)
	if err != nil {
		return nil, err
	}
	response, err := a.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", response.Status)
	}
	var projects []project
	if err := decodeJSON(response.Body, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (a *apiClient) directoryIdle(ctx context.Context, directory string) (bool, string) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/session/status", nil)
	if err != nil {
		return false, err.Error()
	}
	request.Header.Set("x-opencode-directory", directory)
	response, err := a.http.Do(request)
	if err != nil {
		return false, err.Error()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, "HTTP " + response.Status
	}
	var statuses map[string]sessionStatus
	if err := decodeJSON(response.Body, &statuses); err != nil {
		return false, "invalid status: " + err.Error()
	}
	if statuses == nil {
		return false, "invalid null status"
	}
	for id, status := range statuses {
		switch status.Type {
		case "idle":
			continue
		case "busy", "retry":
			return false, id + " is " + status.Type
		default:
			return false, id + " has unknown status " + fmt.Sprintf("%q", status.Type)
		}
	}
	return true, ""
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
