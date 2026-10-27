package cloudflare

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

type StatusCmd struct {
	Zone string `arg:"" optional:"" help:"Zone to show (default: every zone the token sees)."`
}

type edgeCert struct {
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Authority string   `json:"certificate_authority"`
	Hosts     []string `json:"hosts"`
	Expires   string   `json:"expires_on"`
}

type entry struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Proxied bool   `json:"proxied"`
}

type phaseCount struct {
	Phase string `json:"phase"`
	Label string `json:"label"`
	Rules *int   `json:"rules"`
}

type zoneStatus struct {
	Zone        string            `json:"zone"`
	Status      string            `json:"status"`
	Plan        string            `json:"plan"`
	NameServers []string          `json:"name_servers"`
	Registry    *domain           `json:"registry,omitempty"`
	RegistryErr string            `json:"registry_error,omitempty"`
	DNSSEC      dnssec            `json:"dnssec"`
	ParentDS    []string          `json:"parent_ds"`
	ParentErr   string            `json:"parent_ds_error,omitempty"`
	Settings    map[string]string `json:"settings"`
	Certs       []edgeCert        `json:"edge_certificates"`
	Records     []entry           `json:"records"`
	Phases      []phaseCount      `json:"phases"`
}

type statusResult struct {
	Action   string       `json:"action"`
	Zones    []zoneStatus `json:"zones"`
	Evidence []string     `json:"evidence"`
	Error    string       `json:"error,omitempty"`
}

func (cmd *StatusCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	c, err := newClient(root.Home)
	if err != nil {
		return err
	}
	pub := newLookup()
	r := statusResult{Action: "status", Zones: []zoneStatus{}}
	err = gather(ctx, c, pub, cmd.Zone, &r)
	r.Evidence = append(r.Evidence, fmt.Sprintf("Read-only: sent %d Cloudflare requests, all GET, and %d public lookups (RDAP, DNS-over-HTTPS)", c.sent, pub.sent))
	if err != nil {
		r.Error = err.Error()
	}
	if u.JSON() {
		return errors.Join(err, u.Emit(r))
	}
	for _, z := range r.Zones {
		showZone(u, z)
	}
	u.Section("evidence", "")
	for _, e := range r.Evidence {
		u.Info("%s", e)
	}
	if err != nil {
		return err
	}
	u.Close(ui.OK, "status: %d zones", len(r.Zones))
	return nil
}

func gather(ctx context.Context, c *client, pub *lookup, only string, r *statusResult) error {
	zones, err := listZones(ctx, c)
	if err != nil {
		return err
	}
	if only != "" {
		names := make([]string, len(zones))
		for i, z := range zones {
			names[i] = z.Name
		}
		zones = slices.DeleteFunc(zones, func(z zone) bool { return z.Name != only })
		if len(zones) == 0 {
			return fmt.Errorf("zone %q is not one this token can see (%s)", only, strings.Join(names, ", "))
		}
	}

	for _, z := range zones {
		st := zoneStatus{Zone: z.Name, Status: z.Status, Plan: z.Plan.Name, NameServers: z.NameServers, Settings: map[string]string{}}

		if st.Registry, err = pub.rdap(ctx, z.Name); err != nil {
			st.RegistryErr = err.Error()
		}
		if err := c.get(ctx, &st.DNSSEC, nil, "zones", z.ID, "dnssec"); err != nil {
			return err
		}
		if st.ParentDS, err = pub.parentDS(ctx, z.Name); err != nil {
			st.ParentErr = err.Error()
		}

		for _, id := range slices.Concat(tlsSettings, visibleSettings) {
			v, err := readSetting(ctx, c, z.ID, id)
			if err != nil {
				return err
			}
			st.Settings[id] = v.Value
			if v.Denied != "" {
				st.Settings[id] = "unreadable with this token"
			}
		}

		var packs []struct {
			Type         string   `json:"type"`
			Status       string   `json:"status"`
			Authority    string   `json:"certificate_authority"`
			Hosts        []string `json:"hosts"`
			Certificates []struct {
				Expires string `json:"expires_on"`
			} `json:"certificates"`
		}
		if err := c.get(ctx, &packs, url.Values{"status": {"all"}}, "zones", z.ID, "ssl", "certificate_packs"); err != nil {
			return err
		}
		for _, p := range packs {
			cert := edgeCert{Type: p.Type, Status: p.Status, Authority: p.Authority, Hosts: p.Hosts}
			for _, pc := range p.Certificates {
				if cert.Expires == "" || pc.Expires < cert.Expires {
					cert.Expires = pc.Expires
				}
			}
			st.Certs = append(st.Certs, cert)
		}

		records, err := list[record](ctx, c, nil, "zones", z.ID, "dns_records")
		if err != nil {
			return err
		}
		for _, rec := range records {
			st.Records = append(st.Records, entry{Type: rec.Type, Name: rec.Name, Proxied: rec.Proxied})
		}
		slices.SortFunc(st.Records, func(a, b entry) int {
			return cmp.Or(strings.Compare(a.Type, b.Type), strings.Compare(a.Name, b.Name))
		})

		for _, ph := range phases {
			rs, err := entrypoint(ctx, c, z.ID, ph.ID)
			if err != nil {
				return err
			}
			count := phaseCount{Phase: ph.ID, Label: ph.Label}
			if rs != nil {
				n := len(rs.Rules)
				count.Rules = &n
			}
			st.Phases = append(st.Phases, count)
		}
		r.Zones = append(r.Zones, st)
	}
	return nil
}

