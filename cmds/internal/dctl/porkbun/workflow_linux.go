package porkbun

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"dotfiles/cmds/internal/dctl/ui"
)

type result struct {
	Action   string   `json:"action"`
	Domain   string   `json:"domain"`
	Outcome  string   `json:"outcome"`
	DryRun   bool     `json:"dryRun"`
	Before   *record  `json:"current"`
	Proposed *record  `json:"proposed"`
	After    *record  `json:"observed,omitempty"`
	Records  []record `json:"records"`
	Evidence []string `json:"evidence"`
	Warnings []string `json:"warnings"`
	Error    string   `json:"error,omitempty"`
}

func open(home, domain string) (*client, error) {
	if err := validDomain(domain); err != nil {
		return nil, err
	}

	return newClient(home)
}

// preflight requires a warning-free DNS retrieve and matching domain metadata with notLocal=0.
// Those checks are Porkbun API evidence, not independent DNS resolution.
func preflight(ctx context.Context, c *client, r *result) ([]record, error) {
	records, warnings, err := c.records(ctx, r.Domain)
	r.Warnings = append(r.Warnings, warnings...)
	if err != nil {
		return nil, err
	}
	r.Evidence = append(r.Evidence, fmt.Sprintf("DNS read: %d editable records retrieved", len(records)))
	if len(warnings) > 0 {
		return nil, fmt.Errorf("DNS response carries warnings; refusing writes to a potentially inactive zone")
	}

	warnings, err = c.authority(ctx, r.Domain)
	r.Warnings = append(r.Warnings, warnings...)
	if err != nil {
		return nil, err
	}
	r.Evidence = append(r.Evidence, "Authority: matching Porkbun domain metadata reports notLocal=0; this is API evidence, not independent DNS resolution")
	return records, nil
}

func target(ctx context.Context, c *client, r *result, id string) (*record, error) {
	records, err := preflight(ctx, c, r)
	if err != nil {
		return nil, err
	}

	current := findRecord(records, id)
	if current == nil {
		return nil, fmt.Errorf("record ID %s is not in domain %s; no change made", id, r.Domain)
	}
	r.Before = current
	if err := supported(current.Type); err != nil {
		return nil, err
	}
	return current, nil
}

