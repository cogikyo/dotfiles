package secrets

// secrets_test.go covers manifest validation, trust tracking, and encrypted file writes.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestReadManifestParsesAndValidatesEntries(t *testing.T) {
	home := t.TempDir()
	manifest := filepath.Join(t.TempDir(), "manifest")
	data := "# comment\nssh:~/.ssh/id_ed25519:600\nnetrc:~/netrc:0600\n\n"
	if err := os.WriteFile(manifest, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := readManifest(manifest, home)
	if err != nil {
		t.Fatalf("readManifest() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Name != "ssh" || entries[0].Target != "~/.ssh/id_ed25519" || entries[0].Mode != 0o600 {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[0].Raw != "ssh:~/.ssh/id_ed25519:600" {
		t.Fatalf("raw entry = %q", entries[0].Raw)
	}
}

func TestReadManifestRejectsInvalidEntries(t *testing.T) {
	home := t.TempDir()
	tests := map[string]string{
		"bad name":        "bad/name:~/secret:600",
		"bad mode":        "name:~/secret:999",
		"too many fields": "name:~/secret:600:extra",
		"outside home":    "name:/etc/passwd:600",
		"duplicate name":  "name:~/one:600\nname:~/two:600",
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			manifest := filepath.Join(t.TempDir(), "manifest")
			if err := os.WriteFile(manifest, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readManifest(manifest, home); err == nil {
				t.Fatal("readManifest() error = nil, want error")
			}
		})
	}
}

func TestTrustUsesExactManifestEntries(t *testing.T) {
	trustPath := filepath.Join(t.TempDir(), "trust")
	entries := []entry{
		{Name: "secret", Target: "~/one", Mode: 0o600, Raw: "secret:~/one:600"},
	}
	if err := trustEntries(trustPath, entries); err != nil {
		t.Fatal(err)
	}

	changed := []entry{{Name: "secret", Target: "~/one", Mode: 0o644, Raw: "secret:~/one:644"}}
	untrusted, err := untrustedEntries(trustPath, changed)
	if err != nil {
		t.Fatal(err)
	}
	if len(untrusted) != 1 {
		t.Fatalf("len(untrusted) = %d, want 1", len(untrusted))
	}

	untrusted, err = untrustedEntries(trustPath, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(untrusted) != 0 {
		t.Fatalf("len(untrusted) = %d, want 0", len(untrusted))
	}
}

func TestSafeTargetRequiresHomeBoundary(t *testing.T) {
	home := t.TempDir()
	if got, err := safeTarget(home, "~/secret"); err != nil || got != filepath.Join(home, "secret") {
		t.Fatalf("safeTarget(home-relative) = %q, %v", got, err)
	}
	if _, err := safeTarget(home, filepath.Join(home, "..", "outside")); err == nil {
		t.Fatal("safeTarget(.. outside) error = nil, want error")
	}
}

func TestSafeTargetRejectsSymlinkedParentOutsideHome(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(home, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := safeTarget(home, "~/link/secret"); err == nil {
		t.Fatal("safeTarget(symlinked parent outside home) error = nil, want error")
	}
}

func TestAgeRoundTripWithoutPlaintextIdentityFile(t *testing.T) {
	dir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("secret payload")
	ciphertext := filepath.Join(dir, "payload.age")
	source := filepath.Join(dir, "payload")
	if err := os.WriteFile(source, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := encryptFile(ciphertext, source, identity.Recipient()); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := decryptTo(ciphertext, &out, identity); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), plain) {
		t.Fatalf("decrypted = %q, want %q", out.Bytes(), plain)
	}
}

func TestEncryptedIdentityRoundTripInMemory(t *testing.T) {
	dir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := age.NewScryptRecipient("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(identity.String() + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(dir, "identity.age")
	if err := os.WriteFile(identityPath, encrypted.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := decryptIdentity(identityPath, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != identity.String() {
		t.Fatalf("identity = %q, want %q", got.String(), identity.String())
	}
}
