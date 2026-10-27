package cloudflare

import (
	"context"
	"fmt"
	"path/filepath"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

type Cmd struct {
	Plan   PlanCmd   `cmd:"" default:"withargs" help:"Preview what would make Cloudflare match the sites file; read-only (default)."`
	Apply  ApplyCmd  `cmd:"" help:"Apply the previewed rules, tunnel ingress, DNS, TLS, and DNSSEC changes after consent, then verify by readback."`
	Status StatusCmd `cmd:"" help:"Show each zone's registration, DNSSEC, TLS, certificates, records, and rule counts; read-only."`
	Cert   CertCmd   `cmd:"" help:"Issue a Cloudflare Origin CA certificate after consent, with the key generated locally."`
}

type source struct {
	Sites string `type:"path" placeholder:"PATH" help:"Sites file (default ~/cogikyo/cogikyo/vagarios/sites.toml)."`
}

type PlanCmd struct {
	source `embed:""`
}

type ApplyCmd struct {
	source `embed:""`
}

func (s source) open(ctx context.Context, root paths.Root, action string) (*client, *sites, result, error) {
	path := s.Sites
	if path == "" {
		path = filepath.Join(root.Home, "cogikyo/cogikyo/vagarios/sites.toml")
	}
	r := result{Action: action, Sites: path}
	file, err := load(path)
	if err != nil {
		return nil, nil, r, err
	}
	c, err := newClient(root.Home)
	if err != nil {
		return nil, nil, r, err
	}
	r.Plan, err = survey(ctx, c, file)
	return c, file, r, err
}

func (cmd *PlanCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	c, _, r, err := cmd.open(ctx, root, "plan")
	if c == nil {
		return err
	}
	r.Evidence = append(r.Evidence, readOnly(c))
	if err != nil {
		return report(u, r, err)
	}

	preview(u, r.Plan)
	r.Outcome = "unchanged"
	if n := r.Plan.pending(); n > 0 {
		r.Outcome = fmt.Sprintf("%d changes pending", n)
		r.Evidence = append(r.Evidence, order)
	}
	return report(u, r, nil)
}

func (cmd *ApplyCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	c, file, r, err := cmd.open(ctx, root, "apply")
	if c == nil {
		return err
	}
	if err != nil {
		return report(u, r, err)
	}

	preview(u, r.Plan)
	if r.Plan.pending() == 0 {
		r.Outcome = "unchanged"
		r.Evidence = append(r.Evidence, "Cloudflare already matches the sites file; nothing sent")
		return report(u, r, nil)
	}
	r.Evidence = append(r.Evidence, order)
	return apply(ctx, u, c, file, r)
}
