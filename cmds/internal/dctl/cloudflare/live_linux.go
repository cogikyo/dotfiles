package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

const phase = "http_request_firewall_custom"

type zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Account struct {
		ID string `json:"id"`
	} `json:"account"`
	Plan struct {
		LegacyID string `json:"legacy_id"`
	} `json:"plan"`
}

type tunnel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tunnelConfig struct {
	Version int                        `json:"version"`
	Source  string                     `json:"source"`
	Config  map[string]json.RawMessage `json:"config"`
}

type record struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

type rule struct {
	ID          string `json:"id,omitempty"`
	Version     string `json:"version,omitempty"`
	Ref         string `json:"ref"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Action      string `json:"action"`
	Enabled     bool   `json:"enabled"`
}

func (r *rule) UnmarshalJSON(data []byte) error {
	type wire rule
	w := wire{Enabled: true}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*r = rule(w)
	return nil
}

type ruleset struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Rules   []rule `json:"rules"`
}

type live struct {
	Account  string
	Zones    []zone
	Tunnel   tunnel
	Config   tunnelConfig
	Records  map[string][]record
	Rulesets map[string]*ruleset
}

func read(ctx context.Context, c *client, s *sites) (*live, error) {
	zones, err := list[zone](ctx, c, nil, "zones")
	if err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("the token sees no zones; it needs Zone Read on the account's zones")
	}
	slices.SortFunc(zones, func(a, b zone) int { return strings.Compare(a.Name, b.Name) })

	l := &live{Zones: zones, Records: map[string][]record{}, Rulesets: map[string]*ruleset{}}
	names := make([]string, len(zones))
	for i, z := range zones {
		names[i] = z.Name
		if z.Account.ID == "" || l.Account != "" && z.Account.ID != l.Account {
			return nil, fmt.Errorf("zone %s does not share one account ID with the other zones; dctl expects one account", z.Name)
		}
		l.Account = z.Account.ID
	}
	if err := s.resolve(names); err != nil {
		return nil, err
	}

	tunnels, err := list[tunnel](ctx, c, url.Values{"name": {s.Tunnel}, "is_deleted": {"false"}}, "accounts", l.Account, "cfd_tunnel")
	if err != nil {
		return nil, err
	}
	tunnels = slices.DeleteFunc(tunnels, func(t tunnel) bool { return t.Name != s.Tunnel })
	if len(tunnels) != 1 {
		return nil, fmt.Errorf("found %d live tunnels named %q; need exactly one", len(tunnels), s.Tunnel)
	}
	l.Tunnel = tunnels[0]
	if err := c.get(ctx, &l.Config, nil, "accounts", l.Account, "cfd_tunnel", l.Tunnel.ID, "configurations"); err != nil {
		return nil, err
	}

	for _, z := range zones {
		records, err := list[record](ctx, c, nil, "zones", z.ID, "dns_records")
		if err != nil {
			return nil, err
		}
		l.Records[z.Name] = records

		var rs ruleset
		err = c.get(ctx, &rs, nil, "zones", z.ID, "rulesets", "phases", phase, "entrypoint")
		switch {
		case isMissing(err):
		case err != nil:
			return nil, err
		default:
			l.Rulesets[z.Name] = &rs
		}
	}
	return l, nil
}
