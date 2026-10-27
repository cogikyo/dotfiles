package cloudflare

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"dotfiles/cmds/internal/ui"
)

const order = "Apply order: WAF, redirect, and config rule additions, tunnel ingress, DNS additions, zone settings and DNSSEC, then DNS removals and config, redirect, and WAF rule removals"

type result struct {
	Action   string   `json:"action"`
	Sites    string   `json:"sites"`
	Outcome  string   `json:"outcome"`
	Plan     *plan    `json:"plan"`
	Applied  []string `json:"applied,omitempty"`
	Evidence []string `json:"evidence"`
	Error    string   `json:"error,omitempty"`
}

func survey(ctx context.Context, c *client, s *sites) (*plan, error) {
	l, err := read(ctx, c, s)
	if err != nil {
		return nil, err
	}
	return diff(s, l)
}

func readOnly(c *client) string {
	return fmt.Sprintf("Read-only: sent %d requests, all GET", c.sent)
}

func widen(current, final string) string {
	if strings.HasPrefix(current, "(") && strings.HasSuffix(current, ") or ("+final+")") {
		return current
	}
	return "(" + current + ") or (" + final + ")"
}

type position struct {
	Before string `json:"before"`
}

type placed struct {
	rule
	Position *position `json:"position,omitempty"`
}

type step struct {
	label string
	run   func(context.Context, *client) error
}

func send(method string, body any, segments ...string) func(context.Context, *client) error {
	return func(ctx context.Context, c *client) error {
		_, err := c.request(ctx, method, nil, body, nil, segments...)
		return err
	}
}

// steps keeps WAF protection in place across DNS publication and removal.
func steps(p *plan) []step {
	var guard, publish, configure, retract, release []step
	for _, w := range p.Rules {
		g, r := ruleSteps(w)
		guard = append(guard, g...)
		release = append(r, release...)
	}

	var tunnel []step
	if t := p.Tunnel; t.Action != "unchanged" {
		body := map[string]any{"config": map[string]any{"ingress": t.Proposed}}
		tunnel = append(tunnel, step{fmt.Sprintf("tunnel %s: PUT %d ingress rules", t.Name, len(t.Proposed)), send(http.MethodPut, body, "accounts", p.Account, "cfd_tunnel", t.ID, "configurations")})
	}

	for _, d := range p.DNS {
		body := map[string]any{"type": d.Type, "name": d.Host, "content": d.Target, "proxied": true, "ttl": 1}
		if d.Comment != "" {
			body["comment"] = d.Comment
		}
		label := "dns " + d.Zone + ": " + d.Action + " " + d.Type + " " + d.Host
		switch d.Action {
		case "create":
			publish = append(publish, step{label, send(http.MethodPost, body, "zones", d.zoneID, "dns_records")})
		case "update":
			publish = append(publish, step{label, send(http.MethodPatch, body, "zones", d.zoneID, "dns_records", d.ID)})
		case "delete":
			retract = append(retract, step{label, send(http.MethodDelete, nil, "zones", d.zoneID, "dns_records", d.ID)})
		}
	}

	for _, st := range p.Settings {
		if st.Action != "update" {
			continue
		}
		label := fmt.Sprintf("settings %s: %s %s → %s", st.Zone, st.ID, st.Current, st.Proposed)
		if st.ID == "dnssec" {
			configure = append(configure, step{label, send(http.MethodPatch, map[string]string{"status": st.Proposed}, "zones", st.zoneID, "dnssec")})
			continue
		}
		configure = append(configure, step{label, send(http.MethodPatch, map[string]string{"value": st.Proposed}, "zones", st.zoneID, "settings", st.ID)})
	}

	return slices.Concat(guard, tunnel, publish, configure, retract, release)
}

func ruleSteps(w rulePlan) (guard, release []step) {
	label := w.spec.Label + " " + w.Zone + ": "
	if w.Ruleset == "" {
		var rules []rule
		for _, ch := range w.Changes {
			if ch.Action == "create" {
				rules = append(rules, *ch.Proposed)
			}
		}
		if len(rules) > 0 {
			body := map[string]any{"rules": rules}
			guard = append(guard, step{label + fmt.Sprintf("create the %s entrypoint with %d rules", w.Phase, len(rules)), send(http.MethodPut, body, "zones", w.zoneID, "rulesets", "phases", w.Phase, "entrypoint")})
		}
		return guard, nil
	}

	rules := []string{"zones", w.zoneID, "rulesets", w.Ruleset, "rules"}
	for _, ch := range w.Changes {
		switch ch.Action {
		case "create":
			body := placed{rule: *ch.Proposed}
			if ch.Before != "" {
				body.Position = &position{Before: ch.Before}
			}
			guard = append(guard, step{label + "create " + ch.ref(), send(http.MethodPost, body, rules...)})
		case "update":
			target := append(slices.Clone(rules), ch.Current.ID)
			final := *ch.Proposed
			if w.spec != firewall || ch.Current.Expression == final.Expression {
				guard = append(guard, step{label + "update " + ch.ref(), send(http.MethodPatch, final, target...)})
				continue
			}
			// Block under either policy until routing and DNS reach the new state.
			interim := final
			interim.Expression = widen(ch.Current.Expression, final.Expression)
			if !interim.same(*ch.Current) {
				guard = append(guard, step{label + "widen " + ch.ref() + " to block under the old or new expression", send(http.MethodPatch, interim, target...)})
			}
			release = append(release, step{label + "narrow " + ch.ref() + " to the new expression", send(http.MethodPatch, final, target...)})
		case "delete":
			release = append(release, step{label + "delete " + ch.ref(), send(http.MethodDelete, nil, append(slices.Clone(rules), ch.Current.ID)...)})
		}
	}
	return guard, release
}

