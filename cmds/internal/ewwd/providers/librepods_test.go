package providers

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestParseLibrePodsState(t *testing.T) {
	props := librePodsProperties()
	state, err := parseLibrePodsState(":1.42", props)
	if err != nil {
		t.Fatal(err)
	}
	if state.Owner != ":1.42" || state.Generation != "123-4" || state.Address != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("identity = %+v", state)
	}
	if state.BatteryPercent != 73 || !state.BatteryValid || state.BatteryCharging {
		t.Fatalf("battery = %+v", state)
	}
	if state.NoiseControl != "anc" || len(state.NoiseControlModes) != 4 || state.WearState != "worn" {
		t.Fatalf("metadata = %+v", state)
	}
}

func TestParseLibrePodsStateRejectsWireDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]dbus.Variant)
		is     error
	}{
		{
			name: "version",
			mutate: func(props map[string]dbus.Variant) {
				props["InterfaceVersion"] = dbus.MakeVariant(uint32(2))
			},
			is: errLibrePodsVersion,
		},
		{
			name: "missing generation",
			mutate: func(props map[string]dbus.Variant) {
				delete(props, "SessionGeneration")
			},
		},
		{
			name: "ready without address",
			mutate: func(props map[string]dbus.Variant) {
				props["DeviceAddress"] = dbus.MakeVariant("")
			},
		},
		{
			name: "battery type",
			mutate: func(props map[string]dbus.Variant) {
				props["BatteryPercent"] = dbus.MakeVariant(uint32(73))
			},
		},
		{
			name: "unsupported current mode",
			mutate: func(props map[string]dbus.Variant) {
				props["NoiseControl"] = dbus.MakeVariant("anc")
				props["NoiseControlModes"] = dbus.MakeVariant([]string{"adaptive"})
			},
		},
		{
			name: "invalid wear",
			mutate: func(props map[string]dbus.Variant) {
				props["WearState"] = dbus.MakeVariant("somewhere")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := librePodsProperties()
			tt.mutate(props)
			_, err := parseLibrePodsState(":1.42", props)
			if err == nil {
				t.Fatal("malformed properties accepted")
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Fatalf("error = %v, want %v", err, tt.is)
			}
		})
	}
}

func TestLibrePodsChangedPropertiesRequiresFullCurrentInterface(t *testing.T) {
	props := librePodsProperties()
	signal := &dbus.Signal{
		Sender: ":1.42",
		Path:   librePodsPath,
		Name:   "org.freedesktop.DBus.Properties.PropertiesChanged",
		Body:   []any{librePodsInterface, props, []string{}},
	}
	got, ok := librePodsChangedProperties(signal)
	if !ok || len(got) != len(props) {
		t.Fatalf("full update rejected: ok=%v properties=%d", ok, len(got))
	}
	signal.Body[2] = []string{"NoiseControl"}
	if _, ok := librePodsChangedProperties(signal); ok {
		t.Fatal("invalidated update accepted")
	}
	signal.Body = []any{"wrong.Interface", props, []string{}}
	if _, ok := librePodsChangedProperties(signal); ok {
		t.Fatal("foreign interface accepted")
	}
}

func TestCurrentLibrePodsSignalRequiresUniqueOwnerAndPath(t *testing.T) {
	signal := &dbus.Signal{
		Sender: ":1.42",
		Path:   librePodsPath,
		Name:   "org.freedesktop.DBus.Properties.PropertiesChanged",
	}
	if !currentLibrePodsSignal(signal, ":1.42") {
		t.Fatal("current owner signal rejected")
	}
	if currentLibrePodsSignal(signal, ":1.99") {
		t.Fatal("obsolete owner signal accepted")
	}
	signal.Path = "/foreign"
	if currentLibrePodsSignal(signal, ":1.42") {
		t.Fatal("foreign object path accepted")
	}
}

func TestLibrePodsSnapshotFencesNewerSignal(t *testing.T) {
	snapshot := librePodsSnapshot{epoch: 3, revision: 7, owner: ":1.42"}
	if !currentLibrePodsSnapshot(snapshot, 3, 7, ":1.42") {
		t.Fatal("current snapshot rejected")
	}
	if currentLibrePodsSnapshot(snapshot, 3, 8, ":1.42") {
		t.Fatal("snapshot that predates a signal was accepted")
	}
	if currentLibrePodsSnapshot(snapshot, 4, 7, ":1.42") {
		t.Fatal("snapshot from an old owner epoch was accepted")
	}
	if currentLibrePodsSnapshot(snapshot, 3, 7, ":1.99") {
		t.Fatal("snapshot from an old unique owner was accepted")
	}
}

func librePodsProperties() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"InterfaceVersion":               dbus.MakeVariant(uint32(1)),
		"SessionGeneration":              dbus.MakeVariant("123-4"),
		"DeviceAddress":                  dbus.MakeVariant("AA:BB:CC:DD:EE:FF"),
		"Readiness":                      dbus.MakeVariant("ready"),
		"BatteryPercent":                 dbus.MakeVariant(byte(73)),
		"BatteryValid":                   dbus.MakeVariant(true),
		"BatteryCharging":                dbus.MakeVariant(false),
		"NoiseControl":                   dbus.MakeVariant("anc"),
		"NoiseControlModes":              dbus.MakeVariant([]string{"anc", "transparency", "adaptive", "off"}),
		"WearState":                      dbus.MakeVariant("worn"),
		"ConversationAwarenessSupported": dbus.MakeVariant(true),
		"ConversationAwarenessEnabled":   dbus.MakeVariant(false),
		"PersonalizedVolumeSupported":    dbus.MakeVariant(true),
		"PersonalizedVolumeEnabled":      dbus.MakeVariant(true),
		"HiResMicSupported":              dbus.MakeVariant(true),
		"HiResMicEnabled":                dbus.MakeVariant(false),
	}
}