func mutate(ctx context.Context, u *ui.UI, c *client, r result, payload any) error {
	preview(u, r)

	if r.DryRun {
		r.Outcome = "dry-run"
		// Edit and delete dry-run stay local; only create has a documented dryRun request.
		if r.Action != "create" {
			r.Evidence = append(r.Evidence, "Local preview only; no edit/delete endpoint called and no mutation permission proven")
			return report(u, r, nil)
		}

		body := payload.(change)
		body.DryRun = true
		res, err := c.request(ctx, http.MethodPost, "/dns/create/"+r.Domain, body)
		r.Warnings = append(r.Warnings, res.warnings()...)
		if err != nil {
			return report(u, r, err)
		}
		if !res.WouldSucceed || res.ID != "" {
			return report(u, r, fmt.Errorf("unexpected DNS-create dry-run evidence; require wouldSucceed=true and no record ID"))
		}
		if len(r.Warnings) > 0 {
			return report(u, r, fmt.Errorf("DNS-create dry-run returned warnings; writes are not cleared"))
		}

		r.Evidence = append(r.Evidence, "Provider DNS-create validation: dryRun=true requested, wouldSucceed=true; no record created under the documented API contract")
		return report(u, r, nil)
	}

	if !u.Yes() {
		if !u.Can() {
			return report(u, r, fmt.Errorf("write requires --yes in JSON or non-TTY mode; use --dry-run to preview without consent"))
		}
		identity := r.Proposed
		if r.Before != nil {
			identity = r.Before
		}
		ok, err := u.Confirm(fmt.Sprintf("%s in domain %s: %s, exactly as previewed?", r.Action, r.Domain, identity.describe()))
		if err != nil {
			return report(u, r, err)
		}
		if !ok {
			r.Outcome = "cancelled"
			return report(u, r, nil)
		}
	}

	records, err := preflight(ctx, c, &r)
	if err != nil {
		return report(u, r, err)
	}
	if r.Before != nil {
		current := findRecord(records, r.Before.ID)
		if current == nil || !current.same(*r.Before) {
			return report(u, r, fmt.Errorf("record changed after preview; no mutation sent; list and review again"))
		}
	}
	if r.Proposed != nil {
		if match := duplicate(records, *r.Proposed); match != nil {
			return report(u, r, fmt.Errorf("proposed content and priority duplicate record ID %s; no mutation sent", match.ID))
		}
		if r.Before != nil && r.Before.same(*r.Proposed) {
			r.Outcome = "unchanged"
			r.Evidence = append(r.Evidence, "Requested state already exists; no mutation sent")
			return report(u, r, nil)
		}
	}

	path := "/dns/" + r.Action + "/" + r.Domain
	id := ""
	if r.Before != nil {
		id = r.Before.ID
		path += "/" + id
	}
	res, err := c.request(ctx, http.MethodPost, path, payload)
	r.Warnings = append(r.Warnings, res.warnings()...)
	if err != nil {
		return uncertain(u, r, err)
	}
	if res.DryRun {
		return uncertain(u, r, fmt.Errorf("live mutation unexpectedly returned dryRun=true"))
	}

	if r.Action == "create" {
		if validID(res.ID) != nil || findRecord(records, res.ID) != nil {
			return uncertain(u, r, fmt.Errorf("create did not return a valid new record ID"))
		}
		id = res.ID
		r.Proposed.ID = id
	}

	observed, warnings, err := c.records(ctx, r.Domain)
	r.Warnings = append(r.Warnings, warnings...)
	if err != nil {
		return uncertain(u, r, fmt.Errorf("mutation response succeeded but readback failed: %w", err))
	}
	r.After = findRecord(observed, id)
	if len(r.Warnings) > 0 {
		return uncertain(u, r, fmt.Errorf("mutation or readback returned warnings; active-zone outcome is unverified"))
	}
	if r.Action == "delete" {
		if r.After != nil {
			return uncertain(u, r, fmt.Errorf("record still exists in API readback"))
		}
	} else {
		want := *r.Proposed
		// TTL 0 means the account minimum; compare against the TTL Porkbun stored.
		if r.After != nil && want.TTL == 0 {
			want.TTL = r.After.TTL
		}
		if r.After == nil || !r.After.same(want) {
			return uncertain(u, r, fmt.Errorf("API readback does not exactly match the proposed record, including preserved fields"))
		}
	}

	r.Outcome = "verified"
	r.Evidence = append(r.Evidence, "One mutation sent; intended record state/absence verified in Porkbun API readback, not public DNS propagation")
	return report(u, r, nil)
}

func uncertain(u *ui.UI, r result, cause error) error {
	r.Outcome = "uncertain"
	identity := ""
	if r.Proposed != nil {
		identity = r.Proposed.describe()
	} else if r.Before != nil {
		identity = r.Before.describe()
	}

	return report(u, r, fmt.Errorf("uncertain %s outcome in domain %s (%s): %w; mutation may have succeeded; inspect porkbun list before any manual retry; no retry or rollback attempted", r.Action, r.Domain, identity, cause))
}

func preview(u *ui.UI, r result) {
	u.Header("Porkbun %s: domain %s", r.Action, r.Domain)
	if r.Before == nil {
		u.KV("current", "no record targeted; existing records remain unchanged")
	} else {
		u.KV("current", r.Before.describe())
	}

	if r.Proposed == nil {
		u.KV("proposed", "record absent (delete this ID only)")
	} else {
		u.KV("proposed", r.Proposed.describe())
	}
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
	if r.Warnings == nil {
		r.Warnings = []string{}
	}

	if u.JSON() {
		return errors.Join(err, u.Emit(r))
	}

	for _, warning := range r.Warnings {
		u.Warn("%s", warning)
	}
	for _, evidence := range r.Evidence {
		u.Info("%s", evidence)
	}
	if r.Action == "list" {
		u.Header("Porkbun records: %s", r.Domain)
		for _, record := range r.Records {
			u.Info("%s", record.describe())
		}
	}
	if r.After != nil {
		u.KV("observed", r.After.describe())
	}
	if err == nil {
		u.OK("%s: %s", r.Action, r.Outcome)
	}
	return err
}
