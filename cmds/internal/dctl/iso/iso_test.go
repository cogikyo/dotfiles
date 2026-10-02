package iso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/packages"
)

func TestClosure(t *testing.T) {
	pkgs, err := parseResolved("glibc glibc-2.42-1-x86_64.pkg.tar.zst\neww eww-0.6-1-x86_64.pkg.tar.zst\n")
	if err != nil {
		t.Fatal(err)
	}
	if pkgs[1] != (resolved{"eww", "eww-0.6-1-x86_64.pkg.tar.zst"}) {
		t.Fatalf("parsed %+v", pkgs[1])
	}
	if got := unresolved([]string{"eww", "glibc", "jack"}, pkgs); !slices.Equal(got, []string{"jack"}) {
		t.Fatalf("unresolved = %v", got)
	}
	if _, err := parseResolved("error: target not found: foo\n"); err == nil {
		t.Fatal("accepted a non-closure line")
	}
}

func TestTargets(t *testing.T) {
	file := filepath.Join(t.TempDir(), "targets")
	names := []string{"base", "eww", "linux"}
	if err := writeTargets(file, names); err != nil {
		t.Fatal(err)
	}
	got, err := packages.Read(file)
	if err != nil || !slices.Equal(got, names) {
		t.Fatalf("read back %v, %v", got, err)
	}
}

