package install

import (
	"bytes"
	"context"
	"crypto"
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
	"dotfiles/cmds/internal/dctl/health"
	"dotfiles/cmds/internal/dctl/output"
	"dotfiles/cmds/internal/dctl/paths"
)

const devCertRenewal = 30 * 24 * time.Hour

var developmentCertNames = [...]string{"localhost", "local.leadpier.com", "local.cullyn.dev", "127.0.0.1", "::1"}

type devCertFiles struct {
	dir  string
	cert string
	key  string
}

func installCerts(ctx context.Context, root paths.Root, out *output.Printer, opts Options, runner execx.Runner) error {
	out.Header("Provisioning development TLS certificates")
	if err := requireCommands("mkcert", "certutil"); err != nil {
		return err
	}
	profile, err := detectFirefoxProfile(root.Home)
	if err != nil {
		return err
	}
	files := developmentCertFiles(root)
	if err := validateDevCertPaths(files); err != nil {
		return err
	}
	if err := confirmRisk("modify system and Firefox trust stores and provision a private key", opts); err != nil {
		return err
	}

	query := execx.OSRunner{}
	if opts.DryRun {
		out.Info("[dry-run] Would run mkcert -install for system trust")
		out.Info("[dry-run] Would ensure the mkcert CA is trusted by Firefox at %s", profile)
		if dryRunLeafIsCurrent(ctx, query, files) {
			out.Info("[dry-run] Would not regenerate the current valid local development certificate at %s", files.cert)
		} else {
			out.Info("[dry-run] Would generate a local development certificate and key in %s", files.dir)
		}
		out.Info("[dry-run] Would enforce modes 0700 on %s, 0644 on the certificate, and 0600 on the key", files.dir)
		return nil
	}

	out.Info("Installing the mkcert CA into system and browser trust stores")
	if _, err := runner.Run(ctx, "", "mkcert", "-install"); err != nil {
		return fmt.Errorf("install mkcert root CA: %w", err)
	}
	rootCert, rootPath, err := currentMkcertRoot(ctx, query)
	if err != nil {
		return err
	}
	if err := ensureFirefoxRoot(ctx, runner, query, rootCert, rootPath, profile); err != nil {
		return err
	}
	if err := os.MkdirAll(files.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(files.dir, 0o700); err != nil {
		return err
	}
	if err := validateDevelopmentCert(files.cert, files.key, rootCert, time.Now()); err == nil {
		out.OK("Local development certificate is current")
	} else {
		out.Info("Generating local development certificate")
		if err := generateDevelopmentCert(ctx, runner, files, rootCert); err != nil {
			return err
		}
		out.OK("Localhost certificate generated")
	}
	if err := os.Chmod(files.cert, 0o644); err != nil {
		return err
	}
	if err := os.Chmod(files.key, 0o600); err != nil {
		return err
	}
	out.OK("Development certificate: %s", files.cert)
	out.Warn("Restart Firefox if it was open during certificate installation")
	return nil
}

func certsHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	const fix = "run dctl --yes install certs"
	checks := []health.Check{
		commandCheck("certs:mkcert", "mkcert", "mkcert", health.Fail, "run dctl install packages"),
		commandCheck("certs:certutil", "certutil", "certutil", health.Fail, "run dctl install packages"),
	}
	if _, err := exec.LookPath("mkcert"); err != nil {
		return checks
	}
	if _, err := exec.LookPath("certutil"); err != nil {
		return checks
	}
	profile, err := detectFirefoxProfile(root.Home)
	if err != nil {
		return append(checks, health.Check{ID: "certs:firefox-profile", Name: "Firefox Developer Edition profile", Status: health.Skip, Observed: err.Error()})
	}
	checks = append(checks, ok("certs:firefox-profile", "Firefox Developer Edition profile", profile))

	rootCert, rootPath, err := currentMkcertRoot(ctx, runner)
	if err != nil {
		return append(checks, fail("certs:root", "mkcert root CA", err.Error(), fix))
	}
	checks = append(checks, ok("certs:root", "mkcert root CA", rootPath))
	if err := firefoxRootValid(ctx, runner, rootCert, profile); err != nil {
		checks = append(checks, fail("certs:firefox-trust", "Firefox mkcert trust", err.Error(), fix))
	} else {
		checks = append(checks, ok("certs:firefox-trust", "Firefox mkcert trust", profile))
	}
	if err := systemTrustsRoot(rootCert); err != nil {
		checks = append(checks, fail("certs:system-trust", "system mkcert trust", err.Error(), fix))
	} else {
		checks = append(checks, ok("certs:system-trust", "system mkcert trust", rootCert.Subject.String()))
	}

	files := developmentCertFiles(root)
	if err := validateDevCertModes(files); err != nil {
		checks = append(checks, fail("certs:paths", "development certificate paths", err.Error(), fix))
	} else {
		checks = append(checks, ok("certs:paths", "development certificate paths", files.dir))
	}
	if err := validateDevelopmentCert(files.cert, files.key, rootCert, time.Now()); err != nil {
		checks = append(checks, fail("certs:localhost", "local development certificate", err.Error(), fix))
	} else {
		checks = append(checks, ok("certs:localhost", "local development certificate", files.cert))
	}
	return checks
}

