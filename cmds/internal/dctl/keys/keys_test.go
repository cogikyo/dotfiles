package keys

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/ui"

	"filippo.io/age/plugin"
)

type fake struct {
	t       *testing.T
	log     []string
	out     map[string][]string
	effects map[string]func(dir string)
	fail    map[string]error
}

func (f *fake) Run(_ context.Context, dir string, name string, args ...string) error {
	line := strings.Join(append([]string{name}, args...), " ")
	f.log = append(f.log, line)
	if fn := f.effects[line]; fn != nil {
		fn(dir)
	}
	return f.fail[line]
}

func (f *fake) Output(_ context.Context, _ string, name string, args ...string) (string, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	q, ok := f.out[line]
	if !ok {
		f.t.Fatalf("unexpected %s", line)
	}
	if len(q) > 1 {
		f.out[line] = q[1:]
	}
	return q[0], nil
}

type yubikey struct {
	serial    string
	stub      string
	recipient string
}

func newKey(serial uint32) yubikey {
	point := make([]byte, 33)
	point[0] = 2
	binary.BigEndian.PutUint32(point[1:], serial)
	sum := sha256.Sum256(point)
	data := binary.LittleEndian.AppendUint32(nil, serial)
	data = append(append(data, 0x82), sum[:4]...)
	return yubikey{
		serial:    strconv.FormatUint(uint64(serial), 10),
		stub:      strings.ToUpper(plugin.EncodeIdentity("yubikey", data)),
		recipient: plugin.EncodeRecipient("yubikey", point),
	}
}

const paper = "age1paperpaperpaperpaperpaperpaperpaperpaperpaperpaperpaperp"