func apply(ctx context.Context, u *ui.UI, c *client, s *sites, r result) error {
	p := r.Plan
	if !u.Yes() && !u.Can() {
		return report(u, r, fmt.Errorf("apply requires --yes in JSON or non-TTY mode; run dctl cloudflare plan to preview without consent"))
	}
	ok, err := u.Confirm(fmt.Sprintf("Apply these %d changes to Cloudflare, exactly as previewed?", p.pending()))
	if err != nil {
		return report(u, r, err)
	}
	if !ok {
		r.Outcome = "cancelled"
		return report(u, r, nil)
	}

	again, err := survey(ctx, c, s)
	if err != nil {
		return report(u, r, err)
	}
	if !reflect.DeepEqual(p, again) {
		r.Plan = again
		return report(u, r, fmt.Errorf("Cloudflare state changed after preview; nothing sent; run plan and review again"))
	}
	r.Evidence = append(r.Evidence, "Recheck after consent: live state still matches the preview")

	c.write = true
	for _, st := range steps(p) {
		if err := st.run(ctx, c); err != nil {
			c.write = false
			return uncertain(u, r, fmt.Errorf("%s: %w", st.label, err))
		}
		r.Applied = append(r.Applied, st.label)
	}
	c.write = false

	after, err := survey(ctx, c, s)
	if err != nil {
		return uncertain(u, r, fmt.Errorf("all %d steps were accepted, but readback failed: %w", len(r.Applied), err))
	}
	if n := after.pending(); n > 0 {
		r.Plan = after
		return uncertain(u, r, fmt.Errorf("all %d steps were accepted, but readback still shows %d differences", len(r.Applied), n))
	}
	for _, w := range p.Rules {
		i := slices.IndexFunc(after.Rules, func(a rulePlan) bool { return a.Phase == w.Phase && a.Zone == w.Zone })
		var kept []rule
		if i >= 0 {
			kept = after.Rules[i].Kept
		}
		if !slices.Equal(kept, w.Kept) {
			return uncertain(u, r, fmt.Errorf("%s rules dctl does not own in zone %s differ from before apply; inspect them in the dashboard", w.Phase, w.Zone))
		}
	}

	r.Plan = after
	r.Outcome = "verified"
	r.Evidence = append(r.Evidence, fmt.Sprintf("%d steps sent; readback shows no remaining difference and every rule dctl does not own unchanged and in order", len(r.Applied)))
	if p.Tunnel.Action != "unchanged" {
		r.Evidence = append(r.Evidence, fmt.Sprintf("Ingress changed: on edge, check journalctl -u cloudflared-tunnel-%s for \"Updated to new configuration\"; if it is absent, restart that unit there (dctl never restarts edge)", p.Tunnel.ID))
	}
	r.Evidence = append(r.Evidence, registrarDS(ctx, p, after)...)
	return report(u, r, nil)
}

func registrarDS(ctx context.Context, before, after *plan) []string {
	var notes []string
	pub := newLookup()
	for _, st := range before.Settings {
		if st.ID != "dnssec" || st.Action != "update" {
			continue
		}
		i := slices.IndexFunc(after.Settings, func(a settingChange) bool { return a.ID == "dnssec" && a.Zone == st.Zone })
		if i < 0 || after.Settings[i].DS == nil {
			notes = append(notes, fmt.Sprintf("DNSSEC on %s: Cloudflare returned no DS record yet; run dctl cloudflare status %s later", st.Zone, st.Zone))
			continue
		}
		d := after.Settings[i].DS
		info, err := pub.rdap(ctx, st.Zone)
		switch {
		case err != nil:
			notes = append(notes, fmt.Sprintf("DNSSEC on %s: registrar unknown (%v); if it is not Cloudflare Registrar, paste this DS there: %s", st.Zone, err, d.DS))
		case cloudflareRegistrar(info.Registrar):
			notes = append(notes, fmt.Sprintf("DNSSEC on %s: registrar %s publishes the DS itself, usually within 1–2 days", st.Zone, info.Registrar))
		default:
			notes = append(notes, fmt.Sprintf("DNSSEC on %s: registrar %s needs this DS pasted there: key tag %d, algorithm %s, digest type %s, digest %s (%s)", st.Zone, info.Registrar, d.KeyTag, d.Algorithm, d.DigestType, d.Digest, d.DS))
		}
	}
	return notes
}

