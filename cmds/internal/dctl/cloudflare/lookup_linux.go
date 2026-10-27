package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

type lookup struct {
	http *http.Client
	sent int
}

func newLookup() *lookup {
	return &lookup{http: &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 5 {
				return errors.New("refusing a non-HTTPS or long redirect chain")
			}
			return nil
		},
	}}
}

func (l *lookup) fetch(ctx context.Context, target, accept string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", accept)
	l.sent++
	res, err := l.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s failed: %w", target, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return fmt.Errorf("cannot read bounded response from %s (HTTP %d)", target, res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered HTTP %d", target, res.StatusCode)
	}
	if json.Unmarshal(data, out) != nil {
		return fmt.Errorf("invalid JSON from %s", target)
	}
	return nil
}

type domain struct {
	Registrar   string   `json:"registrar"`
	Status      []string `json:"status"`
	NameServers []string `json:"nameservers"`
	Expires     string   `json:"expires,omitempty"`
	Signed      bool     `json:"delegation_signed"`
}

func (l *lookup) rdap(ctx context.Context, name string) (*domain, error) {
	var raw struct {
		Status      []string `json:"status"`
		Nameservers []struct {
			Name string `json:"ldhName"`
		} `json:"nameservers"`
		SecureDNS struct {
			DelegationSigned bool `json:"delegationSigned"`
		} `json:"secureDNS"`
		Events []struct {
			Action string `json:"eventAction"`
			Date   string `json:"eventDate"`
		} `json:"events"`
		Entities []struct {
			Roles []string          `json:"roles"`
			VCard []json.RawMessage `json:"vcardArray"`
		} `json:"entities"`
	}
	if err := l.fetch(ctx, "https://rdap.org/domain/"+url.PathEscape(name), "application/rdap+json", &raw); err != nil {
		return nil, err
	}

	d := &domain{Status: raw.Status, Signed: raw.SecureDNS.DelegationSigned}
	for _, ns := range raw.Nameservers {
		d.NameServers = append(d.NameServers, strings.ToLower(ns.Name))
	}
	for _, e := range raw.Events {
		if e.Action == "expiration" {
			d.Expires = e.Date
		}
	}
	for _, e := range raw.Entities {
		if !slices.Contains(e.Roles, "registrar") || len(e.VCard) < 2 {
			continue
		}
		var props [][]json.RawMessage
		if json.Unmarshal(e.VCard[1], &props) != nil {
			continue
		}
		for _, p := range props {
			var key, value string
			if len(p) == 4 && json.Unmarshal(p[0], &key) == nil && key == "fn" && json.Unmarshal(p[3], &value) == nil {
				d.Registrar = value
			}
		}
	}
	if d.Registrar == "" {
		return nil, fmt.Errorf("RDAP for %s names no registrar", name)
	}
	return d, nil
}

func (l *lookup) parentDS(ctx context.Context, name string) ([]string, error) {
	var res struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	q := url.Values{"name": {name}, "type": {"DS"}}
	if err := l.fetch(ctx, "https://cloudflare-dns.com/dns-query?"+q.Encode(), "application/dns-json", &res); err != nil {
		return nil, err
	}
	if res.Status != 0 {
		return nil, fmt.Errorf("DNS-over-HTTPS DS lookup for %s returned rcode %d", name, res.Status)
	}
	var ds []string
	for _, a := range res.Answer {
		if a.Type == 43 {
			ds = append(ds, a.Data)
		}
	}
	return ds, nil
}
