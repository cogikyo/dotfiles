package cloudflare

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const public = "public"

var (
	hostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)
	listPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
)

type origin struct {
	Scheme      string `toml:"scheme" json:"scheme"`
	Host        string `toml:"host" json:"host"`
	Port        int    `toml:"port" json:"port"`
	NoTLSVerify bool   `toml:"noTLSVerify" json:"noTLSVerify,omitempty"`
}

func (o origin) service() string {
	return o.Scheme + "://" + o.Host + ":" + strconv.Itoa(o.Port)
}

type route struct {
	Path   string  `toml:"path" json:"path"`
	Stream bool    `toml:"stream" json:"stream,omitempty"`
	Origin *origin `toml:"origin" json:"origin"`
}

type tlsPolicy struct {
	Mode  string `toml:"mode" json:"mode"`
	Min   string `toml:"min" json:"min"`
	HTTPS *bool  `toml:"https" json:"https"`
}

type redirect struct {
	From string `toml:"from" json:"from"`
	To   string `toml:"to" json:"to"`
	Zone string `toml:"-" json:"zone"`
}

type site struct {
	Host   string  `toml:"host" json:"host"`
	Allow  *string `toml:"allow" json:"-"`
	Origin *origin `toml:"origin" json:"origin"`
	Routes []route `toml:"route" json:"routes,omitempty"`
	Zone   string  `toml:"-" json:"zone"`
	Access string  `toml:"-" json:"access"`
}

type sites struct {
	Path        string              `toml:"-" json:"path"`
	Tunnel      string              `toml:"tunnel" json:"tunnel"`
	DNSSEC      *bool               `toml:"dnssec" json:"dnssec,omitempty"`
	Transparent *bool               `toml:"transparent" json:"transparent,omitempty"`
	TLS         *tlsPolicy          `toml:"tls" json:"tls,omitempty"`
	Allow       map[string][]string `toml:"allow" json:"allow"`
	Zones       map[string]struct {
		Allow    string `toml:"allow" json:"allow"`
		Fallback string `toml:"fallback" json:"fallback,omitempty"`
	} `toml:"zone" json:"zones"`
	Redirects []redirect `toml:"redirect" json:"redirects,omitempty"`
	Sites     []site     `toml:"site" json:"sites"`
}