func TestSigningKey(t *testing.T) {
	home := t.TempDir()
	ssh := filepath.Join(home, ".ssh")
	if err := os.Mkdir(ssh, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := signingKey(home, ""); err == nil || !strings.Contains(err.Error(), "dctl keys enroll") {
		t.Fatalf("no keys: %v", err)
	}
	for _, f := range []string{"id_ed25519_sk_rk_a", "id_ed25519_sk_rk_a.pub", "id_ed25519"} {
		if err := os.WriteFile(filepath.Join(ssh, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if key, err := signingKey(home, ""); err != nil || key != filepath.Join(ssh, "id_ed25519_sk_rk_a") {
		t.Fatalf("one key: %q, %v", key, err)
	}
	if err := os.WriteFile(filepath.Join(ssh, "id_ed25519_sk_rk_b"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := signingKey(home, ""); err == nil || !strings.Contains(err.Error(), "--key") {
		t.Fatalf("two keys: %v", err)
	}
	if key, _ := signingKey(home, "~/.ssh/id_ed25519_sk_rk_b"); key != filepath.Join(ssh, "id_ed25519_sk_rk_b") {
		t.Fatalf("flag: %q", key)
	}
}

func TestCopySigned(t *testing.T) {
	var dev bytes.Buffer
	sum := sha256.Sum256([]byte("iso"))
	want := hex.EncodeToString(sum[:])
	if err := copySigned(&dev, strings.NewReader("iso"), "a.iso", want); err != nil || dev.String() != "iso" {
		t.Fatalf("good copy: %v %q", err, dev.String())
	}
	if err := copySigned(io.Discard, strings.NewReader("evil"), "a.iso", want); err == nil || !strings.Contains(err.Error(), "a.iso does not match") {
		t.Fatalf("mismatch passed: %v", err)
	}
}

func TestPayloadSums(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{"a-1-any.pkg.tar.zst": "a", "b-1-any.pkg.tar.zst": "b", "dctl.db.tar.zst": "db"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeSums(t.Context(), dir); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPayload(t.Context(), dir); err != nil {
		t.Fatalf("fresh payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b-1-any.pkg.tar.zst"), []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPayload(t.Context(), dir); err == nil || !strings.Contains(err.Error(), "b-1-any.pkg.tar.zst does not match") || strings.Contains(err.Error(), "a-1") {
		t.Fatalf("corrupt payload: %v", err)
	}
	h := strings.Repeat("ab", sha256.Size)
	got, err := parseSums([]byte(h + "  a b.pkg\n" + h + " *c.pkg\n"))
	if err != nil || !slices.Equal(got, []sum{{h, "a b.pkg"}, {h, "c.pkg"}}) {
		t.Fatalf("text and binary lines: %v, %v", got, err)
	}
	for _, bad := range []string{"abc  a\n", h + "  \n", h + " -a\n", h + "a  b\n"} {
		if _, err := parseSums([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestOversize(t *testing.T) {
	if err := oversize(MaxSize, nil); err != nil {
		t.Fatal(err)
	}
	var pkgs []sized
	for i := range 25 {
		pkgs = append(pkgs, sized{string(rune('a' + i)), int64(i) << 20})
	}
	err := oversize(MaxSize+1, pkgs)
	if err == nil {
		t.Fatal("2 GiB + 1 passed")
	}
	msg := err.Error()
	if n := strings.Count(msg, "\n"); n != 20 {
		t.Fatalf("listed %d packages, want 20", n)
	}
	if !strings.Contains(msg, "24.0 MiB  y") || strings.Contains(msg, "  e\n") {
		t.Fatalf("not the largest 20:\n%s", msg)
	}
}

type git map[string]string

func (g git) Run(ctx context.Context, dir, name string, args ...string) error {
	_, err := g.Output(ctx, dir, name, args...)
	return err
}

func (g git) Output(_ context.Context, _, _ string, args ...string) (string, error) {
	for _, a := range args {
		if v, ok := g[a]; ok {
			return v, nil
		}
	}
	return "", nil
}

func TestRevision(t *testing.T) {
	for name, tc := range map[string]struct {
		repo git
		err  string
	}{
		"branch": {git{"--short": "feature"}, "branch"},
		"dirty":  {git{"--short": "master", "--porcelain": "?? x"}, "dirty"},
		"clean":  {git{"--short": "master", "--porcelain": "", "rev-parse": "0123456789abcdef0123456789abcdef01234567"}, ""},
	} {
		rev, err := revision(t.Context(), tc.repo, "/repo", nil)
		if tc.err == "" {
			if err != nil || rev != tc.repo["rev-parse"] {
				t.Fatalf("%s: %v", name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.err) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func TestStickRefusal(t *testing.T) {
	usb := stick{Path: "/dev/sdb", Type: "disk", Size: 8 << 30, Tran: "usb", Serial: "4C53"}
	mounted := usb
	mounted.Children = []stick{{Path: "/dev/sdb1", Type: "part", Mountpoints: []string{"/run/media/x"}}}
	for _, tc := range []struct {
		dev  stick
		want string
	}{
		{usb, ""},
		{stick{Path: "/dev/sdb1", Type: "part", Tran: "usb", Size: 8 << 30}, "whole disk"},
		{stick{Path: "/dev/nvme0n1", Type: "disk", Size: 1 << 40, Tran: "nvme", Serial: "S1"}, "not removable"},
		{mounted, "mounted"},
		{stick{Path: "/dev/sdc", Type: "disk", RM: true, Size: 1 << 30}, "smaller"},
		{stick{Path: "/dev/sdd", Type: "disk", RM: true, RO: true, Size: 8 << 30}, "read-only"},
		{stick{Path: "/dev/sde", Type: "disk", RM: true, Size: 8 << 30}, "no serial"},
	} {
		got := tc.dev.refusal(2 << 30)
		if tc.want == "" && got != "" || !strings.Contains(got, tc.want) {
			t.Fatalf("%s: refusal %q, want %q", tc.dev.Path, got, tc.want)
		}
	}
}

func TestValidPGPKeys(t *testing.T) {
	recipe := t.TempDir()
	if fprs, err := validpgpkeys(recipe); err != nil || fprs != nil {
		t.Fatalf("no .SRCINFO: %v, %v", fprs, err)
	}
	srcinfo := "pkgbase = s\n\tvalidpgpkeys = 948F158A4E76A27BF3D07532DF42C170B34DBA77\n\tvalidpgpkeys = 2B4A53F4F4C6B3E5BBA2C1E1E1F8C1A1B1C1D1E1\n\tbackup = etc/pgp.conf\n"
	if err := os.WriteFile(filepath.Join(recipe, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{"948F158A4E76A27BF3D07532DF42C170B34DBA77", "2B4A53F4F4C6B3E5BBA2C1E1E1F8C1A1B1C1D1E1"}
	if fprs, err := validpgpkeys(recipe); err != nil || !slices.Equal(fprs, want) {
		t.Fatalf("got %v, %v", fprs, err)
	}
}

func TestPacmanConf(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "iso", "airootfs", "etc", "pacman.conf"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(data), "\n["+Repo+"]\n")
	section, _, _ = strings.Cut(section, "\n[")
	if !ok || !strings.Contains(section, "\nServer = file://"+Payload+"\n") {
		t.Fatalf("pacman.conf has no [%s] section with Server = file://%s", Repo, Payload)
	}
}
