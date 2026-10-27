package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

type zone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	NameServers []string `json:"name_servers"`
	Account     struct {
		ID string `json:"id"`
	} `json:"account"`
	Plan struct {
		Name     string `json:"name"`
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
	Comment string `json:"comment"`
}

type target struct {
	Expression string `json:"expression,omitempty"`
}

type forward struct {
	StatusCode    int    `json:"status_code,omitempty"`
	TargetURL     target `json:"target_url,omitzero"`
	PreserveQuery bool   `json:"preserve_query_string,omitempty"`
}

type params struct {
	FromValue    forward `json:"from_value,omitzero"`
	ResponseBody string  `json:"response_body_buffering,omitempty"`
}

type rule struct {
	ID          string `json:"id,omitempty"`
	Version     string `json:"version,omitempty"`
	Ref         string `json:"ref"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Action      string `json:"action"`
	Params      params `json:"action_parameters,omitzero"`
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

type dnssec struct {
	Status     string `json:"status"`
	Flags      int    `json:"flags"`
	Algorithm  string `json:"algorithm"`
	KeyTag     int    `json:"key_tag"`
	DigestType string `json:"digest_type"`
	Digest     string `json:"digest"`
	DS         string `json:"ds"`
	PublicKey  string `json:"public_key"`
}

type setting struct {
	Value  string `json:"value"`
	Denied string `json:"-"`
}

type live struct {
	Account  string
	Zones    []zone
	Tunnel   tunnel
	Config   tunnelConfig
	Records  map[string][]record
	Rulesets map[string]map[string]*ruleset
	Settings map[string]map[string]setting
	DNSSEC   map[string]dnssec
}

func listZones(ctx context.Context, c *client) ([]zone, error) {
	zones, err := list[zone](ctx, c, nil, "zones")
	if err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("the token sees no zones; it needs Zone Read on the account's zones")
	}
	slices.SortFunc(zones, func(a, b zone) int { return strings.Compare(a.Name, b.Name) })
	return zones, nil
}

func entrypoint(ctx context.Context, c *client, zoneID, phase string) (*ruleset, error) {
	var rs ruleset
	err := c.get(ctx, &rs, nil, "zones", zoneID, "rulesets", "phases", phase, "entrypoint")
	switch {
	case isMissing(err):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &rs, nil
}

func readSetting(ctx context.Context, c *client, zoneID, id string) (setting, error) {
	var s setting
	err := c.get(ctx, &s, nil, "zones", zoneID, "settings", id)
	var refused *apiError
	if id == automatic && errors.As(err, &refused) && (refused.Status == http.StatusForbidden || refused.Status == http.StatusUnauthorized) {
		return setting{Denied: err.Error()}, nil
	}
	return s, err
}

func read(ctx context.Context, c *client, s *sites) (*live, error) {
	zones, err := listZones(ctx, c)
	if err != nil {
		return nil, err
	}

	l := &live{
		Zones:    zones,
		Records:  map[string][]record{},
		Rulesets: map[string]map[string]*ruleset{},
		Settings: map[string]map[string]setting{},
		DNSSEC:   map[string]dnssec{},
	}
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

		l.Rulesets[z.Name] = map[string]*ruleset{}
		for _, ph := range phases {
			rs, err := entrypoint(ctx, c, z.ID, ph.ID)
			if err != nil {
				return nil, err
			}
			l.Rulesets[z.Name][ph.ID] = rs
		}

		l.Settings[z.Name] = map[string]setting{}
		for _, want := range s.settings() {
			v, err := readSetting(ctx, c, z.ID, want.ID)
			if err != nil {
				return nil, err
			}
			l.Settings[z.Name][want.ID] = v
		}

		if s.DNSSEC != nil {
			var d dnssec
			if err := c.get(ctx, &d, nil, "zones", z.ID, "dnssec"); err != nil {
				return nil, err
			}
			l.DNSSEC[z.Name] = d
		}
	}
	return l, nil
}
