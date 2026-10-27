package cloudflare

import (
	"fmt"
	"slices"
	"strings"
)

type phase struct {
	ID     string
	Label  string
	Prefix string
	Noun   string
	Free   int
}

const fallbackRef = "dctl-fallback"

var (
	firewall  = phase{ID: "http_request_firewall_custom", Label: "waf", Prefix: "dctl-allow-", Noun: "custom rules", Free: 5}
	forwards  = phase{ID: "http_request_dynamic_redirect", Label: "redirect", Prefix: "dctl-", Noun: "single redirects", Free: 10}
	configure = phase{ID: "http_config_settings", Label: "config", Prefix: "dctl-stream-", Noun: "configuration rules", Free: 10}
	phases    = []phase{firewall, forwards, configure}
)

type ruleChange struct {
	Action   string `json:"action"`
	Current  *rule  `json:"current,omitempty"`
	Proposed *rule  `json:"proposed,omitempty"`
	Before   string `json:"before,omitempty"`
}

func (ch ruleChange) ref() string {
	if ch.Proposed != nil {
		return ch.Proposed.Ref
	}
	return ch.Current.Ref
}

type rulePlan struct {
	Phase   string       `json:"phase"`
	Zone    string       `json:"zone"`
	Ruleset string       `json:"ruleset"`
	Changes []ruleChange `json:"changes"`
	Kept    []rule       `json:"kept"`
	spec    phase
	zoneID  string
}

func (r rule) same(other rule) bool {
	return r.Ref == other.Ref && r.Description == other.Description && r.Expression == other.Expression && r.Action == other.Action && r.Params == other.Params && r.Enabled == other.Enabled
}

func (r rule) describe() string {
	switch r.Action {
	case "redirect":
		return fmt.Sprintf("%s → %d %s", r.Expression, r.Params.FromValue.StatusCode, r.Params.FromValue.TargetURL.Expression)
	case "set_config":
		return r.Expression + " → response_body_buffering " + r.Params.ResponseBody
	}
	return r.Expression
}

func redirectRules(s *sites, zone string) []rule {
	var rules []rule
	for _, r := range s.Redirects {
		if r.Zone != zone {
			continue
		}
		rules = append(rules, rule{
			Ref:         "dctl-redirect-" + r.From,
			Description: "dctl: redirect " + r.From + " to " + r.To,
			Expression:  fmt.Sprintf("http.host eq %q", r.From),
			Action:      "redirect",
			Params: params{FromValue: forward{
				StatusCode:    301,
				TargetURL:     target{Expression: fmt.Sprintf("concat(%q, http.request.uri.path)", r.To)},
				PreserveQuery: true,
			}},
			Enabled: true,
		})
	}
	if f := fallbackRule(s, zone); f != nil {
		rules = append(rules, *f)
	}
	return rules
}

func fallbackRule(s *sites, zone string) *rule {
	to := s.Zones[zone].Fallback
	if to == "" {
		return nil
	}
	var known []string
	for _, site := range s.Sites {
		if site.Zone == zone {
			known = append(known, site.Host)
		}
	}
	for _, r := range s.Redirects {
		if r.Zone == zone {
			known = append(known, r.From)
		}
	}
	clauses := []string{"true"}
	if len(known) > 0 {
		clauses[0] = "(not http.host in " + quoted(known) + ")"
	}
	for _, g := range groups(s, zone) {
		clauses = append(clauses, "(http.host in "+quoted(g.Hosts)+" and "+s.outside(g.Allow)+")")
	}
	return &rule{
		Ref:         fallbackRef,
		Description: "dctl: send unknown hosts and blocked requests in " + zone + " to " + to,
		Expression:  strings.Join(clauses, " or "),
		Action:      "redirect",
		Params: params{FromValue: forward{
			StatusCode: 302,
			TargetURL:  target{Expression: fmt.Sprintf("concat(%q, \"?from=\", http.host, http.request.uri.path)", to)},
		}},
		Enabled: true,
	}
}

func streamRules(s *sites, zone string) []rule {
	var rules []rule
	for _, site := range s.Sites {
		if site.Zone != zone {
			continue
		}
		for _, r := range site.Routes {
			if !r.Stream {
				continue
			}
			prefix, _ := streamPrefix(r.Path)
			rules = append(rules, rule{
				Ref:         configure.Prefix + site.Host + prefix,
				Description: "dctl: stream " + site.Host + prefix + " without response buffering",
				Expression:  fmt.Sprintf("http.host eq %q and starts_with(http.request.uri.path, %q)", site.Host, prefix),
				Action:      "set_config",
				Params:      params{ResponseBody: "none"},
				Enabled:     true,
			})
		}
	}
	return rules
}

func diffRules(spec phase, z zone, rs *ruleset, want []rule, legacy string) (*rulePlan, error) {
	w := &rulePlan{Phase: spec.ID, Zone: z.Name, spec: spec, zoneID: z.ID}
	var current []rule
	if rs != nil {
		w.Ruleset = rs.ID
		current = rs.Rules
	}

	owned := map[string]rule{}
	removed := 0
	tail := -1
	for i, r := range current {
		mine := strings.HasPrefix(r.Ref, spec.Prefix)
		replaced := legacy != "" && r.Ref == legacy
		if !mine && !replaced {
			w.Kept = append(w.Kept, r)
			continue
		}
		tail = i
		if mine {
			if _, dup := owned[r.Ref]; dup {
				return nil, fmt.Errorf("zone %s has two %s rules with ref %s; remove one in the dashboard", z.Name, spec.Label, r.Ref)
			}
			owned[r.Ref] = r
		}
		if !slices.ContainsFunc(want, func(d rule) bool { return d.Ref == r.Ref }) {
			w.Changes = append(w.Changes, ruleChange{Action: "delete", Current: &r})
			removed++
		}
	}

	// Keep new dctl rules in file order without moving unowned rules.
	after := ""
	if tail >= 0 && tail+1 < len(current) {
		after = current[tail+1].ID
	}
	created := 0
	var changes []ruleChange
	for i, d := range want {
		r, ok := owned[d.Ref]
		switch {
		case !ok:
			ch := ruleChange{Action: "create", Proposed: &d, Before: after}
			for _, next := range want[i+1:] {
				if o, ok := owned[next.Ref]; ok {
					ch.Before = o.ID
					break
				}
			}
			changes = append(changes, ch)
			created++
		case r.same(d):
			changes = append(changes, ruleChange{Action: "unchanged", Current: &r, Proposed: &d})
		default:
			changes = append(changes, ruleChange{Action: "update", Current: &r, Proposed: &d})
		}
	}
	w.Changes = append(changes, w.Changes...)

	if len(w.Changes) == 0 && len(w.Kept) == 0 {
		return nil, nil
	}
	if z.Plan.LegacyID == "free" {
		if peak := len(current) + created; peak > spec.Free {
			return nil, fmt.Errorf("zone %s is on the Free plan, which allows %d %s; adding %d dctl rules before removing %d would need %d at once (%d not owned by dctl)", z.Name, spec.Free, spec.Noun, created, removed, peak, len(w.Kept))
		}
	}
	return w, nil
}
