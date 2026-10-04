package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"

	"filippo.io/age"
	"filippo.io/age/plugin"
)

const phrase = "throwaway test phrase"

type fixture struct {
	root    paths.Root
	u       *ui.UI
	wrapped *age.X25519Identity
	other   *age.X25519Identity
}

func newFixture(t *testing.T, manifest string) fixture {
	t.Helper()
	dir := t.TempDir()
	f := fixture{
		root: paths.Root{Dotfiles: filepath.Join(dir, "dotfiles"), Home: filepath.Join(dir, "home")},
		u:    ui.New(ui.Options{JSON: true, Yes: true}),
	}
	must(t, os.MkdirAll(f.root.Secrets(), 0o755))
	must(t, os.MkdirAll(f.root.Home, 0o700))
	f.wrapped, f.other = identity(t), identity(t)

	scrypt, err := age.NewScryptRecipient(phrase)
	must(t, err)
	scrypt.SetWorkFactor(10)
	keygen := "# created: 2026-02-15T00:00:00Z\n# public key: " + f.wrapped.Recipient().String() + "\n" + f.wrapped.String() + "\n"
	wrapped, err := Seal([]byte(keygen), []age.Recipient{scrypt})
	must(t, err)
	must(t, os.WriteFile(f.root.Secrets("identity.age"), wrapped, 0o644))
	recipients := "# paper\n" + f.wrapped.Recipient().String() + "\n\n" + f.other.Recipient().String() + "\n"
	must(t, os.WriteFile(f.root.Secrets("recipients"), []byte(recipients), 0o644))
	must(t, os.WriteFile(f.root.Secrets("manifest"), []byte(manifest), 0o644))
	return f
}

func (f fixture) keys(ids ...age.Identity) *Keys { return &Keys{u: f.u, ids: ids} }

func (f fixture) recipients(t *testing.T) []age.Recipient {
	t.Helper()
	r, err := Recipients(f.u, f.root)
	must(t, err)
	return r
}

