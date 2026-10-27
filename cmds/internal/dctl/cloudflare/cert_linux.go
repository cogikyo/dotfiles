package cloudflare

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

const validity = 5475

type CertCmd struct {
	Hosts []string `arg:"" name:"host" help:"Hostnames the Origin CA certificate covers; the first names the files."`
	Out   string   `type:"path" required:"" placeholder:"DIR" help:"Existing directory for <host>.pem and <host>.key; existing files are never overwritten."`
}

type certResult struct {
	Action   string   `json:"action"`
	Hosts    []string `json:"hosts"`
	Outcome  string   `json:"outcome"`
	ID       string   `json:"id,omitempty"`
	Expires  string   `json:"expires_on,omitempty"`
	Files    []string `json:"files"`
	Evidence []string `json:"evidence"`
	Error    string   `json:"error,omitempty"`
	posted   bool
}

func (cmd *CertCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	r := certResult{Action: "cert", Hosts: cmd.Hosts}
	c, err := newClient(root.Home)
	if err != nil {
		return err
	}
	err = cmd.issue(ctx, u, c, &r)
	switch {
	case r.posted:
		r.Evidence = append(r.Evidence, fmt.Sprintf("Sent %d Cloudflare requests, one of them POST /certificates", c.sent))
	case c.sent > 0:
		r.Evidence = append(r.Evidence, fmt.Sprintf("Sent %d Cloudflare requests, all GET; nothing issued", c.sent))
	}
	if err != nil {
		r.Error = err.Error()
	}
	if u.JSON() {
		return errors.Join(err, u.Emit(r))
	}
	u.Section("evidence", "")
	for _, e := range r.Evidence {
		u.Info("%s", e)
	}
	if err != nil {
		return err
	}
	u.Close(ui.OK, "cert: %s", r.Outcome)
	return nil
}

func (cmd *CertCmd) issue(ctx context.Context, u *ui.UI, c *client, r *certResult) error {
	seen := map[string]bool{}
	for _, h := range cmd.Hosts {
		if !validHost(h) {
			return fmt.Errorf("host %q must be a lowercase FQDN without scheme, path, or wildcard", h)
		}
		if seen[h] {
			return fmt.Errorf("host %s is listed twice", h)
		}
		seen[h] = true
	}
	if info, err := os.Stat(cmd.Out); err != nil || !info.IsDir() {
		return fmt.Errorf("--out %s must be an existing directory", cmd.Out)
	}
	r.Files = []string{filepath.Join(cmd.Out, cmd.Hosts[0]+".pem"), filepath.Join(cmd.Out, cmd.Hosts[0]+".key")}
	for _, f := range r.Files {
		if _, err := os.Lstat(f); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s already exists; dctl never overwrites certificate files", f)
		}
	}

	zones, err := listZones(ctx, c)
	if err != nil {
		return err
	}
	names := make([]string, len(zones))
	for i, z := range zones {
		names[i] = z.Name
	}
	for _, h := range cmd.Hosts {
		if zoneOf(h, names) == "" {
			return fmt.Errorf("host %s is outside the account's zones (%s)", h, strings.Join(names, ", "))
		}
	}

	u.Section("origin certificate", "Cloudflare Origin CA · ECDSA P-256 · "+fmt.Sprint(validity)+" days")
	for _, h := range cmd.Hosts {
		u.Info("%s", h)
	}
	for _, f := range r.Files {
		u.Row(ui.Todo, "write "+f+" (0600)")
	}

	if !u.Yes() && !u.Can() {
		return fmt.Errorf("cert requires --yes in JSON or non-TTY mode")
	}
	ok, err := u.Confirm("Issue this Origin CA certificate and write the key and certificate?")
	if err != nil {
		return err
	}
	if !ok {
		r.Outcome = "cancelled"
		return nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: cmd.Hosts[0]}, DNSNames: cmd.Hosts}, key)
	if err != nil {
		return err
	}

	body := map[string]any{
		"hostnames":          cmd.Hosts,
		"request_type":       "origin-ecc",
		"requested_validity": validity,
		"csr":                string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr})),
	}
	var issued struct {
		ID          string `json:"id"`
		Certificate string `json:"certificate"`
		Expires     string `json:"expires_on"`
	}
	c.write, r.posted = true, true
	_, err = c.request(ctx, http.MethodPost, nil, body, &issued, "certificates")
	c.write = false
	if err != nil {
		return fmt.Errorf("uncertain outcome: %w; check SSL/TLS > Origin Server in the dashboard before retrying", err)
	}
	r.ID, r.Expires = issued.ID, issued.Expires

	cert := strings.TrimRight(issued.Certificate, "\n") + "\n"
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for i, data := range [][]byte{[]byte(cert), keyPEM} {
		if err := place(r.Files[i], data); err != nil {
			return fmt.Errorf("certificate %s was issued but %w; revoke it in the dashboard or with DELETE /certificates/%s", issued.ID, err, issued.ID)
		}
	}
	r.Outcome = "issued " + issued.ID + ", expires " + day(issued.Expires)
	r.Evidence = append(r.Evidence, "Wrote "+strings.Join(r.Files, " and ")+" with mode 0600")
	return nil
}

// place uses a link rather than rename to avoid overwriting existing files.
func place(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dctl-cert-*")
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if err := errors.Join(err, tmp.Sync(), tmp.Close()); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	if err := os.Link(tmp.Name(), path); err != nil {
		return fmt.Errorf("cannot place %s: %w", path, err)
	}
	return nil
}
