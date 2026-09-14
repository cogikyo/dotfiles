package porkbun

import (
	"fmt"
	"net/http"
	"strings"

	"dotfiles/cmds/internal/dctl/app"
)

type Cmd struct {
	Check  CheckCmd  `cmd:"" help:"Check authentication, DNS authority, and DNS-create dry-run permission."`
	List   ListCmd   `cmd:"" help:"List editable Porkbun DNS records and their IDs."`
	Create CreateCmd `cmd:"" help:"Create one DNS record; never replace matching records."`
	Edit   EditCmd   `cmd:"" help:"Edit one DNS record by numeric ID."`
	Delete DeleteCmd `cmd:"" help:"Delete one DNS record by numeric ID."`
}

type CheckCmd struct {
	Domain string `arg:"" help:"Explicit lowercase domain (ASCII or punycode)."`
}

type ListCmd struct {
	Domain string `arg:"" help:"Explicit lowercase domain (ASCII or punycode)."`
}

type CreateCmd struct {
	Domain  string `arg:"" help:"Explicit lowercase domain (ASCII or punycode)."`
	Name    string `arg:"" help:"Relative subdomain, or @ for the root; never include the domain."`
	Type    string `arg:"" help:"A, AAAA, CNAME, TXT, MX, or SRV."`
	Content string `arg:"" help:"Record value in Porkbun format."`
	TTL     *int   `name:"ttl" help:"TTL in seconds; omitted or 0 uses the account minimum."`
	Prio    *int   `help:"MX/SRV priority (0–65535); omitted means 0."`
	DryRun  bool   `help:"Preview and ask Porkbun to validate with dryRun=true; do not create."`
}

type EditCmd struct {
	Domain  string `arg:"" help:"Explicit lowercase domain (ASCII or punycode)."`
	ID      string `arg:"" name:"id" help:"Numeric record ID from list for this domain."`
	Content string `required:"" help:"Replacement content; other fields stay unchanged unless specified."`
	TTL     *int   `name:"ttl" help:"Replace TTL in seconds; 0 uses the account minimum."`
	Prio    *int   `help:"Replace MX/SRV priority (0–65535)."`
	DryRun  bool   `help:"Read current state and preview locally; do not call edit."`
}

type DeleteCmd struct {
	Domain string `arg:"" help:"Explicit lowercase domain (ASCII or punycode)."`
	ID     string `arg:"" name:"id" help:"Numeric record ID from list for this domain."`
	DryRun bool   `help:"Read current state and preview locally; do not call delete."`
}

func (cmd *ListCmd) Run(ctx *app.Context) error {
	c, err := open(ctx, cmd.Domain)
	if err != nil {
		return err
	}

	r := result{Action: "list", Domain: cmd.Domain, Outcome: "read"}
	r.Records, r.Warnings, err = c.records(ctx, cmd.Domain)
	r.Evidence = append(r.Evidence, "Porkbun editable DNS copy only; authority and public DNS propagation are not verified by list")
	return report(ctx, r, err)
}

func (cmd *CheckCmd) Run(ctx *app.Context) error {
	c, err := open(ctx, cmd.Domain)
	if err != nil {
		return err
	}

	r := result{Action: "check", Domain: cmd.Domain, Outcome: "checked"}
	ping, err := c.request(ctx, http.MethodPost, "/ping", struct{}{})
	r.Warnings = append(r.Warnings, ping.warnings()...)
	if err != nil {
		return report(ctx, r, err)
	}
	r.Evidence = append(r.Evidence, "Authentication: POST /ping succeeded")

	if _, err := preflight(ctx, c, &r); err != nil {
		return report(ctx, r, err)
	}

	probe := change{Name: "_dctl-check", Type: "TXT", Content: "dctl Porkbun permission check", DryRun: true}
	preview, err := c.request(ctx, http.MethodPost, "/dns/create/"+cmd.Domain, probe)
	r.Warnings = append(r.Warnings, preview.warnings()...)
	if err != nil {
		return report(ctx, r, err)
	}
	if !preview.WouldSucceed || preview.ID != "" {
		return report(ctx, r, fmt.Errorf("DNS-create dry-run returned unexpected evidence; require wouldSucceed=true and no record ID"))
	}

	r.Evidence = append(r.Evidence, "DNS-create dry-run: wouldSucceed=true; requested dryRun=true, no record created under the documented API contract")
	r.Evidence = append(r.Evidence, "Live create, edit, delete, and public DNS propagation remain untested")
	if len(r.Warnings) != 0 {
		return report(ctx, r, fmt.Errorf("check received provider warnings; writes are not cleared"))
	}
	return report(ctx, r, nil)
}