func identity(t *testing.T) *age.X25519Identity {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	must(t, err)
	return id
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	must(t, err)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestManifest(t *testing.T) {
	got, err := parseManifest(strings.NewReader("# c\n\nkey:~/.ssh/key:600\nvpn:~/.local/vpn/work.nmconnection:0640:staged\n"))
	must(t, err)
	want := []Entry{
		{Name: "key", Target: ".ssh/key", Mode: 0o600},
		{Name: "vpn", Target: ".local/vpn/work.nmconnection", Mode: 0o640, Staged: true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestManifestErrors(t *testing.T) {
	for _, text := range []string{
		"",
		"# only comments\n",
		"key:~/.ssh/key\n",
		"key:~/.ssh/key:600:staged:extra\n",
		"bad/name:~/.ssh/key:600\n",
		"key:/etc/passwd:600\n",
		"key:.ssh/key:600\n",
		"key:~/../escape:600\n",
		"key:~/.ssh/../key:600\n",
		"key:~/:600\n",
		"key:~/.ssh/key:999\n",
		"key:~/.ssh/key:60\n",
		"key:~/.ssh/key:4755\n",
		"key:~/.ssh/key:600:once\n",
		"key:~/.ssh/key:600\nkey:~/.ssh/other:600\n",
		"key:~/.ssh/key:600\nother:~/.ssh/key:600\n",
	} {
		if _, err := parseManifest(strings.NewReader(text)); err == nil {
			t.Errorf("parseManifest(%q) succeeded", text)
		}
	}
}

func TestWriteRefusesEscape(t *testing.T) {
	dir := t.TempDir()
	home, outside := filepath.Join(dir, "home"), filepath.Join(dir, "outside")
	must(t, os.MkdirAll(home, 0o700))
	must(t, os.MkdirAll(outside, 0o700))
	must(t, os.Symlink(outside, filepath.Join(home, ".ssh")))
	root := pin(t, home)

	if err := root.write(".ssh/key", []byte("secret"), 0o600); err == nil {
		t.Fatal("write through an escaping parent symlink succeeded")
	}
	if got := names(t, outside); len(got) != 0 {
		t.Fatalf("files written outside home: %v", got)
	}
}

func TestLeafSymlink(t *testing.T) {
	dir := t.TempDir()
	home, outside := filepath.Join(dir, "home"), filepath.Join(dir, "outside")
	must(t, os.MkdirAll(home, 0o700))
	must(t, os.WriteFile(outside, []byte("keep"), 0o644))
	inside := filepath.Join(home, "inside")
	must(t, os.WriteFile(inside, []byte("keep"), 0o644))
	must(t, os.Symlink(outside, filepath.Join(home, "out")))
	must(t, os.Symlink(inside, filepath.Join(home, "in")))
	root := pin(t, home)

	for _, rel := range []string{"out", "in"} {
		if err := root.write(rel, []byte("secret"), 0o600); !errors.Is(err, errIrregular) {
			t.Errorf("write %s: got %v, want errIrregular", rel, err)
		}
		if _, _, err := root.read(rel); !errors.Is(err, errIrregular) {
			t.Errorf("read %s: got %v, want errIrregular", rel, err)
		}
	}
	for _, p := range []string{outside, inside} {
		if data, _ := os.ReadFile(p); string(data) != "keep" {
			t.Errorf("%s changed to %q", p, data)
		}
	}
}

func TestWriteModes(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	home := t.TempDir()
	root := pin(t, home)
	must(t, os.WriteFile(filepath.Join(home, "loose"), []byte("old"), 0o666))
	must(t, os.Chmod(filepath.Join(home, "loose"), 0o666))

	for rel, mode := range map[string]os.FileMode{"group": 0o640, "public": 0o644, "loose": 0o600} {
		must(t, root.write(rel, []byte("data"), mode))
		st, err := os.Lstat(filepath.Join(home, rel))
		must(t, err)
		if st.Mode() != mode {
			t.Errorf("%s: mode %v, want %v", rel, st.Mode(), mode)
		}
	}
	if got, want := names(t, home), []string{"group", "loose", "public"}; !slices.Equal(got, want) {
		t.Fatalf("home holds %v, want %v", got, want)
	}
}

func TestRoundTripEveryRecipient(t *testing.T) {
	f := newFixture(t, "key:~/.ssh/key:600\nenv:~/.local/env:640\n")
	plain := map[string]string{".ssh/key": "private key", ".local/env": "TOKEN=x"}
	for rel, data := range plain {
		must(t, os.MkdirAll(filepath.Join(f.root.Home, filepath.Dir(rel)), 0o700))
		must(t, os.WriteFile(filepath.Join(f.root.Home, rel), []byte(data), 0o600))
	}
	must(t, sealTargets(f.u, f.root, f.keys(f.wrapped), f.recipients(t)))
	sealed, err := os.ReadFile(f.root.Secrets("key.age"))
	must(t, err)

	unwrapped, err := Unwrap(mustRead(t, f.root.Secrets("identity.age")), phrase)
	must(t, err)
	entries, err := Manifest(f.root)
	must(t, err)
	for _, id := range []age.Identity{unwrapped, f.other} {
		must(t, os.RemoveAll(f.root.Home))
		must(t, os.MkdirAll(f.root.Home, 0o700))
		must(t, writeTargets(f.u, f.root, f.keys(id), entries))
		for _, e := range entries {
			p := filepath.Join(f.root.Home, e.Target)
			if got := string(mustRead(t, p)); got != plain[e.Target] {
				t.Errorf("%s: got %q", e.Target, got)
			}
			st, err := os.Stat(p)
			must(t, err)
			if st.Mode().Perm() != e.Mode {
				t.Errorf("%s: mode %v, want %v", e.Target, st.Mode().Perm(), e.Mode)
			}
			if got := names(t, filepath.Dir(p)); !slices.Equal(got, []string{filepath.Base(p)}) {
				t.Errorf("%s: side files %v", filepath.Dir(p), got)
			}
		}
	}

	must(t, sealTargets(f.u, f.root, f.keys(f.other), f.recipients(t)))
	if !bytes.Equal(sealed, mustRead(t, f.root.Secrets("key.age"))) {
		t.Fatal("sync rewrote an unchanged secret")
	}
}

func TestFailedDecryptKeepsTarget(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	target := filepath.Join(f.root.Home, "key")
	foreign, err := Seal([]byte("new"), []age.Recipient{identity(t).Recipient()})
	must(t, err)
	good, err := Seal(bytes.Repeat([]byte("new"), 30000), f.recipients(t))
	must(t, err)
	corrupt := slices.Clone(good)
	corrupt[len(corrupt)-1] ^= 1
	entries, err := Manifest(f.root)
	must(t, err)

	for name, ciphertext := range map[string][]byte{"foreign": foreign, "corrupt": corrupt} {
		must(t, os.WriteFile(f.root.Secrets("key.age"), ciphertext, 0o644))
		must(t, os.WriteFile(target, []byte("old"), 0o600))
		if err := writeTargets(f.u, f.root, f.keys(f.wrapped), entries); err == nil {
			t.Fatalf("%s: decrypt succeeded", name)
		}
		if got := string(mustRead(t, target)); got != "old" {
			t.Errorf("%s: target changed to %q", name, got)
		}
		if got := names(t, f.root.Home); !slices.Equal(got, []string{"key"}) {
			t.Errorf("%s: side files %v", name, got)
		}
	}
}

func TestStagedOnlyWhenNamed(t *testing.T) {
	f := newFixture(t, "key:~/key:600\nvpn:~/vpn:600:staged\n")
	all, err := pick(f.root, nil)
	must(t, err)
	if len(all) != 1 || all[0].Name != "key" {
		t.Fatalf("default pick: %+v", all)
	}
	named, err := pick(f.root, []string{"vpn"})
	must(t, err)
	if len(named) != 1 || named[0].Name != "vpn" {
		t.Fatalf("named pick: %+v", named)
	}
	if _, err := pick(f.root, []string{"nope"}); err == nil {
		t.Fatal("unknown name accepted")
	}
	must(t, os.WriteFile(filepath.Join(f.root.Home, "key"), []byte("k"), 0o600))
	must(t, os.Chmod(filepath.Join(f.root.Home, "key"), 0o600))
	if err := checkTargets(f.root); err != nil {
		t.Fatalf("staged target counted as drift: %v", err)
	}
}

func TestCheckPhrase(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	if err := checkPhrase(f.root, phrase); err != nil {
		t.Fatalf("right phrase: %v", err)
	}
	if err := checkPhrase(f.root, "wrong phrase"); !errors.Is(err, errPhrase) {
		t.Fatalf("wrong phrase: got %v", err)
	}
	must(t, os.WriteFile(f.root.Secrets("recipients"), []byte(f.other.Recipient().String()+"\n"), 0o644))
	if err := checkPhrase(f.root, phrase); !errors.Is(err, errUnlisted) {
		t.Fatalf("unlisted identity: got %v", err)
	}
	if got := names(t, f.root.Home); len(got) != 0 {
		t.Fatalf("verify wrote %v", got)
	}
}

type prompting struct{ ui *plugin.ClientUI }

func (p prompting) Unwrap([]*age.Stanza) ([]byte, error) {
	if _, err := p.ui.RequestValue("yubikey", "PIN:", true); err == nil {
		return nil, errors.New("prompt answered")
	}
	return nil, errors.New("yubikey plugin: PIN request failed")
}

func TestPluginPromptAbortSkipsPhrase(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	sealed, err := Seal([]byte("key"), f.recipients(t))
	must(t, err)
	k := &Keys{u: f.u, wrapped: f.root.Secrets("missing.age")}
	k.plugins = []age.Identity{prompting{clientUI(f.u, &k.stop)}}
	if _, err := k.Open(sealed); !aborted(err) {
		t.Fatalf("got %v, want an aborted prompt", err)
	}
}

func TestIdentitiesRejectNativeKeys(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	if _, err := LoadKeys(f.u, f.root); err != nil {
		t.Fatalf("no identities file: %v", err)
	}
	must(t, os.WriteFile(f.root.Secrets("identities"), []byte(f.other.String()+"\n"), 0o644))
	if _, err := LoadKeys(f.u, f.root); err == nil {
		t.Fatal("native secret key accepted as a tracked identity")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return data
}

func TestSymlinkedParentInsideHome(t *testing.T) {
	home := t.TempDir()
	checkout := filepath.Join(home, "dotfiles", "config", "ssh")
	must(t, os.MkdirAll(checkout, 0o755))
	must(t, os.Symlink("dotfiles/config/ssh", filepath.Join(home, ".ssh")))
	dir := pin(t, home)

	if err := dir.write(".ssh/key", []byte("secret"), 0o600); !errors.Is(err, errIrregular) {
		t.Fatalf("write through a symlinked parent: got %v", err)
	}
	if _, _, err := dir.read(".ssh/key"); !errors.Is(err, errIrregular) {
		t.Fatalf("read through a symlinked parent: got %v", err)
	}
	if got := names(t, checkout); len(got) != 0 {
		t.Fatalf("plaintext landed in the checkout: %v", got)
	}
}

func TestCheckoutFence(t *testing.T) {
	dir := t.TempDir()
	root := paths.Root{Home: dir, Dotfiles: filepath.Join(dir, "dotfiles")}
	must(t, os.MkdirAll(filepath.Join(root.Dotfiles, "config"), 0o755))
	home, err := OpenHome(root)
	must(t, err)
	defer home.Close()
	for _, rel := range []string{"dotfiles/key", "dotfiles/config/ssh/key"} {
		if err := home.write(rel, []byte("secret"), 0o600); !errors.Is(err, errCheckout) {
			t.Errorf("write %s: got %v", rel, err)
		}
	}
	must(t, home.write("dotfiles2/key", []byte("x"), 0o600))
	if got := names(t, root.Dotfiles); !slices.Equal(got, []string{"config"}) {
		t.Fatalf("checkout holds %v", got)
	}

	for _, h := range []string{root.Dotfiles, filepath.Join(root.Dotfiles, "config")} {
		if _, err := OpenHome(paths.Root{Home: h, Dotfiles: root.Dotfiles}); !errors.Is(err, errCheckout) {
			t.Errorf("HOME %s: got %v", h, err)
		}
	}
}

func TestChmodByDescriptor(t *testing.T) {
	home, outside := t.TempDir(), filepath.Join(t.TempDir(), "outside")
	for _, p := range []string{filepath.Join(home, "key"), outside} {
		must(t, os.WriteFile(p, []byte("x"), 0o644))
		must(t, os.Chmod(p, 0o644))
	}
	must(t, os.Symlink(outside, filepath.Join(home, "link")))
	dir := pin(t, home)

	must(t, dir.chmod("key", 0o600))
	if st, _ := os.Stat(filepath.Join(home, "key")); st.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", st.Mode().Perm())
	}
	if err := dir.chmod("link", 0o600); !errors.Is(err, errIrregular) {
		t.Fatalf("chmod through a leaf symlink: got %v", err)
	}
	if st, _ := os.Stat(outside); st.Mode().Perm() != 0o644 {
		t.Fatalf("symlink target mode changed to %v", st.Mode().Perm())
	}
}

func TestRekeyAddsRecipient(t *testing.T) {
	f := newFixture(t, "key:~/key:600\nenv:~/env:600\n")
	recipients := f.root.Secrets("recipients")
	must(t, os.WriteFile(recipients, []byte(f.wrapped.Recipient().String()+"\n"), 0o644))
	secrets := []string{"key", "env"}
	for _, name := range secrets {
		sealed, err := Seal([]byte(name), f.recipients(t))
		must(t, err)
		must(t, os.WriteFile(f.root.Secrets(name+".age"), sealed, 0o644))
	}
	added := identity(t)
	if _, err := open(mustRead(t, f.root.Secrets("key.age")), added); err == nil {
		t.Fatal("new identity decrypted before rekey")
	}
	must(t, os.WriteFile(recipients, []byte(f.wrapped.Recipient().String()+"\n"+added.Recipient().String()+"\n"), 0o644))
	must(t, rekey(f.u, f.root, f.keys(f.wrapped), f.recipients(t), nil))
	for _, name := range secrets {
		got, err := open(mustRead(t, f.root.Secrets(name+".age")), added)
		must(t, err)
		if string(got) != name {
			t.Errorf("%s: got %q", name, got)
		}
	}
	if got := names(t, f.root.Dotfiles); !slices.Equal(got, []string{"secrets"}) {
		t.Fatalf("staging left behind: %v", got)
	}
}

func TestAgeDebugRefused(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	t.Setenv("AGEDEBUG", "plugin")
	if _, err := LoadKeys(f.u, f.root); err == nil {
		t.Fatal("LoadKeys ran with AGEDEBUG set")
	}
	if _, err := Recipients(f.u, f.root); err == nil {
		t.Fatal("Recipients ran with AGEDEBUG set")
	}
}

func TestRekeyUndecryptableChangesNothing(t *testing.T) {
	f := newFixture(t, "key:~/key:600\nenv:~/env:600\n")
	good, err := Seal([]byte("key"), f.recipients(t))
	must(t, err)
	foreign, err := Seal([]byte("env"), []age.Recipient{identity(t).Recipient()})
	must(t, err)
	must(t, os.WriteFile(f.root.Secrets("key.age"), good, 0o644))
	must(t, os.WriteFile(f.root.Secrets("env.age"), foreign, 0o644))
	before := names(t, f.root.Secrets())

	added := identity(t)
	must(t, os.WriteFile(f.root.Secrets("recipients"), []byte(f.wrapped.Recipient().String()+"\n"+added.Recipient().String()+"\n"), 0o644))
	if err := rekey(f.u, f.root, f.keys(f.wrapped), f.recipients(t), nil); err == nil {
		t.Fatal("rekey succeeded with an undecryptable entry")
	}
	if !bytes.Equal(good, mustRead(t, f.root.Secrets("key.age"))) || !bytes.Equal(foreign, mustRead(t, f.root.Secrets("env.age"))) {
		t.Fatal("rekey changed a ciphertext")
	}
	if got := names(t, f.root.Secrets()); !slices.Equal(got, before) {
		t.Fatalf("secrets dir changed from %v to %v", before, got)
	}
}

func pin(t *testing.T, dir string) *Tree {
	t.Helper()
	f, err := os.Open(dir)
	must(t, err)
	t.Cleanup(func() { f.Close() })
	return &Tree{File: f}
}

func TestRekeyRefusesUnexpectedEntry(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	sealed, err := Seal([]byte("key"), f.recipients(t))
	must(t, err)
	must(t, os.WriteFile(f.root.Secrets("key.age"), sealed, 0o644))
	must(t, os.WriteFile(f.root.Secrets("identity"), []byte("plaintext"), 0o600))
	before := map[string][]byte{}
	for _, name := range names(t, f.root.Secrets()) {
		before[name] = mustRead(t, f.root.Secrets(name))
	}
	err = rekey(f.u, f.root, f.keys(f.wrapped), f.recipients(t), nil)
	if err == nil || !strings.Contains(err.Error(), "identity is not managed") {
		t.Fatalf("got %v", err)
	}
	if got := names(t, f.root.Secrets()); len(got) != len(before) {
		t.Fatalf("secrets/ holds %v", got)
	}
	for name, data := range before {
		if !bytes.Equal(data, mustRead(t, f.root.Secrets(name))) {
			t.Errorf("%s changed", name)
		}
	}
	if got := names(t, f.root.Dotfiles); !slices.Equal(got, []string{"secrets"}) {
		t.Fatalf("staging left behind: %v", got)
	}
}

func TestRekeyBesideLeftover(t *testing.T) {
	f := newFixture(t, "key:~/key:600\n")
	sealed, err := Seal([]byte("key"), f.recipients(t))
	must(t, err)
	must(t, os.WriteFile(f.root.Secrets("key.age"), sealed, 0o644))
	leftover := filepath.Join(f.root.Dotfiles, ".secrets.dctl-old")
	must(t, os.MkdirAll(leftover, 0o700))
	must(t, os.WriteFile(filepath.Join(leftover, "key.age"), []byte("stale"), 0o644))
	added := identity(t)
	files := map[string][]byte{"recipients": []byte(f.wrapped.Recipient().String() + "\n" + added.Recipient().String() + "\n")}
	recipients, err := parseRecipients(f.u, files["recipients"])
	must(t, err)
	must(t, rekey(f.u, f.root, f.keys(f.wrapped), recipients, files))
	if got, err := open(mustRead(t, f.root.Secrets("key.age")), added); err != nil || string(got) != "key" {
		t.Fatalf("rekeyed key.age: %q %v", got, err)
	}
	if got := names(t, f.root.Secrets()); !slices.Equal(got, []string{"identity.age", "key.age", "manifest", "recipients"}) {
		t.Fatalf("secrets/ holds %v", got)
	}
	if got := names(t, f.root.Dotfiles); !slices.Equal(got, []string{".secrets.dctl-old", "secrets"}) {
		t.Fatalf("dotfiles holds %v", got)
	}
}

func TestWriteRefusesSymlinkedParents(t *testing.T) {
	dir := t.TempDir()
	root := paths.Root{Home: filepath.Join(dir, "home"), Dotfiles: filepath.Join(dir, "home", "dotfiles")}
	outside := filepath.Join(dir, "outside")
	must(t, os.MkdirAll(root.Dotfiles, 0o755))
	must(t, os.MkdirAll(outside, 0o700))
	must(t, os.Symlink(outside, filepath.Join(root.Home, ".ssh")))
	must(t, os.Symlink(outside, filepath.Join(root.Dotfiles, "share")))
	home, err := OpenHome(root)
	must(t, err)
	defer home.Close()
	checkout, err := OpenCheckout(root)
	must(t, err)
	defer checkout.Close()
	if err := home.WriteFile(".ssh/id", []byte("handle"), 0o600); !errors.Is(err, errIrregular) {
		t.Errorf("home write: got %v", err)
	}
	if err := checkout.WriteFile("share/allowed_signers", []byte("x"), 0o644); !errors.Is(err, errIrregular) {
		t.Errorf("checkout write: got %v", err)
	}
	if got := names(t, outside); len(got) != 0 {
		t.Fatalf("written through a symlink: %v", got)
	}
}
