package cloudflare

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/netip"
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
	Origin *origin `toml:"origin" json:"origin"`
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
	Path   string              `toml:"-" json:"path"`
	Tunnel string              `toml:"tunnel" json:"tunnel"`
	Allow  map[string][]string `toml:"allow" json:"allow"`
	Zones  map[string]struct {
		Allow string `toml:"allow" json:"allow"`
	} `toml:"zone" json:"zones"`
	Sites []site `toml:"site" json:"sites"`
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
		for j, r := range site.Routes {
			if err := checkPath(r.Path); err != nil {
				return fail("%s: route %d: %v", label, j+1, err)
			}
			if err := checkOrigin(r.Origin); err != nil {
				return fail("%s: route %d (%s): origin: %v", label, j+1, r.Path, err)
			}
		}
	}
	return s, nil
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

	for i := range s.Sites {
		site := &s.Sites[i]
		site.Zone = ""
		for _, zone := range zones {
			if (site.Host == zone || strings.HasSuffix(site.Host, "."+zone)) && len(zone) > len(site.Zone) {
				site.Zone = zone
			}
		}
		if site.Zone == "" {
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
	return nil
}