func showZone(u *ui.UI, z zoneStatus) {
	u.Section("zone "+z.Zone, z.Status+" · "+z.Plan)
	u.Info("nameservers %s", strings.Join(z.NameServers, ", "))

	switch reg := z.Registry; {
	case reg == nil:
		u.Warn("registrar unknown: %s", z.RegistryErr)
	default:
		u.Info("registrar %s · %s · expires %s", reg.Registrar, strings.Join(reg.Status, ", "), day(reg.Expires))
		if !slices.Equal(slices.Sorted(slices.Values(reg.NameServers)), slices.Sorted(slices.Values(z.NameServers))) {
			u.Warn("registry delegates to %s", strings.Join(reg.NameServers, ", "))
		}
	}

	parent := "no DS at the parent"
	switch {
	case z.ParentErr != "":
		parent = "parent DS unknown (" + z.ParentErr + ")"
	case len(z.ParentDS) > 0:
		parent = "parent DS " + strings.Join(z.ParentDS, "; ")
	}
	level := ui.Info
	if z.DNSSEC.Status == "active" && len(z.ParentDS) > 0 {
		level = ui.OK
	}
	u.Row(level, "dnssec "+z.DNSSEC.Status+" · "+parent)
	if z.DNSSEC.Status == "pending" && z.ParentErr == "" && len(z.ParentDS) == 0 && z.Registry != nil && !cloudflareRegistrar(z.Registry.Registrar) {
		showDS(u, z.Zone, z.DNSSEC)
	}

	var tls []string
	for _, id := range tlsSettings {
		tls = append(tls, id+" "+z.Settings[id])
	}
	u.Info("tls %s", strings.Join(tls, " · "))
	var visible []string
	for _, id := range visibleSettings {
		visible = append(visible, id+" "+z.Settings[id])
	}
	u.Info("visitors %s", strings.Join(visible, " · "))

	for _, c := range z.Certs {
		u.Info("edge cert %s %s (%s) %s · expires %s", c.Type, c.Status, c.Authority, strings.Join(c.Hosts, ", "), day(c.Expires))
	}

	u.Info("records %d", len(z.Records))
	for _, rec := range z.Records {
		mode := "dns only"
		if rec.Proxied {
			mode = "proxied"
		}
		u.Detail("%s %s · %s", rec.Type, rec.Name, mode)
	}

	var counts []string
	for _, ph := range z.Phases {
		n := "none"
		if ph.Rules != nil {
			n = fmt.Sprint(*ph.Rules)
		}
		counts = append(counts, ph.Label+" "+n)
	}
	u.Info("rules %s", strings.Join(counts, " · "))
}

func day(stamp string) string {
	if len(stamp) >= 10 {
		return stamp[:10]
	}
	return stamp
}
