package cloudflare

import (
	"fmt"
	"strings"

	"dotfiles/cmds/internal/ui"
)

const automatic = "ssl_automatic_mode"

var (
	tlsSettings     = []string{automatic, "ssl", "min_tls_version", "always_use_https"}
	visibleSettings = []string{"browser_check", "email_obfuscation", "security_level"}
)

type settingChange struct {
	Action   string  `json:"action"`
	Zone     string  `json:"zone"`
	ID       string  `json:"id"`
	Current  string  `json:"current"`
	Proposed string  `json:"proposed"`
	Denied   string  `json:"denied,omitempty"`
	DS       *dnssec `json:"ds,omitempty"`
	zoneID   string
}

type pair struct {
	ID    string
	Value string
}

func (s *sites) settings() []pair {
	var out []pair
	if t := s.TLS; t != nil {
		https := "off"
		if *t.HTTPS {
			https = "on"
		}
		out = append(out, pair{automatic, "custom"}, pair{"ssl", t.Mode}, pair{"min_tls_version", t.Min}, pair{"always_use_https", https})
	}
	if s.Transparent != nil {
		out = append(out, pair{"browser_check", "off"}, pair{"email_obfuscation", "off"}, pair{"security_level", "essentially_off"})
	}
	return out
}

func diffSettings(s *sites, l *live) []settingChange {
	var changes []settingChange
	for _, z := range l.Zones {
		for _, want := range s.settings() {
			v := l.Settings[z.Name][want.ID]
			ch := settingChange{Action: "unchanged", Zone: z.Name, ID: want.ID, Current: v.Value, Proposed: want.Value, zoneID: z.ID}
			switch {
			case v.Denied != "":
				ch.Action, ch.Denied = "denied", v.Denied
			case v.Value != want.Value:
				ch.Action = "update"
			}
			changes = append(changes, ch)
		}
		if s.DNSSEC != nil {
			d := l.DNSSEC[z.Name]
			ch := settingChange{Action: "update", Zone: z.Name, ID: "dnssec", Current: d.Status, Proposed: "active", zoneID: z.ID}
			if d.Status == "active" || d.Status == "pending" {
				ch.Action = "unchanged"
			}
			if d.DS != "" {
				ch.DS = &d
			}
			changes = append(changes, ch)
		}
	}
	return changes
}

func previewSettings(u *ui.UI, changes []settingChange) {
	zone := ""
	for _, ch := range changes {
		if ch.Zone != zone {
			zone = ch.Zone
			u.Section("settings "+zone, "")
		}
		switch ch.Action {
		case "unchanged":
			u.OK("%s %s", ch.ID, ch.Current)
		case "update":
			u.Row(ui.Todo, fmt.Sprintf("%s %s → %s", ch.ID, ch.Current, ch.Proposed))
		case "denied":
			u.Warn("%s unreadable with this token; planned without it", ch.ID)
			u.Detail("%s", ch.Denied)
		}
	}
}

func cloudflareRegistrar(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "cloudflare")
}

func showDS(u *ui.UI, zone string, d dnssec) {
	u.Row(ui.Todo, "paste this DS record at "+zone+"'s registrar")
	u.Detail("key tag %d · algorithm %s · digest type %s", d.KeyTag, d.Algorithm, d.DigestType)
	u.Detail("digest %s", d.Digest)
	u.Detail("DS record: %s", d.DS)
}