func uncertain(u *ui.UI, r result, cause error) error {
	r.Outcome = "uncertain"
	return report(u, r, fmt.Errorf("uncertain apply outcome: %w; %d earlier steps were sent and the failed one may have taken effect; run dctl cloudflare plan before any retry; no retry or rollback attempted", cause, len(r.Applied)))
}

func access(s site) string {
	switch {
	case s.Allow != nil:
		return s.Access
	case s.Host == s.Zone:
		return s.Access + " (apex default)"
	default:
		return s.Access + " (zone default)"
	}
}

func preview(u *ui.UI, p *plan) {
	u.Section("sites", fmt.Sprintf("%d hosts", len(p.Sites)))
	for _, s := range p.Sites {
		u.Info("%s → %s", s.Host, access(s))
	}

	t := p.Tunnel
	u.Section("tunnel "+t.Name, fmt.Sprintf("%s · source %s · version %d", t.ID, t.Source, t.Version))
	if t.Action == "unchanged" {
		u.OK("ingress matches the file (%d rules)", len(t.Proposed))
	} else {
		u.Row(ui.Todo, fmt.Sprintf("PUT the whole ingress (%d rules)", len(t.Proposed)))
		if t.Source != "cloudflare" {
			u.Detail("config source is %s; this PUT converts the tunnel to Cloudflare-managed config", t.Source)
		}
		if t.Unreadable != "" {
			u.Detail("%s", t.Unreadable)
		}
		if len(t.Dropped) > 0 {
			u.Detail("drops other config keys: %s", strings.Join(t.Dropped, ", "))
		}
		for _, r := range t.Current {
			u.Detail("- %s", r.describe())
		}
		for _, r := range t.Proposed {
			u.Detail("+ %s", r.describe())
		}
	}

	zone := ""
	for _, d := range p.DNS {
		if d.Zone != zone {
			zone = d.Zone
			u.Section("dns "+zone, "proxied records")
		}
		line := d.Type + " " + d.Host + " → " + d.Target
		if d.Comment != "" {
			line += " (" + d.Comment + ")"
		}
		switch d.Action {
		case "unchanged":
			u.OK("%s", line)
		case "create", "update":
			u.Row(ui.Todo, d.Action+" "+line)
		case "delete":
			u.Row(ui.Todo, "delete "+line+" (owned by dctl but not in the file)")
		}
	}

	for _, w := range p.Rules {
		note := "ruleset " + w.Ruleset
		if w.Ruleset == "" {
			note = "no entrypoint yet; apply creates it"
		}
		u.Section(w.spec.Label+" "+w.Zone, note)
		for _, ch := range w.Changes {
			switch ch.Action {
			case "unchanged":
				u.OK("%s", ch.ref())
			case "create":
				u.Row(ui.Todo, "create "+ch.ref())
				u.Detail("+ %s", ch.Proposed.describe())
			case "update":
				u.Row(ui.Todo, "update "+ch.ref())
				u.Detail("- %s", ch.Current.describe())
				u.Detail("+ %s", ch.Proposed.describe())
			case "delete":
				u.Row(ui.Todo, "delete "+ch.ref())
				u.Detail("- %s", ch.Current.describe())
			}
			switch {
			case w.spec != forwards || ch.Action == "delete":
			case ch.ref() == fallbackRef:
				u.Detail("runs before WAF custom rules, which stay in place as the backstop")
			default:
				u.Detail("redirects run before WAF custom rules, so anyone gets this 301")
			}
		}
		for _, k := range w.Kept {
			u.Info("keep %s (%s, not owned by dctl)", k.Ref, k.Action)
		}
	}

	previewSettings(u, p.Settings)
}

func report(u *ui.UI, r result, err error) error {
	if err != nil {
		if r.Outcome != "uncertain" {
			r.Outcome = "failed"
		}
		r.Error = err.Error()
	}
	if r.Evidence == nil {
		r.Evidence = []string{}
	}

	if u.JSON() {
		return errors.Join(err, u.Emit(r))
	}

	if len(r.Applied) > 0 {
		u.Section("applied", fmt.Sprintf("%d steps", len(r.Applied)))
		for _, label := range r.Applied {
			u.Info("%s", label)
		}
	}
	if len(r.Evidence) > 0 {
		u.Section("evidence", "")
	}
	for _, evidence := range r.Evidence {
		u.Info("%s", evidence)
	}
	switch {
	case err != nil:
	case r.Outcome == "cancelled":
		u.Close(ui.Warn, "%s: %s", r.Action, r.Outcome)
	default:
		u.Close(ui.OK, "%s: %s", r.Action, r.Outcome)
	}
	return err
}
