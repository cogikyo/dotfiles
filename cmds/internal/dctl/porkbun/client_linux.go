package porkbun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type client struct {
	http  *http.Client
	creds credentials
}

type response struct {
	Status       string          `json:"status"`
	Code         string          `json:"code"`
	Message      string          `json:"message"`
	Warnings     any             `json:"warnings"` // Porkbun does not document a stable warnings array
	Records      json.RawMessage `json:"records"`
	ID           string          `json:"id"` // create-response id; documented JSON type is string
	DryRun       bool            `json:"dryRun"`
	WouldSucceed bool            `json:"wouldSucceed"`
	Domain       json.RawMessage `json:"domain"`
}

func newClient(home string) (*client, error) {
	creds, err := loadCredentials(home)
	if err != nil {
		return nil, err
	}

	return &client{
		creds: creds,
		http: &http.Client{
			Timeout: 20 * time.Second,
			// Refuse redirects so a 3xx is not treated as success.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *client) request(ctx context.Context, method, path string, body any) (response, error) {
	var result response
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return result, fmt.Errorf("cannot encode Porkbun request")
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://api.porkbun.com/api/json/v3"+path, reader)
	if err != nil {
		return result, fmt.Errorf("cannot construct Porkbun request")
	}
	req.Header.Set("X-API-Key", c.creds.key)
	req.Header.Set("X-Secret-API-Key", c.creds.secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return result, fmt.Errorf("Porkbun request failed (transport, timeout, or cancellation); no automatic retry")
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 {
		return result, fmt.Errorf("cannot read bounded Porkbun response (HTTP %d)", res.StatusCode)
	}
	if json.Unmarshal(data, &result) != nil {
		return response{}, fmt.Errorf("invalid Porkbun JSON response (HTTP %d)", res.StatusCode)
	}

	// Redact credentials only in diagnostic code, message, and warnings.
	// Records, domain metadata, and record IDs stay as returned API values.
	redact := strings.NewReplacer(c.creds.key, "[REDACTED]", c.creds.secret, "[REDACTED]")
	result.Code = redact.Replace(result.Code)
	result.Message = redact.Replace(result.Message)
	if result.Warnings != nil {
		raw, err := json.Marshal(result.Warnings)
		if err != nil {
			result.Warnings = "[REDACTED]"
		} else {
			redacted := redact.Replace(string(raw))
			if json.Unmarshal([]byte(redacted), &result.Warnings) != nil {
				result.Warnings = redacted
			}
		}
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 || result.Status != "SUCCESS" {
		detail := fmt.Sprintf("HTTP %d", res.StatusCode)
		if result.Code != "" {
			detail += fmt.Sprintf(", code %q", result.Code)
		}
		if result.Message != "" {
			detail += fmt.Sprintf(", message %q", result.Message)
		}
		return result, fmt.Errorf("Porkbun refused request (%s); check API keys, domain API access, permissions, and input in the Porkbun dashboard", detail)
	}
	return result, nil
}

func (r response) warnings() []string {
	if r.Warnings == nil {
		return nil
	}
	if items, ok := r.Warnings.([]any); ok && len(items) == 0 {
		return nil
	}
	data, _ := json.Marshal(r.Warnings)
	return []string{"Porkbun warning: " + string(data)}
}

func (c *client) authority(ctx context.Context, domain string) ([]string, error) {
	r, err := c.request(ctx, http.MethodGet, "/domain/get/"+domain, nil)
	warnings := r.warnings()
	if err != nil {
		return warnings, err
	}
	if len(warnings) > 0 {
		return warnings, fmt.Errorf("domain metadata carries warnings; authority is not safe for writes")
	}

	var metadata struct {
		Name     string `json:"domain"`
		NotLocal *int   `json:"notLocal"` // 0 means Porkbun reports the zone as authoritative
	}
	if json.Unmarshal(r.Domain, &metadata) != nil || metadata.Name != domain || metadata.NotLocal == nil {
		return warnings, fmt.Errorf("missing or unexpected domain authority evidence; require matching domain metadata with notLocal=0")
	}
	if *metadata.NotLocal != 0 {
		return warnings, fmt.Errorf("Porkbun reports notLocal=%d; refusing to write an inactive or unrecognized zone", *metadata.NotLocal)
	}
	return warnings, nil
}

func (c *client) records(ctx context.Context, domain string) ([]record, []string, error) {
	r, err := c.request(ctx, http.MethodGet, "/dns/retrieve/"+domain, nil)
	warnings := r.warnings()
	if err != nil {
		return nil, warnings, err
	}

	records, err := decodeRecords(domain, r.Records)
	return records, warnings, err
}
