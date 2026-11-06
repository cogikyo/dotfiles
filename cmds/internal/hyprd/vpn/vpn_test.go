package vpn

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
)

type fakeConnection struct {
	dbus.BusObject
	settings map[string]map[string]dbus.Variant
	secrets  map[string]string
	updates  int
}

func (f *fakeConnection) Call(method string, _ dbus.Flags, args ...any) *dbus.Call {
	switch method {
	case nmConnection + ".GetSettings":
		return &dbus.Call{Body: []any{f.settings}}
	case nmConnection + ".GetSecrets":
		vpn := map[string]map[string]dbus.Variant{"vpn": {"secrets": dbus.MakeVariant(maps.Clone(f.secrets))}}
		return &dbus.Call{Body: []any{vpn}}
	case nmConnection + ".Update2":
		f.updates++
		settings := args[0].(map[string]map[string]dbus.Variant)
		if err := settings["vpn"]["secrets"].Store(&f.secrets); err != nil {
			return &dbus.Call{Err: err}
		}
		return &dbus.Call{Body: []any{map[string]dbus.Variant{}}}
	}
	return &dbus.Call{Err: dbus.ErrMsgNoObject}
}

type fakeSettings struct {
	dbus.BusObject
	calls []string
}

func (f *fakeSettings) Call(method string, _ dbus.Flags, args ...any) *dbus.Call {
	f.calls = append(f.calls, method)
	if method == nmSettings+".GetConnectionByUuid" && args[0] == "8f2c4f2e-uuid" {
		return &dbus.Call{Body: []any{dbus.ObjectPath(nmSettingsPath + "/7")}}
	}
	return &dbus.Call{Err: dbus.ErrMsgNoObject}
}

func TestStagedKeyfileUUIDSelectsConnection(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "Trend.nmconnection")
	keyfile := "[connection]\nid=Trend\nuuid=8f2c4f2e-uuid\ntype=vpn\n\n[vpn]\nservice-type=org.freedesktop.NetworkManager.l2tp\n"
	if err := os.WriteFile(profile, []byte(keyfile), 0o600); err != nil {
		t.Fatal(err)
	}
	uuid, complete, err := readKeyfile(profile)
	if err != nil || !complete {
		t.Fatalf("readKeyfile = %q, %v, %v; want a complete keyfile", uuid, complete, err)
	}

	settings := &fakeSettings{}
	path, err := connectionPath(settings, uuid)
	if err != nil {
		t.Fatal(err)
	}
	if path != nmSettingsPath+"/7" || !slices.Equal(settings.calls, []string{nmSettings + ".GetConnectionByUuid"}) {
		t.Fatalf("path = %s, calls = %q; want one UUID lookup", path, settings.calls)
	}
}

func TestNameOperationsRejectAmbiguousID(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho \"$*\" >> " + log + "\nprintf 'Trend\\nHome\\nTrend\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "nmcli"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	v := New(nil)
	for verb, next := range modes {
		if _, err := v.apply(connection{Name: "Trend"}, next); err == nil || !strings.Contains(err.Error(), "matches 2") {
			t.Errorf("%s: err = %v, want ambiguous name rejected", verb, err)
		}
	}
	if _, err := connectionExists("Trend"); err == nil {
		t.Error("connectionExists accepted an ambiguous name")
	}
	if exists, err := connectionExists("Home"); err != nil || !exists {
		t.Errorf("connectionExists(Home) = %v, %v; want unique match", exists, err)
	}

	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "connection up") || strings.Contains(string(calls), "connection down") || strings.Contains(string(calls), "connection modify") {
		t.Fatalf("nmcli calls = %q; want no up/down/modify on an ambiguous name", calls)
	}
}

func TestVPNSecretsTravelOnlyOverDBus(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	conn := &fakeConnection{
		settings: map[string]map[string]dbus.Variant{
			"connection": {"id": dbus.MakeVariant("Trend")},
			"vpn":        {"service-type": dbus.MakeVariant("org.freedesktop.NetworkManager.l2tp")},
		},
		secrets: map[string]string{},
	}
	want := map[string]string{"password": `pa,ss\word`, "ipsec-psk": "shared-psk-value"}

	h := tempLock(t)
	defer h.unlock()
	if err := h.storeSecrets(conn, want); err != nil {
		t.Fatal(err)
	}
	got, err := vpnSecrets(conn)
	if err != nil {
		t.Fatal(err)
	}
	if conn.updates != 1 || !maps.Equal(got, want) {
		t.Fatalf("updates = %d, stored secrets = %q; want one update with %q", conn.updates, got, want)
	}
}

