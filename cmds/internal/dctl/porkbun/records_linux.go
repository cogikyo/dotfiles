package porkbun

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

var (
	domainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)
	namePattern   = regexp.MustCompile(`^(?:\*|[a-z0-9_-]+)(?:\.[a-z0-9_-]+)*$`)
	idPattern     = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type record struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// change is a Porkbun create/edit body; the API field is prio, not priority.
type change struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     *int   `json:"ttl,omitempty"`
	Prio    *int   `json:"prio,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
}

func validDomain(domain string) error {
	if len(domain) > 253 || !domainPattern.MatchString(domain) {
		return fmt.Errorf("domain must be an explicit lowercase ASCII domain without a scheme, path, or trailing dot; use punycode for IDNs")
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) > 63 {
			return fmt.Errorf("domain label exceeds 63 bytes")
		}
	}
	if _, err := netip.ParseAddr(domain); err == nil {
		return fmt.Errorf("domain must be a DNS domain, not an IP address")
	}
	return nil
}

func validID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("record ID must be a positive decimal Porkbun ID without signs, leading zeros, or path characters; use porkbun list for this domain")
	}
	if _, err := strconv.ParseUint(id, 10, 64); err != nil {
		return fmt.Errorf("record ID exceeds the numeric ID range")
	}
	return nil
}

func fullName(domain, name string) (string, error) {
	if name == "@" || name == "" {
		return domain, nil
	}
	if name == domain || strings.HasSuffix(name, "."+domain) || !namePattern.MatchString(name) {
		return "", fmt.Errorf("name must be @ (root) or a relative lowercase subdomain such as www, _acme-challenge, or *; do not include the domain or a trailing dot")
	}

	full := name + "." + domain
	if len(full) > 253 {
		return "", fmt.Errorf("full record name exceeds 253 bytes")
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) > 63 {
			return "", fmt.Errorf("record name label exceeds 63 bytes")
		}
	}
	return full, nil
}

func relativeName(domain, name string) (string, error) {
	// Porkbun create/edit use a blank name for the apex, not @.
	if name == domain {
		return "", nil
	}

	if !strings.HasSuffix(name, "."+domain) {
		return "", fmt.Errorf("API record name is outside the explicit domain")
	}
	name = strings.TrimSuffix(name, "."+domain)
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("API record name has an unsupported shape")
	}
	return name, nil
}

func supported(kind string) error {
	switch kind {
	case "A", "AAAA", "CNAME", "TXT", "MX", "SRV":
		return nil
	default:
		return fmt.Errorf("writes support only A, AAAA, CNAME, TXT, MX, and SRV records")
	}
}

func options(kind string, ttl, prio *int) error {
	if ttl != nil && (*ttl < 0 || *ttl > 2147483647) {
		return fmt.Errorf("TTL must be 0 (account minimum) or a positive 32-bit integer; Porkbun validates the account minimum")
	}
	if prio != nil && (kind != "MX" && kind != "SRV" || *prio < 0 || *prio > 65535) {
		return fmt.Errorf("priority is only valid for MX/SRV and must be between 0 and 65535")
	}
	return nil
}

func decodeRecords(domain string, data json.RawMessage) ([]record, error) {
	// Retrieve encodes id, ttl, and prio as strings; create/edit send ttl and prio as integers.
	var wire []struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		Type    string  `json:"type"`
		Content *string `json:"content"`
		TTL     string  `json:"ttl"`
		Prio    *string `json:"prio"`
		Notes   *string `json:"notes"`
	}
	if json.Unmarshal(data, &wire) != nil || wire == nil {
		return nil, fmt.Errorf("API did not return a records array; cannot establish current DNS state")
	}

	records := make([]record, 0, len(wire))
	seen := make(map[string]bool, len(wire))
	for _, item := range wire {
		if validID(item.ID) != nil || seen[item.ID] || item.Type == "" || item.Content == nil {
			return nil, fmt.Errorf("API returned incomplete records or invalid/duplicate record IDs")
		}
		seen[item.ID] = true
		if _, err := relativeName(domain, item.Name); err != nil {
			return nil, err
		}
		ttl, err := strconv.Atoi(item.TTL)
		if err != nil || ttl <= 0 || ttl > 2147483647 {
			return nil, fmt.Errorf("API returned an invalid TTL for ID %s", item.ID)
		}

		r := record{ID: item.ID, Name: item.Name, Type: item.Type, Content: *item.Content, TTL: ttl}
		if item.Type == "MX" || item.Type == "SRV" {
			if item.Prio == nil {
				return nil, fmt.Errorf("API omitted priority for ID %s", item.ID)
			}
			prio, err := strconv.Atoi(*item.Prio)
			if err != nil || prio < 0 || prio > 65535 {
				return nil, fmt.Errorf("API returned invalid priority for ID %s", item.ID)
			}
			r.Priority = &prio
		}
		if item.Notes != nil {
			r.Notes = *item.Notes
		}
		records = append(records, r)
	}
	return records, nil
}

func (r record) priority() int {
	if r.Priority == nil {
		return 0
	}
	return *r.Priority
}

func (r record) same(other record) bool {
	return r.ID == other.ID && r.Name == other.Name && r.Type == other.Type && r.Content == other.Content && r.TTL == other.TTL && r.priority() == other.priority() && r.Notes == other.Notes
}

func (r record) describe() string {
	ttl := strconv.Itoa(r.TTL)
	if r.TTL == 0 {
		ttl = "account minimum"
	}
	prio := "n/a"
	if r.Priority != nil {
		prio = strconv.Itoa(*r.Priority)
	}
	id := r.ID
	if id == "" {
		id = "new"
	}
	return fmt.Sprintf("%s %s ID=%s content=%q TTL=%s priority=%s notes=%q", r.Name, r.Type, id, r.Content, ttl, prio, r.Notes)
}

func findRecord(records []record, id string) *record {
	for _, r := range records {
		if r.ID == id {
			return &r
		}
	}
	return nil
}

// duplicate treats another record with the same name, type, content, and priority as a clash, ignoring TTL.
func duplicate(records []record, proposed record) *record {
	for _, r := range records {
		if r.ID != proposed.ID && r.Name == proposed.Name && r.Type == proposed.Type && r.Content == proposed.Content && r.priority() == proposed.priority() {
			return &r
		}
	}
	return nil
}
