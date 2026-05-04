// Package secrets manages age-encrypted files declared by etc/secrets/manifest.
//
// Responsibilities:
// - Create and unlock the local age identity.
// - Encrypt manifest targets into the repo and decrypt them back into HOME.
// - Require per-machine trust for new or changed manifest entries.
package secrets

// secrets.go defines the secrets CLI, manifest format, trust store, and safe file writes.

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"dotfiles/dctl/internal/app"
	"dotfiles/dctl/internal/prompt"

	"filippo.io/age"
)

const (
	manifestName  = "manifest"
	identityName  = "identity.age"
	recipientName = "recipient.txt"
	trustName     = "secrets-manifest.approved"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Cmd struct {
	Init    InitCmd    `cmd:"" help:"Create an age identity."`
	Sync    SyncCmd    `cmd:"" help:"Encrypt manifest targets into the repo."`
	Decrypt DecryptCmd `cmd:"" help:"Decrypt secrets to target paths."`
	Trust   TrustCmd   `cmd:"" help:"Trust current manifest entries on this machine."`
	List    ListCmd    `cmd:"" help:"List manifest entries."`
}
type InitCmd struct{}
type SyncCmd struct {
	Force bool `short:"f" help:"Re-encrypt even if ciphertext is newer."`
}
type DecryptCmd struct {
	DryRun bool `short:"n" help:"Verify decryptability without writing files."`
}
type TrustCmd struct{}
type ListCmd struct{}

type entry struct {
	Name   string
	Target string
	Mode   os.FileMode
	Raw    string
}

func (c *InitCmd) Run(ctx *app.Context) error {
	paths := newPaths(ctx)
	if !prompt.Interactive() {
		return fmt.Errorf("secrets init requires an interactive terminal")
	}
	if err := os.MkdirAll(paths.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(paths.dir, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(paths.identity); err == nil {
		if !ctx.Yes {
			ok, err := prompt.Confirm("Regenerate secrets identity? This requires re-syncing all secrets", false)
			if err != nil {
				return err
			}
			if !ok {
				ctx.Output.Warn("Identity unchanged")
				return nil
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	passphrase, err := readNewPassphrase()
	if err != nil {
		return err
	}

	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return err
	}
	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, identity.String()+"\n"); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	if err := writeFileAtomic(paths.identity, encrypted.Bytes(), 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(paths.recipient, []byte(identity.Recipient().String()+"\n"), 0o644); err != nil {
		_ = os.Remove(paths.identity)
		return err
	}
	ctx.Output.OK("Identity created")
	ctx.Output.KV("recipient", identity.Recipient().String())
	return nil
}

func (c *SyncCmd) Run(ctx *app.Context) error {
	paths := newPaths(ctx)
	entries, err := readManifest(paths.manifest, ctx.Root.Home)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		ctx.Output.Warn("Manifest is empty")
		return nil
	}
	if err := reviewTrust(ctx, paths.trust, entries, "sync"); err != nil {
		return err
	}
	recipient, err := readRecipient(paths.recipient)
	if err != nil {
		return err
	}

	synced, skipped, missing := 0, 0, 0
	for _, e := range entries {
		target, err := safeTarget(ctx.Root.Home, e.Target)
		if err != nil {
			return err
		}
		ciphertext := filepath.Join(paths.dir, e.Name+".age")
		if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
			ctx.Output.Warn("Not found: %s (skipping %s)", e.Target, e.Name)
			missing++
			continue
		} else if err != nil {
			return err
		}
		if !c.Force && newerThan(ciphertext, target) {
			skipped++
			continue
		}
		if err := encryptFile(ciphertext, target, recipient); err != nil {
			return err
		}
		ctx.Output.Step("Encrypted %s <- %s", e.Name, e.Target)
		synced++
	}
	if missing > 0 {
		ctx.Output.Warn("Synced %d, unchanged %d, missing %d", synced, skipped, missing)
		return nil
	}
	ctx.Output.OK("Synced %d, unchanged %d", synced, skipped)
	return nil
}

func (c *DecryptCmd) Run(ctx *app.Context) error {
	paths := newPaths(ctx)
	entries, err := readManifest(paths.manifest, ctx.Root.Home)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		ctx.Output.Warn("Manifest is empty")
		return nil
	}
	if err := reviewTrust(ctx, paths.trust, entries, "decrypt"); err != nil {
		return err
	}
	identity, err := unlockIdentity(paths.identity)
	if err != nil {
		return err
	}

	count, failed := 0, 0
	for _, e := range entries {
		target, err := safeTarget(ctx.Root.Home, e.Target)
		if err != nil {
			return err
		}
		ciphertext := filepath.Join(paths.dir, e.Name+".age")
		if _, err := os.Stat(ciphertext); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				ctx.Output.Error("Missing: secrets/%s.age", e.Name)
				failed++
				continue
			}
			return err
		}
		if c.DryRun {
			if err := decryptTo(ciphertext, io.Discard, identity); err != nil {
				ctx.Output.Error("Failed to decrypt %s", e.Name)
				failed++
				continue
			}
			count++
			continue
		}
		if err := decryptFileAtomic(ciphertext, target, e.Mode, identity); err != nil {
			ctx.Output.Error("Failed to decrypt %s: %v", e.Name, err)
			failed++
			continue
		}
		ctx.Output.Step("Decrypted %s -> %s", e.Name, e.Target)
		count++
	}
	if failed > 0 {
		return fmt.Errorf("decrypted %d, failed %d", count, failed)
	}
	if c.DryRun {
		ctx.Output.OK("Dry-run verified %d secrets", count)
	} else {
		ctx.Output.OK("Decrypted %d secrets", count)
	}
	return nil
}