func load(path string) (*sites, error) {
	fail := func(format string, args ...any) (*sites, error) {
		return nil, fmt.Errorf("%s: %s", path, fmt.Sprintf(format, args...))
	}

	s := &sites{Path: path}
	meta, err := toml.DecodeFile(path, s)
	var unreadable *fs.PathError
	if errors.As(err, &unreadable) {
		return nil, fmt.Errorf("cannot read sites file: %w; pass --sites PATH", err)
	}
	if err != nil {
		return fail("%v", err)
	}
	if extra := meta.Undecoded(); len(extra) > 0 {
		keys := make([]string, len(extra))
		for i, k := range extra {
			keys[i] = k.String()
		}
		return fail("unknown keys: %s", strings.Join(keys, ", "))
	}

	if s.Tunnel == "" {
		return fail("tunnel is required; name the Cloudflare tunnel, not its ID")
	}
	if s.DNSSEC != nil && !*s.DNSSEC {
		return fail("dnssec = false is not supported; omit dnssec to leave each zone's DNSSEC as it is")
	}
	if s.Transparent != nil && !*s.Transparent {
		return fail("transparent = false is not supported; omit transparent to leave browser_check, email_obfuscation, and security_level as they are")
	}
	if err := checkTLS(s.TLS); err != nil {
		return fail("[tls] %v", err)
	}

	for _, name := range slices.Sorted(maps.Keys(s.Allow)) {
		if name == public {
			return fail("[allow] cannot define %q; it is reserved for no restriction", public)
		}
		if !listPattern.MatchString(name) {
			return fail("[allow] name %q must be lowercase letters, digits, and inner hyphens", name)
		}
		if err := checkList(s.Allow[name]); err != nil {
			return fail("[allow] %s: %v", name, err)
		}
	}
	known := func(name string) bool {
		_, ok := s.Allow[name]
		return ok || name == public
	}

	for _, name := range slices.Sorted(maps.Keys(s.Zones)) {
		allow := s.Zones[name].Allow
		if !validHost(name) {
			return fail("[zone.%q] must name a lowercase zone such as example.com, with labels of at most 63 bytes", name)
		}
		if allow == "" {
			return fail("[zone.%q] needs allow, the default allowlist for its subdomains (%q opts out)", name, public)
		}
		if !known(allow) {
			return fail("[zone.%q] allow %q is not defined in [allow]", name, allow)
		}
		if f := s.Zones[name].Fallback; f != "" {
			if _, err := fallbackHost(f); err != nil {
				return fail("[zone.%q] fallback %q %v", name, f, err)
			}
		}
	}

	seen := map[string]bool{}
	for i, site := range s.Sites {
		label := fmt.Sprintf("site %d (%s)", i+1, site.Host)
		if !validHost(site.Host) {
			return fail("%s: host must be a lowercase FQDN without scheme, port, wildcard, or trailing dot, with labels of at most 63 bytes and at most 253 bytes overall", label)
		}
		if seen[site.Host] {
			return fail("%s: duplicate host; each host appears once", label)
		}
		seen[site.Host] = true
		if site.Allow != nil && !known(*site.Allow) {
			return fail("%s: allow %q is not defined in [allow]; define it or use %q", label, *site.Allow, public)
		}
		if err := checkOrigin(site.Origin); err != nil {
			return fail("%s: origin: %v", label, err)
		}
		streams := map[string]bool{}
		for j, r := range site.Routes {
			if err := checkPath(r.Path); err != nil {
				return fail("%s: route %d: %v", label, j+1, err)
			}
			if _, ok := streamPrefix(r.Path); r.Stream && !ok {
				return fail("%s: route %d: stream needs a literal path prefix such as ^/v1/api/sse, not %q; the Free plan's rule expressions have no regex", label, j+1, r.Path)
			}
			if r.Stream && streams[r.Path] {
				return fail("%s: route %d: duplicate stream path %q; each path streams once per host", label, j+1, r.Path)
			}
			if r.Stream {
				streams[r.Path] = true
			}
			if err := checkOrigin(r.Origin); err != nil {
				return fail("%s: route %d (%s): origin: %v", label, j+1, r.Path, err)
			}
		}
	}

	from := map[string]bool{}
	for i, r := range s.Redirects {
		label := fmt.Sprintf("redirect %d (%s)", i+1, r.From)
		if !validHost(r.From) {
			return fail("%s: from must be a lowercase FQDN without scheme, path, or wildcard", label)
		}
		if from[r.From] {
			return fail("%s: duplicate from; each host redirects once", label)
		}
		from[r.From] = true
		if seen[r.From] {
			return fail("%s: from is also a [[site]] host; a host either serves a site or redirects", label)
		}
		host, ok := strings.CutPrefix(r.To, "https://")
		if !ok || !validHost(host) {
			return fail("%s: to %q must be https:// and a lowercase host, without path or trailing slash; dctl keeps the request's path and query", label, r.To)
		}
		if host == r.From {
			return fail("%s: to points back at from", label)
		}
	}
	return s, nil
}

func fallbackHost(target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || !validHost(u.Host) || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", errors.New("must be an https:// URL with a lowercase host and optional path, without port, query, or fragment; dctl appends ?from=<host><path>")
	}
	return u.Host, nil
}

func checkTLS(t *tlsPolicy) error {
	if t == nil {
		return nil
	}
	switch t.Mode {
	case "off", "flexible", "full", "strict":
	default:
		return fmt.Errorf("mode %q must be off, flexible, full, or strict", t.Mode)
	}
	switch t.Min {
	case "1.0", "1.1", "1.2", "1.3":
	default:
		return fmt.Errorf("min %q must be \"1.0\", \"1.1\", \"1.2\", or \"1.3\"", t.Min)
	}
	if t.HTTPS == nil {
		return fmt.Errorf("https is required (true or false)")
	}
	return nil
}

func streamPrefix(path string) (string, bool) {
	prefix := strings.TrimPrefix(path, "^")
	return prefix, strings.HasPrefix(prefix, "/") && regexp.QuoteMeta(prefix) == prefix
}

