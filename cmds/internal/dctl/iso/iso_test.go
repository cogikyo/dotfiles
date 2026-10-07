package iso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/packages"
)

func TestClosure(t *testing.T) {
	pkgs, err := parseResolved("core glibc glibc-2.42-1-x86_64.pkg.tar.zst\ndctl eww eww-0.6-1-x86_64.pkg.tar.zst\n")
	if err != nil {
		t.Fatal(err)
	}
	if pkgs[1] != (resolved{Repo, "eww", "eww-0.6-1-x86_64.pkg.tar.zst"}) {
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

func TestRecipeCache(t *testing.T) {
	store := t.TempDir()
	entry := filepath.Join(store, "eww", "new")
	for _, dir := range []string{entry, filepath.Join(store, "eww", "old"), filepath.Join(store, "gone", "k")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cached(t.Context(), entry); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("entry without %s: %v", Sums, err)
	}
	pkg := filepath.Join(entry, "eww-1-1-x86_64.pkg.tar.zst")
	if err := os.WriteFile(pkg, []byte("eww"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSums(t.Context(), entry); err != nil {
		t.Fatal(err)
	}
	if files, err := cached(t.Context(), entry); err != nil || !slices.Equal(files, []string{pkg}) {
		t.Fatalf("complete entry: %v, %v", files, err)
	}
	if err := os.WriteFile(pkg, []byte("EWW"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cached(t.Context(), entry); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("corrupt entry: %v", err)
	}
	if err := prune(store, map[string]string{"eww": "new"}); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(store, "*", "*"))
	if !slices.Equal(left, []string{entry}) {
		t.Fatalf("after prune: %v", left)
	}

	r := srcinfo("pkgbase = s\n\tdepends = gtk3>=3.24\n\tmakedepends = rust\npkgname = s\n\tprovides = s-bin=1\npkgname = s-cli\n\tdepends = glibc\n\tdepends_x86_64 = sh\n")
	if !slices.Equal(r.pkgnames, []string{"s", "s-cli"}) || !slices.Equal(r.provides, []string{"s-bin"}) || !slices.Equal(r.depends, []string{"glibc", "gtk3>=3.24", "sh"}) {
		t.Fatalf("srcinfo = %+v", r)
	}
	info := "installed = gtk3-1:3.24.52-1-x86_64\ninstalled = glibc-2.42-1-x86_64\n"
	if !slices.Equal(r.global, []string{"gtk3>=3.24"}) {
		t.Fatalf("global depends = %v", r.global)
	}
	if err := drift(map[string]pin{"gtk3": {"1:3.24.52-1", true}, "glibc": {"2.42-1", true}, "sh-only": {"1-1", false}}, info); err != nil {
		t.Fatalf("matching chroot: %v", err)
	}
	for _, p := range []map[string]pin{{"gtk3": {"1:3.24.53-1", false}}, {"jq": {"1.8-1", true}}} {
		if err := drift(p, info); err == nil {
			t.Fatalf("drifted chroot %v passed", p)
		}
	}
	key := fingerprint("abc", []string{"repo gtk3 1:3.24.52-1", "repo glibc 2.42-1"})
	if key != fingerprint("abc", []string{"repo glibc 2.42-1", "repo gtk3 1:3.24.52-1"}) {
		t.Fatal("key depends on dependency order")
	}
	if key == fingerprint("abc", []string{"repo gtk3 1:3.24.53-1", "repo glibc 2.42-1"}) {
		t.Fatal("a dependency version change kept the key")
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
