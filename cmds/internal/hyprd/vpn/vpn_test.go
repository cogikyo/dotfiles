package vpn

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	for _, op := range []func(connection) (string, error){v.up, v.down} {
		if _, err := op(connection{Name: "Trend"}); err == nil || !strings.Contains(err.Error(), "matches 2") {
			t.Errorf("err = %v, want ambiguous name rejected", err)
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
	if strings.Contains(string(calls), "connection up") || strings.Contains(string(calls), "connection down") {
		t.Fatalf("nmcli calls = %q; want no up/down on an ambiguous name", calls)
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

	if err := storeVPNSecrets(conn, want); err != nil {
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
