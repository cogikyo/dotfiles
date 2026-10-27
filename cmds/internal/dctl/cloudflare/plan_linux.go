package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	refPrefix  = "dctl-allow-"
	legacyRef  = "vagarios-allowlist"
	freeRules  = 5
	catchAll   = "http_status:404"
	tunnelHost = ".cfargotunnel.com"
)

// ingress keeps only written fields so provider defaults do not cause drift.
type ingress struct {
	Hostname      string        `json:"hostname,omitempty"`
	Path          string        `json:"path,omitempty"`
	Service       string        `json:"service"`
	OriginRequest originRequest `json:"originRequest,omitzero"`
}

type originRequest struct {
	NoTLSVerify bool `json:"noTLSVerify,omitempty"`
}

func (r ingress) describe() string {
	host := r.Hostname
	if host == "" {
		host = "*"
	}
	if r.Path != "" {
		host += " " + r.Path
	}
	line := host + " → " + r.Service
	if r.OriginRequest.NoTLSVerify {
		line += " (noTLSVerify)"
	}
	return line
}

type tunnelPlan struct {
	Name       string    `json:"name"`
	ID         string    `json:"id"`
	Version    int       `json:"version"`
	Source     string    `json:"source"`
	Action     string    `json:"action"`
	Current    []ingress `json:"current"`
	Unreadable string    `json:"unreadable,omitempty"`
	Proposed   []ingress `json:"proposed"`
	Dropped    []string  `json:"dropped,omitempty"`
}

type dnsChange struct {
	Action string `json:"action"`
	Zone   string `json:"zone"`
	Host   string `json:"host"`
	Target string `json:"target"`
	ID     string `json:"id,omitempty"`
	zoneID string
}

type wafChange struct {
	Action   string `json:"action"`
	Current  *rule  `json:"current,omitempty"`
	Proposed *rule  `json:"proposed,omitempty"`
}

func (w wafChange) ref() string {
	if w.Proposed != nil {
		return w.Proposed.Ref
	}
	return w.Current.Ref
}

type wafPlan struct {
	Zone    string      `json:"zone"`
	Ruleset string      `json:"ruleset"`
	Anchor  string      `json:"anchor,omitempty"`
	Changes []wafChange `json:"changes"`
	Kept    []rule      `json:"kept"`
	zoneID  string
}

type plan struct {
	Account string      `json:"account"`
	Sites   []site      `json:"sites"`
	Tunnel  tunnelPlan  `json:"tunnel"`
	DNS     []dnsChange `json:"dns"`
	WAF     []wafPlan   `json:"waf"`
}

func (p *plan) pending() int {
	n := 0
	if p.Tunnel.Action != "unchanged" {
		n++
	}
	for _, d := range p.DNS {
		if d.Action != "unchanged" {
			n++
		}
	}
	for _, w := range p.WAF {
		for _, c := range w.Changes {
			if c.Action != "unchanged" {
				n++
			}
		}
	}
	return n
}

func diff(s *sites, l *live) (*plan, error) {
	p := &plan{Account: l.Account, Sites: s.Sites}

	p.Tunnel = diffTunnel(s, l)

	var err error
	if p.DNS, err = diffDNS(s, l); err != nil {
		return nil, err
	}

	for _, z := range l.Zones {
		w, err := diffWAF(s, l, z)
		if err != nil {
			return nil, err
		}
		if w != nil {
			p.WAF = append(p.WAF, *w)
		}
	}
	return p, nil
}

func desiredIngress(s *sites) []ingress {
	var rules []ingress
	for _, site := range s.Sites {
		for _, r := range site.Routes {
			rules = append(rules, ingress{Hostname: site.Host, Path: r.Path, Service: r.Origin.service(), OriginRequest: originRequest{NoTLSVerify: r.Origin.NoTLSVerify}})
		}
		rules = append(rules, ingress{Hostname: site.Host, Service: site.Origin.service(), OriginRequest: originRequest{NoTLSVerify: site.Origin.NoTLSVerify}})
	}
	return append(rules, ingress{Service: catchAll})
}

func diffTunnel(s *sites, l *live) tunnelPlan {
	tp := tunnelPlan{Name: l.Tunnel.Name, ID: l.Tunnel.ID, Version: l.Config.Version, Source: l.Config.Source, Proposed: desiredIngress(s)}
	if raw, ok := l.Config.Config["ingress"]; ok {
		if err := json.Unmarshal(raw, &tp.Current); err != nil {
			tp.Current = nil
			tp.Unreadable = fmt.Sprintf("cannot read live ingress (%v): %s", err, compact(raw))
		}
	}

	tp.Action = "unchanged"
	if tp.Source != "cloudflare" || tp.Unreadable != "" || !slices.Equal(tp.Current, tp.Proposed) {
		tp.Action = "update"
		for _, key := range slices.Sorted(maps.Keys(l.Config.Config)) {
			if key != "ingress" {
				tp.Dropped = append(tp.Dropped, key)
			}
		}
	}
	return tp
}

func compact(raw json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return string(raw)
	}
	return b.String()
}

