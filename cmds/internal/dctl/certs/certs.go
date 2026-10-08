package certs

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/home"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

const (
	renewal = 30 * 24 * time.Hour
	bundle  = "/etc/ssl/certs/ca-certificates.crt"
)

var hosts = []string{"localhost", "local.leadpier.com", "local.cullyn.dev", "127.0.0.1", "::1"}

type leaf struct{ dir, cert, key string }

func Stage(r paths.Root, run execx.Runner) setup.Stage {
	dir := filepath.Join(r.Home, ".local", "share", "dev-certs")
	l := leaf{dir, filepath.Join(dir, "localhost.pem"), filepath.Join(dir, "localhost-key.pem")}
	return setup.Stage{Name: "certs", Items: []setup.Item{
		{
			Name: "certs-ca",
			Check: func(ctx context.Context) error {
				profile, err := prerequisites(r.Home)
				if err != nil {
					return err
				}
				root, _, err := ca(ctx, run)
				if err != nil {
					return err
				}
				if err := systemTrusts(root); err != nil {
					return err
				}
				return firefoxTrusts(ctx, run, root, profile)
			},
			Fix: func(ctx context.Context) error {
				profile, err := prerequisites(r.Home)
				if err != nil {
					return err
				}
				if err := execx.Interactive(run).Run(ctx, "", "mkcert", "-install"); err != nil {
					return fmt.Errorf("mkcert -install: %w", err)
				}
				root, path, err := ca(ctx, run)
				if err != nil {
					return err
				}
				return trustInFirefox(ctx, run, root, path, profile)
			},
		},
		{
			Name: "certs-leaf",
			Check: func(ctx context.Context) error {
				root, _, err := ca(ctx, run)
				if err != nil {
					return setup.Manual("needs a valid mkcert CA (certs-ca): %v", err)
				}
				if err := l.modes(); err != nil {
					return err
				}
				return verify(l.cert, l.key, root, time.Now())
			},
			Fix: func(ctx context.Context) error {
				root, _, err := ca(ctx, run)
				if err != nil {
					return setup.Manual("needs a valid mkcert CA (certs-ca): %v", err)
				}
				return l.generate(ctx, run, root)
			},
		},
	}}
}

func prerequisites(homeDir string) (string, error) {
	for _, tool := range []string{"mkcert", "certutil"} {
		if _, err := exec.LookPath(tool); err != nil {
			return "", setup.Manual("%s not found; run dctl setup packages", tool)
		}
	}
	return home.FirefoxProfile(homeDir)
}

func ca(ctx context.Context, run execx.Runner) (*x509.Certificate, string, error) {
	dir, err := run.Output(ctx, "", "mkcert", "-CAROOT")
	if err != nil {
		return nil, "", fmt.Errorf("locate mkcert CA: %w", err)
	}
	path := filepath.Join(strings.TrimSpace(dir), "rootCA.pem")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("read mkcert CA: %w", err)
	}
	cert, err := parsePEM(data)
	if err != nil {
		return nil, path, fmt.Errorf("parse mkcert CA: %w", err)
	}
	now := time.Now()
	switch {
	case !cert.IsCA:
		return nil, path, errors.New("mkcert root certificate is not a CA")
	case cert.CheckSignatureFrom(cert) != nil:
		return nil, path, errors.New("mkcert CA is not self-signed")
	case now.Before(cert.NotBefore) || now.After(cert.NotAfter):
		return nil, path, errors.New("mkcert CA is outside its validity period")
	}
	return cert, path, nil
}

func parsePEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("certificate PEM block not found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func systemTrusts(root *x509.Certificate) error {
	data, err := os.ReadFile(bundle)
	if err != nil {
		return fmt.Errorf("read system trust bundle: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(data)
	if _, err := root.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		return fmt.Errorf("mkcert CA verification against %s failed: %w", bundle, err)
	}
	return nil
}

func nickname(root *x509.Certificate) string {
	return "mkcert development CA " + root.SerialNumber.String()
}

func firefoxTrusts(ctx context.Context, run execx.Runner, root *x509.Certificate, profile string) error {
	if _, err := os.Stat(filepath.Join(profile, "cert9.db")); err != nil {
		return setup.Manual("Firefox NSS database unavailable; run dctl setup firefox")
	}
	db, name := "sql:"+profile, nickname(root)
	installed, err := run.Output(ctx, "", "certutil", "-L", "-d", db, "-n", name, "-a")
	if err != nil {
		return fmt.Errorf("could not read Firefox CA %q", name)
	}
	cert, err := parsePEM([]byte(installed))
	if err != nil || !bytes.Equal(cert.Raw, root.Raw) {
		return fmt.Errorf("Firefox CA %q is invalid or differs from the current mkcert CA", name)
	}
	if _, err := run.Output(ctx, "", "certutil", "-V", "-d", db, "-n", name, "-u", "L"); err != nil {
		return fmt.Errorf("Firefox CA %q validation failed: %w", name, err)
	}
	return nil
}

func trustInFirefox(ctx context.Context, run execx.Runner, root *x509.Certificate, path, profile string) error {
	if firefoxTrusts(ctx, run, root, profile) == nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(profile, "cert9.db")); err != nil {
		return setup.Manual("Firefox NSS database unavailable; run dctl setup firefox")
	}
	db, name := "sql:"+profile, nickname(root)
	_, _ = run.Output(ctx, "", "certutil", "-D", "-d", db, "-n", name)
	_, err := run.Output(ctx, "", "certutil", "-A", "-d", db, "-n", name, "-t", "C,,", "-i", path)
	return err
}

func (l leaf) modes() error {
	for path, want := range map[string]fs.FileMode{l.dir: fs.ModeDir | 0o700, l.cert: 0o644, l.key: 0o600} {
		st, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if st.Mode() != want {
			return fmt.Errorf("%s has mode %v, want %v", path, st.Mode(), want)
		}
	}
	return nil
}

func verify(certPath, keyPath string, root *x509.Certificate, now time.Time) error {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}
	pool := x509.NewCertPool()
	pool.AddCert(root)
	if _, err := pair.Leaf.Verify(x509.VerifyOptions{Roots: pool, CurrentTime: now}); err != nil {
		return fmt.Errorf("verify certificate chain: %w", err)
	}
	for _, host := range hosts {
		if err := pair.Leaf.VerifyHostname(host); err != nil {
			return err
		}
	}
	if !pair.Leaf.NotAfter.After(now.Add(renewal)) {
		return fmt.Errorf("certificate expires within %s", renewal)
	}
	return nil
}

func (l leaf) generate(ctx context.Context, run execx.Runner, root *x509.Certificate) error {
	if st, err := os.Lstat(l.dir); err == nil && !st.IsDir() {
		return fmt.Errorf("%s must be a directory, not a symlink or file", l.dir)
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(l.dir, 0o700); err != nil {
		return err
	}
	for _, path := range []string{l.cert, l.key} {
		if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace %s: not a regular file", path)
		}
	}
	if verify(l.cert, l.key, root, time.Now()) == nil {
		return errors.Join(os.Chmod(l.cert, 0o644), os.Chmod(l.key, 0o600))
	}
	tmp, err := os.MkdirTemp(l.dir, ".mkcert-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	cert, key := filepath.Join(tmp, "cert.pem"), filepath.Join(tmp, "key.pem")
	if err := run.Run(ctx, "", "mkcert", append([]string{"-cert-file", cert, "-key-file", key}, hosts...)...); err != nil {
		return fmt.Errorf("mkcert: %w", err)
	}
	if err := verify(cert, key, root, time.Now()); err != nil {
		return fmt.Errorf("mkcert produced an invalid certificate: %w", err)
	}
	if err := errors.Join(os.Chmod(cert, 0o644), os.Chmod(key, 0o600)); err != nil {
		return err
	}
	if err := os.Rename(key, l.key); err != nil {
		return err
	}
	return os.Rename(cert, l.cert)
}