func (cmd *CreateCmd) Run(ctx *app.Context) error {
	if err := validDomain(cmd.Domain); err != nil {
		return err
	}
	name, err := fullName(cmd.Domain, cmd.Name)
	if err != nil {
		return err
	}
	kind := strings.ToUpper(cmd.Type)
	if err := supported(kind); err != nil {
		return err
	}
	if err := options(kind, cmd.TTL, cmd.Prio); err != nil {
		return err
	}

	c, err := open(ctx, cmd.Domain)
	if err != nil {
		return err
	}

	proposed := record{Name: name, Type: kind, Content: cmd.Content}
	if cmd.TTL != nil {
		proposed.TTL = *cmd.TTL
	}
	if kind == "MX" || kind == "SRV" {
		proposed.Priority = new(0)
		if cmd.Prio != nil {
			proposed.Priority = cmd.Prio
		}
	}

	r := result{Action: "create", Domain: cmd.Domain, Proposed: &proposed, DryRun: cmd.DryRun}
	records, err := preflight(ctx, c, &r)
	if err != nil {
		return report(ctx, r, err)
	}
	if match := duplicate(records, proposed); match != nil {
		r.Before = match
		return report(ctx, r, fmt.Errorf("duplicate content and priority already exist at ID %s; no record created (TTL differences do not create a distinct DNS value)", match.ID))
	}

	relative, err := relativeName(cmd.Domain, name)
	if err != nil {
		return report(ctx, r, err)
	}
	payload := change{Name: relative, Type: kind, Content: cmd.Content, TTL: cmd.TTL, Prio: proposed.Priority}
	return mutate(ctx, c, r, payload)
}

func (cmd *EditCmd) Run(ctx *app.Context) error {
	if err := validID(cmd.ID); err != nil {
		return err
	}

	c, err := open(ctx, cmd.Domain)
	if err != nil {
		return err
	}

	r := result{Action: "edit", Domain: cmd.Domain, DryRun: cmd.DryRun}
	current, err := target(ctx, c, &r, cmd.ID)
	if err != nil {
		return report(ctx, r, err)
	}
	if err := options(current.Type, cmd.TTL, cmd.Prio); err != nil {
		return report(ctx, r, err)
	}

	proposed := *current
	proposed.Content = cmd.Content
	if cmd.TTL != nil {
		proposed.TTL = *cmd.TTL
	}
	if cmd.Prio != nil {
		proposed.Priority = cmd.Prio
	}
	r.Proposed = &proposed

	name, err := relativeName(cmd.Domain, current.Name)
	if err != nil {
		return report(ctx, r, err)
	}
	payload := change{Name: name, Type: current.Type, Content: proposed.Content, TTL: &proposed.TTL, Prio: proposed.Priority}
	return mutate(ctx, c, r, payload)
}

func (cmd *DeleteCmd) Run(ctx *app.Context) error {
	if err := validID(cmd.ID); err != nil {
		return err
	}

	c, err := open(ctx, cmd.Domain)
	if err != nil {
		return err
	}

	r := result{Action: "delete", Domain: cmd.Domain, DryRun: cmd.DryRun}
	if _, err := target(ctx, c, &r, cmd.ID); err != nil {
		return report(ctx, r, err)
	}
	return mutate(ctx, c, r, struct{}{})
}