func (c *TrustCmd) Run(ctx *app.Context) error {
	paths := newPaths(ctx)
	entries, err := readManifest(paths.manifest, ctx.Root.Home)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		ctx.Output.Warn("Manifest is empty")
		return nil
	}
	if err := trustEntries(paths.trust, entries); err != nil {
		return err
	}
	ctx.Output.OK("Trusted %d manifest entries for this machine", len(entries))
	return nil
}

func (c *ListCmd) Run(ctx *app.Context) error {
	paths := newPaths(ctx)
	entries, err := readManifest(paths.manifest, ctx.Root.Home)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		ctx.Output.Warn("Manifest is empty")
		return nil
	}
	if ctx.Output.JSONMode() {
		return ctx.Output.Emit(entries)
	}
	fmt.Fprintf(ctx.Output.Writer(), "%-20s %-40s %s\n", "NAME", "TARGET", "MODE")
	fmt.Fprintf(ctx.Output.Writer(), "%-20s %-40s %s\n", "----", "------", "----")
	for _, e := range entries {
		marker := "  "
		if _, err := os.Stat(filepath.Join(paths.dir, e.Name+".age")); errors.Is(err, os.ErrNotExist) {
			marker = "!!"
		}
		fmt.Fprintf(ctx.Output.Writer(), "%s %-18s %-40s %04o\n", marker, e.Name, e.Target, e.Mode.Perm())
	}
	return nil
}

type pathSet struct {
	dir       string
	manifest  string
	identity  string
	recipient string
	trust     string
}

func newPaths(ctx *app.Context) pathSet {
	dir := ctx.Root.Etc("secrets")
	return pathSet{
		dir:       dir,
		manifest:  filepath.Join(dir, manifestName),
		identity:  filepath.Join(dir, identityName),
		recipient: filepath.Join(dir, recipientName),
		trust:     filepath.Join(ctx.Root.State, trustName),
	}
}

func readManifest(path string, home string) ([]entry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []entry
	seen := map[string]struct{}{}
	s := bufio.NewScanner(f)
	for lineNo := 1; s.Scan(); lineNo++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		e, err := parseManifestLine(line, home)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		if _, ok := seen[e.Name]; ok {
			return nil, fmt.Errorf("%s:%d: duplicate secret name %q", path, lineNo, e.Name)
		}
		seen[e.Name] = struct{}{}
		entries = append(entries, e)
	}
	return entries, s.Err()
}

// parseManifestLine accepts `name:target:mode` entries.
//
// Targets must resolve inside HOME and modes are octal file permissions.
func parseManifestLine(line string, home string) (entry, error) {
	parts := strings.Split(line, ":")
	if len(parts) != 3 {
		return entry{}, fmt.Errorf("invalid manifest entry: expected name:target:mode")
	}
	name, target, modeText := parts[0], parts[1], parts[2]
	if name == "" || target == "" || modeText == "" {
		return entry{}, fmt.Errorf("invalid manifest entry: expected name:target:mode")
	}
	if !nameRE.MatchString(name) {
		return entry{}, fmt.Errorf("invalid secret name %q", name)
	}
	if len(modeText) != 3 && len(modeText) != 4 {
		return entry{}, fmt.Errorf("invalid mode %q for %q", modeText, name)
	}
	mode64, err := strconv.ParseUint(modeText, 8, 32)
	if err != nil || mode64 > 0o7777 {
		return entry{}, fmt.Errorf("invalid mode %q for %q", modeText, name)
	}
	if _, err := safeTarget(home, target); err != nil {
		return entry{}, fmt.Errorf("invalid target for %q: %w", name, err)
	}
	return entry{Name: name, Target: target, Mode: os.FileMode(mode64), Raw: name + ":" + target + ":" + modeText}, nil
}

// safeTarget expands a manifest target and rejects paths that escape HOME.
//
// Existing parent directories are resolved through symlinks before approval.
func safeTarget(home string, target string) (string, error) {
	expanded := target
	if target == "~" {
		expanded = home
	} else if strings.HasPrefix(target, "~/") {
		expanded = filepath.Join(home, target[2:])
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	homeAbs = filepath.Clean(homeAbs)
	if !pathWithin(abs, homeAbs) {
		return "", fmt.Errorf("target is outside HOME: %s", target)
	}
	if err := existingParentWithinHome(abs, homeAbs); err != nil {
		return "", err
	}
	return abs, nil
}

func pathWithin(path string, root string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func existingParentWithinHome(path string, home string) error {
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return err
	}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if st, err := os.Stat(dir); err == nil {
			if !st.IsDir() {
				return fmt.Errorf("target parent is not a directory: %s", dir)
			}
			realDir, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return err
			}
			if !pathWithin(filepath.Clean(realDir), filepath.Clean(realHome)) {
				return fmt.Errorf("target parent resolves outside HOME: %s", dir)
			}
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if dir == filepath.Dir(dir) {
			return fmt.Errorf("no existing target parent for %s", path)
		}
	}
}