func validHost(name string) bool {
	if len(name) > 253 || !hostPattern.MatchString(name) {
		return false
	}
	return !slices.ContainsFunc(strings.Split(name, "."), func(label string) bool { return len(label) > 63 })
}

func checkList(entries []string) error {
	if len(entries) == 0 {
		return fmt.Errorf("list is empty; an empty allowlist would block everyone")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry] {
			return fmt.Errorf("%q appears twice", entry)
		}
		seen[entry] = true
		if addr, err := netip.ParseAddr(entry); err == nil {
			if addr.Zone() != "" {
				return fmt.Errorf("%q carries an IPv6 zone", entry)
			}
			continue
		}
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return fmt.Errorf("%q is not an IP address or CIDR", entry)
		}
		if prefix != prefix.Masked() {
			return fmt.Errorf("%q has host bits set; write %s", entry, prefix.Masked())
		}
	}
	return nil
}

func checkOrigin(o *origin) error {
	if o == nil {
		return fmt.Errorf("required as { scheme, host, port }")
	}
	switch o.Scheme {
	case "http", "https", "ssh":
	default:
		return fmt.Errorf("scheme %q must be http, https, or ssh", o.Scheme)
	}
	addr, err := netip.ParseAddr(o.Host)
	if err != nil || !addr.Is4() {
		return fmt.Errorf("host %q must be an IPv4 address", o.Host)
	}
	if o.Port < 1 || o.Port > 65535 {
		return fmt.Errorf("port %d must be 1–65535", o.Port)
	}
	if o.NoTLSVerify && o.Scheme != "https" {
		return fmt.Errorf("noTLSVerify applies only to https, not %s", o.Scheme)
	}
	return nil
}

func checkPath(path string) error {
	if !strings.HasPrefix(path, "^") {
		return fmt.Errorf("path %q must start with ^; ingress paths are regexes and match anywhere unless anchored", path)
	}
	if _, err := regexp.Compile(path); err != nil {
		return fmt.Errorf("path %q is not a valid regex: %v", path, err)
	}
	return nil
}

func zoneOf(host string, zones []string) string {
	best := ""
	for _, zone := range zones {
		if (host == zone || strings.HasSuffix(host, "."+zone)) && len(zone) > len(best) {
			best = zone
		}
	}
	return best
}

func (s *sites) resolve(zones []string) error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", s.Path, fmt.Sprintf(format, args...))
	}
	list := strings.Join(zones, ", ")

	for _, name := range slices.Sorted(maps.Keys(s.Zones)) {
		if !slices.Contains(zones, name) {
			return fail("[zone.%q] is not a zone this token can see (%s)", name, list)
		}
	}

	for i := range s.Redirects {
		r := &s.Redirects[i]
		if r.Zone = zoneOf(r.From, zones); r.Zone == "" {
			return fail("redirect from %q is outside the account's zones (%s)", r.From, list)
		}
	}

	for i := range s.Sites {
		site := &s.Sites[i]
		if site.Zone = zoneOf(site.Host, zones); site.Zone == "" {
			return fail("site %q is outside the account's zones (%s)", site.Host, list)
		}

		policy, managed := s.Zones[site.Zone]
		switch {
		case site.Allow != nil:
			site.Access = *site.Allow
		case site.Host == site.Zone:
			site.Access = public
		case managed:
			site.Access = policy.Allow
		default:
			return fail("site %q has no effective allowlist: set allow, or add [zone.%q] with a default allow", site.Host, site.Zone)
		}
		if site.Access != public && !managed {
			return fail("site %q uses allowlist %q, but zone %q has no [zone] table, so dctl writes no WAF rules there; add [zone.%q]", site.Host, site.Access, site.Zone, site.Zone)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(s.Zones)) {
		target := s.Zones[name].Fallback
		if target == "" {
			continue
		}
		host, _ := fallbackHost(target)
		home := zoneOf(host, zones)
		if home == "" || s.Zones[home].Fallback == "" {
			continue
		}
		if !slices.ContainsFunc(s.Sites, func(site site) bool { return site.Host == host && site.Access == public }) {
			return fail("[zone.%q] fallback %s would loop: zone %s sends its unknown and blocked requests to a fallback, so %s must be a public [[site]]", name, target, home, host)
		}
	}
	return nil
}