func diffDNS(s *sites, l *live) ([]dnsChange, error) {
	target := l.Tunnel.ID + tunnelHost
	ids := map[string]string{}
	for _, z := range l.Zones {
		ids[z.Name] = z.ID
	}

	var changes []dnsChange
	declared := map[string]bool{}
	for _, site := range s.Sites {
		declared[site.Host] = true
		change := dnsChange{Action: "create", Zone: site.Zone, Host: site.Host, Target: target, zoneID: ids[site.Zone]}
		var found []string
		for _, r := range l.Records[site.Zone] {
			if r.Name != site.Host {
				continue
			}
			if r.Type == "CNAME" && strings.EqualFold(r.Content, target) && r.Proxied {
				change.Action, change.ID = "unchanged", r.ID
				continue
			}
			found = append(found, fmt.Sprintf("%s %s (proxied=%t)", r.Type, r.Content, r.Proxied))
		}
		if len(found) > 0 {
			return nil, fmt.Errorf("%s already has %s; dctl overwrites nothing, so remove that record or drop the site", site.Host, strings.Join(found, ", "))
		}
		changes = append(changes, change)
	}

	for _, z := range l.Zones {
		for _, r := range l.Records[z.Name] {
			if r.Type == "CNAME" && strings.EqualFold(r.Content, target) && !declared[r.Name] {
				changes = append(changes, dnsChange{Action: "delete", Zone: z.Name, Host: r.Name, Target: r.Content, ID: r.ID, zoneID: z.ID})
			}
		}
	}
	return changes, nil
}

func desiredRules(s *sites, zone string) []rule {
	policy, managed := s.Zones[zone]
	if !managed {
		return nil
	}
	def := policy.Allow

	var order []string
	hosts := map[string][]string{}
	apex := def
	var others []string
	for _, site := range s.Sites {
		if site.Zone != zone {
			continue
		}
		if site.Host == zone {
			apex = site.Access
		}
		if site.Access != def && site.Host != zone {
			others = append(others, site.Host)
		}
		if site.Access == def || site.Access == public {
			continue
		}
		if _, ok := hosts[site.Access]; !ok {
			order = append(order, site.Access)
		}
		hosts[site.Access] = append(hosts[site.Access], site.Host)
	}

	var rules []rule
	if def != public {
		var clauses []string
		if apex != def {
			clauses = append(clauses, fmt.Sprintf("http.host ne %q", zone))
		}
		if len(others) > 0 {
			clauses = append(clauses, "not http.host in "+quoted(others))
		}
		clauses = append(clauses, "not ip.src in {"+strings.Join(s.Allow[def], " ")+"}")
		rules = append(rules, rule{
			Ref:         refPrefix + def,
			Description: fmt.Sprintf("dctl: %s subdomains default to allowlist %s", zone, def),
			Expression:  strings.Join(clauses, " and "),
			Action:      "block",
			Enabled:     true,
		})
	}
	for _, name := range order {
		rules = append(rules, rule{
			Ref:         refPrefix + name,
			Description: "dctl: allowlist " + name + " for listed hosts",
			Expression:  "http.host in " + quoted(hosts[name]) + " and not ip.src in {" + strings.Join(s.Allow[name], " ") + "}",
			Action:      "block",
			Enabled:     true,
		})
	}
	return rules
}

func quoted(hosts []string) string {
	items := make([]string, len(hosts))
	for i, h := range hosts {
		items[i] = fmt.Sprintf("%q", h)
	}
	return "{" + strings.Join(items, " ") + "}"
}

func (r rule) same(other rule) bool {
	return r.Ref == other.Ref && r.Description == other.Description && r.Expression == other.Expression && r.Action == other.Action && r.Enabled == other.Enabled
}

func diffWAF(s *sites, l *live, z zone) (*wafPlan, error) {
	_, managed := s.Zones[z.Name]
	want := desiredRules(s, z.Name)
	rs := l.Rulesets[z.Name]

	w := &wafPlan{Zone: z.Name, zoneID: z.ID}
	var current []rule
	if rs != nil {
		w.Ruleset = rs.ID
		current = rs.Rules
	}

	owned := map[string]rule{}
	removed := 0
	for _, r := range current {
		mine := strings.HasPrefix(r.Ref, refPrefix)
		replaced := managed && r.Ref == legacyRef
		if !mine && !replaced {
			w.Kept = append(w.Kept, r)
			continue
		}
		if w.Anchor == "" {
			w.Anchor = r.ID
		}
		if mine {
			if _, dup := owned[r.Ref]; dup {
				return nil, fmt.Errorf("zone %s has two rules with ref %s; remove one in the dashboard", z.Name, r.Ref)
			}
			owned[r.Ref] = r
		}
		if !slices.ContainsFunc(want, func(d rule) bool { return d.Ref == r.Ref }) {
			w.Changes = append(w.Changes, wafChange{Action: "delete", Current: &r})
			removed++
		}
	}

	created := 0
	var changes []wafChange
	for _, d := range want {
		r, ok := owned[d.Ref]
		switch {
		case !ok:
			changes = append(changes, wafChange{Action: "create", Proposed: &d})
			created++
		case r.same(d):
			changes = append(changes, wafChange{Action: "unchanged", Current: &r, Proposed: &d})
		default:
			changes = append(changes, wafChange{Action: "update", Current: &r, Proposed: &d})
		}
	}
	w.Changes = append(changes, w.Changes...)

	if !managed && len(w.Changes) == 0 {
		return nil, nil
	}
	if z.Plan.LegacyID == "free" {
		peak := len(current) + created
		if peak > freeRules {
			return nil, fmt.Errorf("zone %s is on the Free plan, which allows %d custom rules; adding %d dctl rules before removing %d would need %d at once (%d not owned by dctl)", z.Name, freeRules, created, removed, peak, len(w.Kept))
		}
	}
	return w, nil
}