func requireCommands(names ...string) error {
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("%s not found; run the packages step first", name)
		}
	}
	return nil
}

func developmentCertFiles(root paths.Root) devCertFiles {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(root.Home, ".local", "share")
	}
	dir := filepath.Join(data, "dev-certs")
	return devCertFiles{dir: dir, cert: filepath.Join(dir, "localhost.pem"), key: filepath.Join(dir, "localhost-key.pem")}
}

func validateDevCertPaths(files devCertFiles) error {
	if st, err := os.Lstat(files.dir); err == nil {
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return fmt.Errorf("development certificate directory must be a local directory: %s", files.dir)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, path := range []string{files.cert, files.key} {
		if st, err := os.Lstat(path); err == nil {
			if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
				return fmt.Errorf("development certificate path must be a local regular file: %s", path)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func validateDevCertModes(files devCertFiles) error {
	if err := validateDevCertPaths(files); err != nil {
		return err
	}
	for path, want := range map[string]fs.FileMode{files.dir: 0o700, files.cert: 0o644, files.key: 0o600} {
		st, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if st.Mode().Perm() != want {
			return fmt.Errorf("%s has mode %04o; want %04o", path, st.Mode().Perm(), want)
		}
	}
	return nil
}

func dryRunLeafIsCurrent(ctx context.Context, runner execx.Runner, files devCertFiles) bool {
	rootCert, _, err := currentMkcertRoot(ctx, runner)
	return err == nil && validateDevelopmentCert(files.cert, files.key, rootCert, time.Now()) == nil
}

func currentMkcertRoot(ctx context.Context, runner execx.Runner) (*x509.Certificate, string, error) {
	caroot, err := runner.Output(ctx, "", "mkcert", "-CAROOT")
	if err != nil {
		return nil, "", fmt.Errorf("locate mkcert root CA: %w", err)
	}
	path := filepath.Join(strings.TrimSpace(caroot), "rootCA.pem")
	cert, err := readCertificate(path)
	if err != nil {
		return nil, path, fmt.Errorf("read mkcert root CA: %w", err)
	}
	if !cert.IsCA {
		return nil, path, errors.New("mkcert root certificate is not a CA")
	}
	if err := cert.CheckSignatureFrom(cert); err != nil {
		return nil, path, fmt.Errorf("verify mkcert root CA: %w", err)
	}
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return nil, path, errors.New("mkcert root CA is outside its validity period")
	}
	return cert, path, nil
}

func ensureFirefoxRoot(ctx context.Context, runner, query execx.Runner, root *x509.Certificate, rootPath, profile string) error {
	if _, err := os.Stat(filepath.Join(profile, "cert9.db")); err != nil {
		return errors.New("Firefox NSS database is missing; start Firefox, then re-run")
	}
	if firefoxRootValid(ctx, query, root, profile) == nil {
		return nil
	}
	nickname := mkcertNSSName(root)
	_, _ = runner.Run(ctx, "", "certutil", "-D", "-d", "sql:"+profile, "-n", nickname)
	if _, err := runner.Run(ctx, "", "certutil", "-A", "-d", "sql:"+profile, "-n", nickname, "-t", "C,,", "-i", rootPath); err != nil {
		return fmt.Errorf("install mkcert root in Firefox: %w", err)
	}
	if err := firefoxRootValid(ctx, query, root, profile); err != nil {
		return fmt.Errorf("verify mkcert root in Firefox: %w", err)
	}
	return nil
}

func firefoxRootValid(ctx context.Context, runner execx.Runner, root *x509.Certificate, profile string) error {
	if _, err := os.Stat(filepath.Join(profile, "cert9.db")); err != nil {
		return fmt.Errorf("Firefox NSS database: %w", err)
	}
	nickname := mkcertNSSName(root)
	installed, err := runner.Output(ctx, "", "certutil", "-L", "-d", "sql:"+profile, "-n", nickname, "-a")
	if err != nil {
		return fmt.Errorf("read Firefox CA %q: %w", nickname, err)
	}
	cert, err := parseCertificate([]byte(installed))
	if err != nil {
		return fmt.Errorf("parse Firefox CA %q: %w", nickname, err)
	}
	if !bytes.Equal(cert.Raw, root.Raw) {
		return fmt.Errorf("Firefox CA %q differs from the current mkcert root", nickname)
	}
	if _, err := runner.Run(ctx, "", "certutil", "-V", "-d", "sql:"+profile, "-n", nickname, "-u", "L"); err != nil {
		return fmt.Errorf("validate Firefox CA %q: %w", nickname, err)
	}
	return nil
}

func mkcertNSSName(root *x509.Certificate) string {
	return "mkcert development CA " + root.SerialNumber.String()
}

func generateDevelopmentCert(ctx context.Context, runner execx.Runner, files devCertFiles, root *x509.Certificate) error {
	tmp, err := os.MkdirTemp(files.dir, ".mkcert-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	tmpCert := filepath.Join(tmp, "localhost.pem")
	tmpKey := filepath.Join(tmp, "localhost-key.pem")
	args := []string{"-cert-file", tmpCert, "-key-file", tmpKey}
	args = append(args, developmentCertNames[:]...)
	if _, err := runner.Run(ctx, "", "mkcert", args...); err != nil {
		return fmt.Errorf("generate local development certificate: %w", err)
	}
	if err := validateDevelopmentCert(tmpCert, tmpKey, root, time.Now()); err != nil {
		return fmt.Errorf("mkcert generated an invalid local development certificate: %w", err)
	}
	if err := os.Chmod(tmpCert, 0o644); err != nil {
		return err
	}
	if err := os.Chmod(tmpKey, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpKey, files.key); err != nil {
		return err
	}
	return os.Rename(tmpCert, files.cert)
}

func validateDevelopmentCert(certPath, keyPath string, root *x509.Certificate, now time.Time) error {
	if err := regularFile(certPath); err != nil {
		return err
	}
	if err := regularFile(keyPath); err != nil {
		return err
	}
	cert, err := readCertificate(certPath)
	if err != nil {
		return fmt.Errorf("read certificate: %w", err)
	}
	key, err := readPrivateKey(keyPath)
	if err != nil {
		return fmt.Errorf("read private key: %w", err)
	}
	certKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return err
	}
	privateKey, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return err
	}
	if !bytes.Equal(certKey, privateKey) {
		return errors.New("certificate and private key do not match")
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "localhost", CurrentTime: now}); err != nil {
		return fmt.Errorf("verify certificate chain: %w", err)
	}
	for _, name := range developmentCertNames {
		if err := cert.VerifyHostname(name); err != nil {
			return fmt.Errorf("verify SAN %s: %w", name, err)
		}
	}
	if !cert.NotAfter.After(now.Add(devCertRenewal)) {
		return fmt.Errorf("certificate expires within %s", devCertRenewal)
	}
	return nil
}

func regularFile(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		return fmt.Errorf("not a local regular file: %s", path)
	}
	return nil
}

func readCertificate(path string) (*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCertificate(b)
}

func parseCertificate(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("certificate PEM block not found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func readPrivateKey(path string) (crypto.Signer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("private key PEM block not found")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := key.(crypto.Signer); ok {
			return signer, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("unsupported private key encoding")
}

func systemTrustsRoot(root *x509.Certificate) error {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("load system trust store: %w", err)
	}
	_, err = root.Verify(x509.VerifyOptions{Roots: roots, CurrentTime: time.Now(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	if err != nil {
		return fmt.Errorf("current mkcert root is absent from system trust: %w", err)
	}
	return nil
}