func tempLock(t *testing.T) *held {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vpn-keep.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := lockPath
	lockPath = path
	t.Cleanup(func() { lockPath = old })
	h, err := lock()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func lockFree(t *testing.T) bool {
	t.Helper()
	f, err := os.Open(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return true
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal(err)
	}
	return false
}

func TestMutationsRefuseWithoutLock(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	for _, name := range []string{"nmcli", "sudo"} {
		script := "#!/bin/sh\necho \"" + name + " $*\" >> " + log + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	conn := &fakeConnection{
		settings: map[string]map[string]dbus.Variant{"vpn": {}},
		secrets:  map[string]string{},
	}
	secrets := map[string]string{"password": "p"}

	var none *held
	h := tempLock(t)
	if lockFree(t) {
		t.Fatal("lock() returned without holding the flock")
	}
	if _, err := h.run("nmcli", "connection", "down", "id", "Trend"); err != nil {
		t.Fatalf("held run: %v", err)
	}
	h.unlock()
	if !lockFree(t) {
		t.Fatal("unlock() left the flock held")
	}

	for name, h := range map[string]*held{"nil": none, "unlocked": h} {
		for _, args := range [][]string{
			{"nmcli", "connection", "modify", "id", "Trend"},
			{"nmcli", "connection", "up", "id", "Trend"},
			{"sudo", "nmcli", "connection", "load", "/x"},
		} {
			if _, err := h.run(args[0], args[1:]...); !errors.Is(err, errUnlocked) {
				t.Errorf("%s run %q: err = %v; want errUnlocked", name, args, err)
			}
		}
		if err := h.storeSecrets(conn, secrets); !errors.Is(err, errUnlocked) {
			t.Errorf("%s storeSecrets: err = %v; want errUnlocked", name, err)
		}
	}
	if conn.updates != 0 {
		t.Errorf("Update2 calls = %d without the lock; want 0", conn.updates)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(calls)); got != "nmcli connection down id Trend" {
		t.Errorf("exec calls = %q; want only the held call", got)
	}
}

func TestOnlyHeldLockMutatesNetworkManager(t *testing.T) {
	readOnly := map[string]bool{"nmcliOutput": true, "export": true, "authorize": true}
	mutating := map[string]bool{"modify": true, "up": true, "down": true, "load": true, "delete": true, "add": true, "reload": true, "import": true, "edit": true, "clone": true}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "vpn.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		onHeld := false
		if fn.Recv != nil {
			if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok && id.Name == "held" {
					onHeld = true
				}
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if x, ok := n.X.(*ast.Ident); ok && x.Name == "exec" && n.Sel.Name == "Command" && !onHeld && !readOnly[fn.Name.Name] {
					t.Errorf("%s: %s runs exec.Command outside (*held)", fset.Position(n.Pos()), fn.Name.Name)
				}
			case *ast.BasicLit:
				if strings.Contains(n.Value, "Update2") && !onHeld {
					t.Errorf("%s: %s calls Update2 outside (*held)", fset.Position(n.Pos()), fn.Name.Name)
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "nmcliOutput" {
					for _, arg := range n.Args {
						if lit, ok := arg.(*ast.BasicLit); ok && mutating[strings.Trim(lit.Value, `"`)] {
							t.Errorf("%s: %s passes %s to read-only nmcliOutput", fset.Position(lit.Pos()), fn.Name.Name, lit.Value)
						}
					}
				}
			}
			return true
		})
	}
}

func TestReconcileComparesSavedModeWithAppliedRoutes(t *testing.T) {
	split := profile{keep: true, split: true}
	full := profile{keep: true}
	off := profile{split: true}
	for _, tc := range []struct {
		name           string
		want           profile
		active, routed bool
		down, up       bool
	}{
		{"split over applied full reconnects", split, true, true, true, true},
		{"split over applied split is a no-op", split, true, false, false, false},
		{"full over applied split reconnects", full, true, false, true, true},
		{"full over applied full is a no-op", full, true, true, false, false},
		{"split while inactive connects", split, false, false, false, true},
		{"full while inactive connects", full, false, false, false, true},
		{"off while active disconnects", off, true, false, true, false},
		{"off while inactive does nothing", off, false, false, false, false},
	} {
		down, up := reconcile(tc.want, tc.active, tc.routed)
		if down != tc.down || up != tc.up {
			t.Errorf("%s: reconcile = down %v, up %v; want down %v, up %v", tc.name, down, up, tc.down, tc.up)
		}
	}
}

func TestHasDefaultRouteReadsAppliedRoutes(t *testing.T) {
	split := "dst = 192.168.4.1/32, nh = 0.0.0.0, mt = 0 | dst = 10.1.0.0/24, nh = 0.0.0.0, mt = 50 | dst = 192.168.4.1/32, nh = 0.0.0.0, mt = 50"
	full := split + " | dst = 0.0.0.0/0, nh = 0.0.0.0, mt = 50"
	if hasDefaultRoute(split) {
		t.Errorf("hasDefaultRoute(split routes) = true")
	}
	if !hasDefaultRoute(full) {
		t.Errorf("hasDefaultRoute(full routes) = false")
	}
	if hasDefaultRoute("") {
		t.Errorf("hasDefaultRoute(\"\") = true")
	}
}

func TestVerbsMapToModes(t *testing.T) {
	split := profile{keep: true, split: true}
	full := profile{keep: true}
	off := profile{split: true}
	for verb, cases := range map[string][][2]profile{
		"up":     {{off, full}, {split, full}, {full, full}},
		"full":   {{off, full}, {split, full}},
		"down":   {{off, split}, {full, split}, {split, split}},
		"split":  {{off, split}, {full, split}},
		"off":    {{full, profile{}}, {split, off}},
		"toggle": {{full, split}, {split, full}, {off, full}},
	} {
		for _, c := range cases {
			if got := modes[verb](c[0]); got != c[1] {
				t.Errorf("%s from %s = %+v; want %+v", verb, c[0].mode(), got, c[1])
			}
		}
	}
}
