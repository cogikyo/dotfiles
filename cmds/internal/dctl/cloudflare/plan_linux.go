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
	legacyRef   = "vagarios-allowlist"
	catchAll    = "http_status:404"
	tunnelHost  = ".cfargotunnel.com"
	placeholder = "192.0.2.1"
	marker      = "dctl: redirect placeholder"
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
	Action  string `json:"action"`
	Zone    string `json:"zone"`
	Host    string `json:"host"`
	Type    string `json:"type"`
	Target  string `json:"target"`
	Comment string `json:"comment,omitempty"`
	ID      string `json:"id,omitempty"`
	zoneID  string
}

func (r record) placeholder() bool {
	return r.Type == "A" && r.Comment == marker
}

// address excludes records such as MX and TXT, which may share the name.
func (r record) address() bool {
	return r.Type == "A" || r.Type == "AAAA" || r.Type == "CNAME"
}

type plan struct {
	Account   string          `json:"account"`
	Sites     []site          `json:"sites"`
	Redirects []redirect      `json:"redirects,omitempty"`
	Tunnel    tunnelPlan      `json:"tunnel"`
	DNS       []dnsChange     `json:"dns"`
	Rules     []rulePlan      `json:"rules"`
	Settings  []settingChange `json:"settings,omitempty"`
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
	for _, w := range p.Rules {
		for _, c := range w.Changes {
			if c.Action != "unchanged" {
				n++
			}
		}
	}
	for _, st := range p.Settings {
		if st.Action == "update" {
			n++
		}
	}
	return n
}

func diff(s *sites, l *live) (*plan, error) {
	p := &plan{Account: l.Account, Sites: s.Sites, Redirects: s.Redirects}

	p.Tunnel = diffTunnel(s, l)

	var err error
	if p.DNS, err = diffDNS(s, l); err != nil {
		return nil, err
	}

	for _, ph := range phases {
		for _, z := range l.Zones {
			var want []rule
			legacy := ""
			switch ph {
			case firewall:
				want = allowRules(s, z.Name)
				if _, managed := s.Zones[z.Name]; managed {
					legacy = legacyRef
				}
			case forwards:
				want = redirectRules(s, z.Name)
			case configure:
				want = streamRules(s, z.Name)
			}
			w, err := diffRules(ph, z, l.Rulesets[z.Name][ph.ID], want, legacy)
			if err != nil {
				return nil, err
			}
			if w != nil {
				p.Rules = append(p.Rules, *w)
			}
		}
	}

	p.Settings = diffSettings(s, l)
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
		change := dnsChange{Action: "create", Zone: site.Zone, Host: site.Host, Type: "CNAME", Target: target, zoneID: ids[site.Zone]}
		var found []string
		for _, r := range l.Records[site.Zone] {
			if r.Name != site.Host || !r.address() {
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

	type spot struct{ zone, host, owner string }
	var spots []spot
	for _, rd := range s.Redirects {
		spots = append(spots, spot{rd.Zone, rd.From, "the redirect"})
	}
	for _, name := range slices.Sorted(maps.Keys(s.Zones)) {
		if s.Zones[name].Fallback != "" {
			spots = append(spots, spot{name, "*." + name, "the fallback"})
		}
	}
	held := map[string]bool{}
	for _, sp := range spots {
		held[sp.host] = true
		change := dnsChange{Action: "create", Zone: sp.zone, Host: sp.host, Type: "A", Target: placeholder, Comment: marker, zoneID: ids[sp.zone]}
		var found []string
		for _, r := range l.Records[sp.zone] {
			if r.Name != sp.host || !r.address() {
				continue
			}
			if r.placeholder() && change.ID == "" {
				change.Action, change.ID = "unchanged", r.ID
				if r.Content != placeholder || !r.Proxied {
					change.Action = "update"
				}
				continue
			}
			found = append(found, fmt.Sprintf("%s %s (proxied=%t)", r.Type, r.Content, r.Proxied))
		}
		if len(found) > 0 {
			return nil, fmt.Errorf("%s already has %s; dctl overwrites nothing, so remove that record or drop %s", sp.host, strings.Join(found, ", "), sp.owner)
		}
		changes = append(changes, change)
	}

	for _, z := range l.Zones {
		var strays []string
		for _, r := range l.Records[z.Name] {
			switch {
			case r.Type == "CNAME" && strings.EqualFold(r.Content, target) && !declared[r.Name]:
				changes = append(changes, dnsChange{Action: "delete", Zone: z.Name, Host: r.Name, Type: r.Type, Target: r.Content, ID: r.ID, zoneID: z.ID})
			case r.placeholder() && !held[r.Name]:
				changes = append(changes, dnsChange{Action: "delete", Zone: z.Name, Host: r.Name, Type: r.Type, Target: r.Content, Comment: r.Comment, ID: r.ID, zoneID: z.ID})
			case r.Proxied && s.Zones[z.Name].Fallback != "" && !declared[r.Name] && !held[r.Name]:
				strays = append(strays, fmt.Sprintf("%s %s", r.Type, r.Name))
			}
		}
		if len(strays) > 0 {
			return nil, fmt.Errorf("zone %s has a fallback, which redirects every undeclared proxied host, but has proxied records dctl does not own: %s; declare each as a [[site]] or [[redirect]], or make it DNS-only", z.Name, strings.Join(strays, ", "))
		}
	}
	slices.SortStableFunc(changes, func(a, b dnsChange) int { return strings.Compare(a.Zone, b.Zone) })
	return changes, nil
}

type group struct {
	Allow string
	Hosts []string
}

func groups(s *sites, zone string) []group {
	var out []group
	for _, site := range s.Sites {
		if site.Zone != zone || site.Access == public {
			continue
		}
		i := slices.IndexFunc(out, func(g group) bool { return g.Allow == site.Access })
		if i < 0 {
			out = append(out, group{Allow: site.Access})
			i = len(out) - 1
		}
		out[i].Hosts = append(out[i].Hosts, site.Host)
	}
	return out
}

func (s *sites) outside(allow string) string {
	return "not ip.src in {" + strings.Join(s.Allow[allow], " ") + "}"
}

func allowRules(s *sites, zone string) []rule {
	policy, managed := s.Zones[zone]
	if !managed {
		return nil
	}
	def := policy.Allow

	var others []string
	for _, site := range s.Sites {
		if site.Zone == zone && site.Access != def {
			others = append(others, site.Host)
		}
	}

	var rules []rule
	if def != public {
		var clauses []string
		if len(others) > 0 {
			clauses = append(clauses, "not http.host in "+quoted(others))
		}
		clauses = append(clauses, s.outside(def))
		rules = append(rules, rule{
			Ref:         firewall.Prefix + def,
			Description: fmt.Sprintf("dctl: %s subdomains default to allowlist %s", zone, def),
			Expression:  strings.Join(clauses, " and "),
			Action:      "block",
			Enabled:     true,
		})
	}
	for _, g := range groups(s, zone) {
		if g.Allow == def {
			continue
		}
		rules = append(rules, rule{
			Ref:         firewall.Prefix + g.Allow,
			Description: "dctl: allowlist " + g.Allow + " for listed hosts",
			Expression:  "http.host in " + quoted(g.Hosts) + " and " + s.outside(g.Allow),
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