func sandbox(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	root := paths.Root{Home: filepath.Join(dir, "home"), Dotfiles: filepath.Join(dir, "dotfiles")}
	for _, d := range []string{root.Home, root.Secrets(), root.Share()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, root.Secrets("recipients"), paper+"\n")
	write(t, root.Secrets("manifest"), "key:~/key:600\n")
	write(t, root.Share("allowed_signers"), "# header\n")
	return root
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileLines(t *testing.T, path string) []string {
	t.Helper()
	got, err := read(path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func quiet() *ui.UI { return ui.New(ui.Options{JSON: true, Yes: true}) }

func device(k yubikey, fido string, generated bool) map[string][]string {
	ids, list, piv := []string{k.stub}, []string{k.recipient}, "PIV version: 5.7.4\n"
	if generated {
		ids, list, piv = []string{"", k.stub}, []string{"", k.recipient}, factory
	}
	return map[string][]string{
		"ykman list --serials":                               {k.serial},
		"ykman --device " + k.serial + " fido info":          {fido},
		"ykman --device " + k.serial + " piv info":           {piv},
		"age-plugin-yubikey --identity --serial " + k.serial: ids,
		"age-plugin-yubikey --list --serial " + k.serial:     list,
	}
}

func swapper(t *testing.T, root paths.Root, log *[]string, fail *error) func(secrets.Edit) error {
	return func(edit secrets.Edit) error {
		var l secrets.Ledger
		l.Recipients, _ = os.ReadFile(root.Secrets("recipients"))
		l.Identities, _ = os.ReadFile(root.Secrets("identities"))
		if err := edit(&l); err != nil {
			return err
		}
		*log = append(*log, "rekey")
		if *fail != nil {
			return *fail
		}
		write(t, root.Secrets("recipients"), string(l.Recipients))
		write(t, root.Secrets("identities"), string(l.Identities))
		return nil
	}
}

const (
	unset   = "PIN: Not set\nAlways Require UV: Off\n"
	factory = "PIV version: 5.7.4\nWARNING: Using default PIN!\nPIN tries remaining: 3/3\nWARNING: Using default PUK!\nPUK tries remaining: 3/3\nWARNING: Using default Management key!\nManagement key algorithm: AES192\n"
	protect = "ykman --device 1234 piv access change-management-key --management-key 010203040506070801020304050607080102030405060708 --algorithm TDES --protect"
)

func TestEnrollOrder(t *testing.T) {
	root := sandbox(t)
	k := newKey(1234)
	ssh := filepath.Join(root.Home, ".ssh", "id_ed25519_sk_1234")
	keygen := "ssh-keygen -t ed25519-sk -O resident -O application=ssh:dctl-release -C yubikey-1234 -f id -N "
	f := &fake{t: t, out: device(k, unset, true)}
	f.effects = map[string]func(string){keygen: func(dir string) {
		write(t, filepath.Join(dir, "id"), "handle")
		write(t, filepath.Join(dir, "id.pub"), "sk-ssh-ed25519@openssh.com AAAAsk yubikey-1234\n")
	}}
	var fail error
	if err := Enroll(t.Context(), quiet(), root, f, swapper(t, root, &f.log, &fail)); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"ykman --device 1234 fido access change-pin",
		"ykman --device 1234 fido config toggle-always-uv",
		"ykman --device 1234 piv access change-pin --pin 123456",
		"ykman --device 1234 piv access change-puk --puk 12345678",
		protect,
		"age-plugin-yubikey --generate --serial 1234 --pin-policy once --touch-policy cached",
		"rekey",
		keygen,
	}
	if !slices.Equal(f.log, want) {
		t.Fatalf("ran\n%s\nwant\n%s", strings.Join(f.log, "\n"), strings.Join(want, "\n"))
	}
	if got := fileLines(t, root.Secrets("recipients")); !slices.Equal(got, []string{paper, k.recipient}) {
		t.Errorf("recipients %v", got)
	}
	if got := fileLines(t, root.Secrets("identities")); !slices.Equal(got, []string{k.stub}) {
		t.Errorf("identities %v", got)
	}
	if got := fileLines(t, root.Share("allowed_signers")); !slices.Equal(got, []string{`yubikey-1234 namespaces="file" sk-ssh-ed25519@openssh.com AAAAsk`}) {
		t.Errorf("allowed_signers %v", got)
	}
	if data, _ := os.ReadFile(ssh); string(data) != "handle" {
		t.Errorf("handle %q", data)
	}
}

func TestEnrollRetry(t *testing.T) {
	root := sandbox(t)
	k := newKey(1234)
	write(t, root.Share("allowed_signers"), `yubikey-1234 namespaces="file" sk-ssh-ed25519@openssh.com AAAAsk`+"\n")
	if err := os.MkdirAll(filepath.Join(root.Home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root.Home, ".ssh", "id_ed25519_sk_1234"), "handle")
	write(t, filepath.Join(root.Home, ".ssh", "id_ed25519_sk_1234.pub"), "sk-ssh-ed25519@openssh.com AAAAsk\n")
	f := &fake{t: t, out: device(k, "PIN: 8 attempt(s) remaining\nAlways Require UV: On\n", false)}
	fail := errors.New("rekey failed")
	rekey := swapper(t, root, &f.log, &fail)
	if err := Enroll(t.Context(), quiet(), root, f, rekey); !errors.Is(err, fail) {
		t.Fatalf("failed rekey: got %v", err)
	}
	if got := fileLines(t, root.Secrets("recipients")); !slices.Equal(got, []string{paper}) {
		t.Fatalf("failed rekey left recipients %v", got)
	}
	fail = nil
	for range 2 {
		f.log = nil
		if err := Enroll(t.Context(), quiet(), root, f, rekey); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(f.log, "rekey") || slices.ContainsFunc(f.log, func(s string) bool { return strings.Contains(s, "--generate") || strings.HasPrefix(s, "ssh-keygen") }) {
			t.Fatalf("retry ran %v", f.log)
		}
		if got := fileLines(t, root.Secrets("recipients")); !slices.Equal(got, []string{paper, k.recipient}) {
			t.Errorf("recipients %v", got)
		}
		if got := fileLines(t, root.Secrets("identities")); !slices.Equal(got, []string{k.stub}) {
			t.Errorf("identities %v", got)
		}
	}
}

func TestRemoveRetry(t *testing.T) {
	root := sandbox(t)
	a, b := newKey(1111), newKey(2222)
	write(t, root.Secrets("recipients"), fmt.Sprintf("# paper\n%s\n%s\n%s\n", paper, a.recipient, b.recipient))
	write(t, root.Secrets("identities"), a.stub+"\n"+b.stub+"\n")
	write(t, root.Share("allowed_signers"), "# header\nyubikey-1111 namespaces=\"file\" sk a\nyubikey-2222 namespaces=\"file\" sk b\n")
	var log []string
	fail := errors.New("rekey failed")
	rekey := swapper(t, root, &log, &fail)
	if err := Remove(quiet(), root, "1111", rekey); !errors.Is(err, fail) {
		t.Fatalf("failed rekey: got %v", err)
	}
	if got := fileLines(t, root.Share("allowed_signers")); !slices.Equal(got, []string{`yubikey-2222 namespaces="file" sk b`}) {
		t.Errorf("signer kept after failed rekey: %v", got)
	}
	if got := fileLines(t, root.Secrets("identities")); !slices.Equal(got, []string{a.stub, b.stub}) {
		t.Errorf("stub dropped before rekey succeeded: %v", got)
	}
	fail = nil
	if err := Remove(quiet(), root, "1111", rekey); err != nil {
		t.Fatal(err)
	}
	if got := fileLines(t, root.Secrets("recipients")); !slices.Equal(got, []string{paper, b.recipient}) {
		t.Errorf("recipients %v", got)
	}
	if got := fileLines(t, root.Secrets("identities")); !slices.Equal(got, []string{b.stub}) {
		t.Errorf("identities %v", got)
	}
	if data, _ := os.ReadFile(root.Secrets("recipients")); !strings.HasPrefix(string(data), "# paper\n") {
		t.Errorf("comment lost: %q", data)
	}
	if err := Remove(quiet(), root, "1111", rekey); err == nil {
		t.Error("removing an unknown serial succeeded")
	}
}

func TestLuksNeverWipes(t *testing.T) {
	sys := t.TempDir()
	dm := filepath.Join(sys, "block", "dm-0")
	if err := os.MkdirAll(filepath.Join(dm, "dm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dm, "slaves", "nvme0n1p2"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dm, "dm", "name"), "root\n")
	write(t, filepath.Join(dm, "dm", "uuid"), "CRYPT-LUKS2-0123-root\n")
	for name, tc := range map[string]struct {
		tokens string
		want   []string
	}{
		"fresh": {`{}`, []string{
			"systemd-cryptenroll --fido2-device=auto --fido2-with-client-pin=yes --fido2-with-user-presence=yes /dev/nvme0n1p2",
			"systemd-cryptenroll --recovery-key /dev/nvme0n1p2",
		}},
		"second key": {`{"0":{"type":"systemd-fido2","keyslots":["1"]},"1":{"type":"systemd-recovery","keyslots":["2"]}}`, []string{
			"systemd-cryptenroll --fido2-device=auto --fido2-with-client-pin=yes --fido2-with-user-presence=yes /dev/nvme0n1p2",
		}},
	} {
		f := &fake{t: t, out: map[string][]string{
			"findmnt -nvo SOURCE /":                                   {"/dev/mapper/root"},
			"cryptsetup luksDump --dump-json-metadata /dev/nvme0n1p2": {`{"keyslots":{"0":{"type":"luks2"}},"tokens":` + tc.tokens + `}`},
		}}
		if err := Luks(t.Context(), quiet(), f, sys, func(string) (bool, error) { return true, nil }); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !slices.Equal(f.log, tc.want) {
			t.Errorf("%s: ran %v", name, f.log)
		}
		for _, line := range f.log {
			if strings.Contains(line, "wipe") {
				t.Errorf("%s: %s", name, line)
			}
		}
	}
}

func TestLuksRetryRecovery(t *testing.T) {
	sys := t.TempDir()
	dm := filepath.Join(sys, "block", "dm-0")
	for _, d := range []string{filepath.Join(dm, "dm"), filepath.Join(dm, "slaves", "vda2")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(dm, "dm", "name"), "root\n")
	write(t, filepath.Join(dm, "dm", "uuid"), "CRYPT-LUKS2-0123-root\n")
	fido := "systemd-cryptenroll --fido2-device=auto --fido2-with-client-pin=yes --fido2-with-user-presence=yes /dev/vda2"
	recovery := "systemd-cryptenroll --recovery-key /dev/vda2"
	dump := "cryptsetup luksDump --dump-json-metadata /dev/vda2"
	f := &fake{t: t, fail: map[string]error{recovery: errors.New("recovery failed")}, out: map[string][]string{
		"findmnt -nvo SOURCE /": {"/dev/mapper/root"},
		dump:                    {`{"tokens":{}}`, `{"tokens":{"0":{"type":"systemd-fido2","keyslots":["1"]}}}`},
	}}
	yes := func(string) (bool, error) { return true, nil }
	if err := Luks(t.Context(), quiet(), f, sys, yes); err == nil {
		t.Fatal("recovery failure ignored")
	}
	if !slices.Equal(f.log, []string{fido, recovery}) {
		t.Fatalf("first run ran %v", f.log)
	}
	f.log, f.fail = nil, nil
	asked := false
	no := func(string) (bool, error) { asked = true; return false, nil }
	if err := Luks(t.Context(), quiet(), f, sys, no); err != nil {
		t.Fatal(err)
	}
	if !asked || !slices.Equal(f.log, []string{recovery}) {
		t.Fatalf("retry ran %v (asked %v)", f.log, asked)
	}
}

func TestSignerRewriteFailureKeepsFile(t *testing.T) {
	root := sandbox(t)
	old := "# header\nyubikey-1 namespaces=\"file\" sk a\nyubikey-2 namespaces=\"file\" sk b\n"
	write(t, root.Share("allowed_signers"), old)
	if err := os.Chmod(root.Share(), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root.Share(), 0o755) })
	err := rewrite(root, func(data []byte) []byte {
		return dropped(data, func(line string) bool { return signs(line, "yubikey-1") })
	})
	if err == nil {
		t.Fatal("rewrite into a read-only dir succeeded")
	}
	if data, _ := os.ReadFile(root.Share("allowed_signers")); string(data) != old {
		t.Fatalf("allowed_signers changed to %q", data)
	}
}

func TestSignerHandleRecovered(t *testing.T) {
	const pub = "sk-ssh-ed25519@openssh.com AAAAgood"
	for name, tc := range map[string]struct{ local, keyed string }{
		"missing":   {"", pub},
		"no pub":    {"-", pub},
		"wrong pub": {"sk-ssh-ed25519@openssh.com AAAAold", pub},
		"mismatch":  {"", "sk-ssh-ed25519@openssh.com AAAAother"},
	} {
		root := sandbox(t)
		write(t, root.Share("allowed_signers"), "yubikey-1234 namespaces=\"file\" "+pub+"\n")
		file := filepath.Join(root.Home, ".ssh", "id_ed25519_sk_1234")
		if tc.local != "" {
			if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
				t.Fatal(err)
			}
			write(t, file, "old handle")
			if tc.local != "-" {
				write(t, file+".pub", tc.local+"\n")
			}
		}
		f := &fake{t: t, effects: map[string]func(string){"ssh-keygen -K -N ": func(dir string) {
			write(t, filepath.Join(dir, "id_ed25519_sk_rk_dctl-release"), "handle")
			write(t, filepath.Join(dir, "id_ed25519_sk_rk_dctl-release.pub"), tc.keyed+" comment\n")
		}}}
		err := enrollSigner(t.Context(), quiet(), root, f, "1234")
		if !slices.Equal(f.log, []string{"ssh-keygen -K -N "}) {
			t.Errorf("%s: ran %v", name, f.log)
		}
		if name == "mismatch" {
			if _, statErr := os.Stat(file); err == nil || statErr == nil {
				t.Fatalf("mismatch: err %v, handle installed: %v", err, statErr == nil)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for path, mode := range map[string]os.FileMode{file: 0o600, file + ".pub": 0o644} {
			st, err := os.Stat(path)
			if err != nil || st.Mode().Perm() != mode {
				t.Errorf("%s: %s: %v %v", name, path, st, err)
			}
		}
		if data, _ := os.ReadFile(file); string(data) != "handle" {
			t.Errorf("%s: handle %q", name, data)
		}
	}
}

func TestForcedPinChange(t *testing.T) {
	info := strings.Join([]string{
		"AAGUID:                       a25342c0-3cdc-4414-8e46-f4807fca511c",
		"PIN:                          8 attempt(s) remaining",
		"Minimum PIN length:           4",
		"Always Require UV:            Off",
		"Credential storage remaining: 100",
		"Enterprise Attestation:       Disabled",
		"NOTE: The FIDO PIN is disabled and must be changed before it can be used!",
	}, "\n") + "\n"
	f := &fake{t: t, out: map[string][]string{
		"ykman --device 1234 fido info": {info},
		"ykman --device 1234 piv info":  {factory},
	}}
	steps, err := pinSteps(t.Context(), f, "1234")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range steps {
		got = append(got, "ykman "+strings.Join(s.args, " "))
	}
	want := []string{
		"ykman --device 1234 fido access change-pin",
		"ykman --device 1234 fido config toggle-always-uv",
		"ykman --device 1234 piv access change-pin --pin 123456",
		"ykman --device 1234 piv access change-puk --puk 12345678",
		protect,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("steps %v, want %v", got, want)
	}
}

func TestEnrollPreflight(t *testing.T) {
	root := sandbox(t)
	write(t, root.Secrets("identity"), "plaintext")
	f := &fake{t: t, out: map[string][]string{}}
	var fail error
	err := Enroll(t.Context(), quiet(), root, f, swapper(t, root, &f.log, &fail))
	if err == nil || !strings.Contains(err.Error(), "identity is not managed") || len(f.log) != 0 {
		t.Fatalf("got %v, ran %v", err, f.log)
	}
}