func readRecipient(path string) (age.Recipient, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no recipient key: run dctl secrets init")
	}
	if err != nil {
		return nil, err
	}
	return age.ParseX25519Recipient(strings.TrimSpace(string(data)))
}

func readNewPassphrase() (string, error) {
	passphrase, err := prompt.Hidden("Passphrase: ")
	if err != nil {
		return "", err
	}
	if passphrase == "" {
		return "", fmt.Errorf("passphrase cannot be empty")
	}
	confirm, err := prompt.Hidden("Confirm passphrase: ")
	if err != nil {
		return "", err
	}
	if passphrase != confirm {
		return "", fmt.Errorf("passphrases do not match")
	}
	return passphrase, nil
}

func unlockIdentity(path string) (*age.X25519Identity, error) {
	if !prompt.Interactive() {
		return nil, fmt.Errorf("unlocking identity requires an interactive terminal")
	}
	for {
		passphrase, err := prompt.Hidden("Identity passphrase: ")
		if err != nil {
			return nil, err
		}
		identity, err := decryptIdentity(path, passphrase)
		if err == nil {
			return identity, nil
		}
		if !prompt.Interactive() {
			return nil, fmt.Errorf("failed to unlock identity in non-interactive mode")
		}
		fmt.Fprintln(os.Stderr, "Incorrect passphrase, try again")
	}
}

func decryptIdentity(path string, passphrase string) (*age.X25519Identity, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no identity file: run dctl secrets init")
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scryptIdentity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(f, scryptIdentity)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	identity, err := age.ParseX25519Identity(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("decrypted identity is invalid: %w", err)
	}
	return identity, nil
}

func encryptFile(dst string, src string, recipient age.Recipient) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return writeFileAtomic(dst, encrypted.Bytes(), 0o600)
}

func decryptTo(src string, dst io.Writer, identity age.Identity) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	r, err := age.Decrypt(in, identity)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, r)
	return err
}

// decryptFileAtomic writes decrypted content through a same-directory temp file.
//
// Changed existing files are copied to a timestamped backup before replacement.
func decryptFileAtomic(src string, dst string, mode os.FileMode, identity age.Identity) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := decryptTo(src, tmp, identity); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if old, err := os.ReadFile(dst); err == nil {
		newData, err := os.ReadFile(tmpName)
		if err != nil {
			return err
		}
		if !bytes.Equal(old, newData) {
			backup := dst + ".bak." + time.Now().Format("20060102-150405.000000000")
			if err := copyExistingSecret(dst, backup, mode.Perm()); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	return nil
}

func copyExistingSecret(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func newerThan(a string, b string) bool {
	aInfo, err := os.Stat(a)
	if err != nil {
		return false
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false
	}
	return aInfo.ModTime().After(bInfo.ModTime())
}

// reviewTrust blocks sync and decrypt until new manifest entries are approved on this machine.
func reviewTrust(ctx *app.Context, trustPath string, entries []entry, action string) error {
	untrusted, err := untrustedEntries(trustPath, entries)
	if err != nil {
		return err
	}
	if len(untrusted) == 0 {
		return nil
	}
	ctx.Output.Warn("Found %d new or changed manifest entries", len(untrusted))
	for _, e := range untrusted {
		ctx.Output.KV(e.Name, e.Target+" "+fmt.Sprintf("%04o", e.Mode.Perm()))
	}
	if !prompt.Interactive() {
		return fmt.Errorf("non-interactive session cannot approve new manifest entries; run dctl secrets trust")
	}
	if !ctx.Yes {
		ok, err := prompt.Confirm("Trust these entries on this machine and continue "+action+"?", false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted due to untrusted manifest entries")
		}
	}
	return trustEntries(trustPath, untrusted)
}

func untrustedEntries(trustPath string, entries []entry) ([]entry, error) {
	trusted, err := trustedSet(trustPath)
	if err != nil {
		return nil, err
	}
	var out []entry
	for _, e := range entries {
		if _, ok := trusted[e.Raw]; !ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func trustedSet(path string) (map[string]struct{}, error) {
	trusted := map[string]struct{}{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return trusted, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line != "" {
			trusted[line] = struct{}{}
		}
	}
	return trusted, s.Err()
}

func trustEntries(path string, entries []entry) error {
	trusted, err := trustedSet(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		trusted[e.Raw] = struct{}{}
	}
	lines := make([]string, 0, len(trusted))
	for line := range trusted {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	data := []byte(strings.Join(lines, "\n"))
	if len(data) > 0 {
		data = append(data, '\n')
	}
	return writeFileAtomic(path, data, 0o600)
}
