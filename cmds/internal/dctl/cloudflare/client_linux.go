package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const api = "https://api.cloudflare.com/client/v4"

var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type client struct {
	http  *http.Client
	token string
	write bool
	sent  int
}

type apiMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiError struct {
	Method string
	Path   string
	Status int
	Errors []apiMessage
}

func (e *apiError) Error() string {
	var detail strings.Builder
	fmt.Fprintf(&detail, "Cloudflare refused %s %s (HTTP %d)", e.Method, e.Path, e.Status)
	for _, m := range e.Errors {
		fmt.Fprintf(&detail, "; code %d: %s", m.Code, m.Message)
	}
	return detail.String()
}

func newClient(home string) (*client, error) {
	token, err := loadToken(home)
	if err != nil {
		return nil, err
	}
	return &client{
		token: token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func endpoint(segments []string) (string, error) {
	for _, s := range segments {
		if !segmentPattern.MatchString(s) {
			return "", fmt.Errorf("refusing Cloudflare path segment %q; IDs and names must be plain ASCII words", s)
		}
	}
	return "/" + strings.Join(segments, "/"), nil
}

func (c *client) request(ctx context.Context, method string, query url.Values, body, out any, segments ...string) (*envelope, error) {
	path, err := endpoint(segments)
	if err != nil {
		return nil, err
	}
	if method != http.MethodGet && !c.write {
		return nil, fmt.Errorf("read-only run refused %s %s; no request sent", method, path)
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("cannot encode Cloudflare request for %s %s", method, path)
		}
		reader = bytes.NewReader(data)
	}
	target := api + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("cannot construct Cloudflare request for %s %s", method, path)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.sent++
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloudflare %s %s failed (transport, timeout, or cancellation); no automatic retry", method, path)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 {
		return nil, fmt.Errorf("cannot read bounded Cloudflare response for %s %s (HTTP %d)", method, path, res.StatusCode)
	}
	data = bytes.ReplaceAll(data, []byte(c.token), []byte("[REDACTED]"))

	var env envelope
	if json.Unmarshal(data, &env) != nil {
		return nil, fmt.Errorf("invalid Cloudflare JSON for %s %s (HTTP %d)", method, path, res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || !env.Success {
		return nil, &apiError{Method: method, Path: path, Status: res.StatusCode, Errors: env.Errors}
	}
	if out != nil && json.Unmarshal(env.Result, out) != nil {
		return nil, fmt.Errorf("unexpected Cloudflare result shape for %s %s", method, path)
	}
	return &env, nil
}

type envelope struct {
	Success bool            `json:"success"`
	Errors  []apiMessage    `json:"errors"`
	Result  json.RawMessage `json:"result"`
	Info    *struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

func (c *client) get(ctx context.Context, out any, query url.Values, segments ...string) error {
	_, err := c.request(ctx, http.MethodGet, query, nil, out, segments...)
	return err
}

func list[T any](ctx context.Context, c *client, query url.Values, segments ...string) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		q := url.Values{}
		maps.Copy(q, query)
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", "50")

		var items []T
		env, err := c.request(ctx, http.MethodGet, q, nil, &items, segments...)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if env.Info == nil || env.Info.TotalPages <= page {
			return all, nil
		}
		if page >= 100 {
			return nil, fmt.Errorf("Cloudflare listing %s exceeds 100 pages; refusing to guess at the rest", strings.Join(segments, "/"))
		}
	}
}

func isMissing(err error) bool {
	var e *apiError
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}
