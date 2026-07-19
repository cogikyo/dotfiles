package providers

import (
	"encoding/json"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestBluetoothSnapshot(t *testing.T) {
	const address = "AA:BB:CC:DD:EE:FF"
	devicePath := dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF")
	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{
		devicePath: {
			bluezDeviceInterface: {
				"Address":   dbus.MakeVariant("aa:bb:cc:dd:ee:ff"),
				"Name":      dbus.MakeVariant("AirPods Max"),
				"Connected": dbus.MakeVariant(true),
			},
			bluezBatteryInterface: {
				"Percentage": dbus.MakeVariant(byte(61)),
			},
		},
	}

	path, state := bluetoothSnapshot(objects, address)
	if path != devicePath {
		t.Fatalf("device path = %q", path)
	}
	if state.Status != "connected" || state.Name != "AirPods Max" || !state.BatteryPresent || state.BatteryPercent != 61 {
		t.Fatalf("snapshot = %+v", state)
	}

	objects[devicePath][bluezDeviceInterface]["Connected"] = dbus.MakeVariant(false)
	_, state = bluetoothSnapshot(objects, address)
	if state.Status != "disconnected" || state.BatteryPresent || state.BatteryPercent != 0 {
		t.Fatalf("disconnected snapshot retained battery: %+v", state)
	}
}

func TestMergeBluetoothState(t *testing.T) {
	bluez := BluetoothState{
		Status:         "connected",
		Name:           "AirPods Max",
		BatteryPresent: true,
		BatteryPercent: 61,
	}
	metadata := &librePodsState{
		Owner:                          ":1.42",
		Generation:                     "123-4",
		Address:                        "aa:bb:cc:dd:ee:ff",
		Readiness:                      "ready",
		BatteryPercent:                 73,
		BatteryValid:                   true,
		BatteryCharging:                true,
		NoiseControl:                   "anc",
		NoiseControlModes:              []string{"anc", "transparency", "adaptive"},
		WearState:                      "not_worn",
		ConversationAwarenessSupported: true,
		PersonalizedVolumeSupported:    true,
		PersonalizedVolumeEnabled:      true,
	}
	pending := &noiseOperation{owner: ":1.42", generation: "123-4", desired: "adaptive"}

	state := mergeBluetoothState(bluez, "AA:BB:CC:DD:EE:FF", metadata, pending)
	if !state.BatteryPresent || state.BatteryPercent != 73 || state.BatteryCharging == nil || !*state.BatteryCharging {
		t.Fatalf("LibrePods battery did not take precedence: %+v", state)
	}
	if state.NoiseControl != "anc" || !state.NoisePending || state.NoiseDesired != "adaptive" || state.WearState != "not_worn" {
		t.Fatalf("enrichment = %+v", state)
	}
	if state.ConversationAwareness == nil || *state.ConversationAwareness || state.PersonalizedVolume == nil || !*state.PersonalizedVolume || state.HiResMic != nil {
		t.Fatalf("optional capabilities = %+v", state)
	}

	metadata.BatteryValid = false
	state = mergeBluetoothState(bluez, "AA:BB:CC:DD:EE:FF", metadata, pending)
	if state.BatteryPercent != 61 || state.BatteryCharging != nil {
		t.Fatalf("BlueZ battery fallback = %+v", state)
	}

	metadata.Generation = "new-session"
	state = mergeBluetoothState(bluez, "AA:BB:CC:DD:EE:FF", metadata, pending)
	if state.NoisePending || state.NoiseDesired != "" {
		t.Fatalf("stale pending action survived generation change: %+v", state)
	}

	metadata.Address = "11:22:33:44:55:66"
	state = mergeBluetoothState(bluez, "AA:BB:CC:DD:EE:FF", metadata, nil)
	if state.NoiseControl != "" || state.WearState != "" || state.BatteryPercent != 61 {
		t.Fatalf("foreign device enriched state: %+v", state)
	}

	bluez.Status = "disconnected"
	state = mergeBluetoothState(bluez, "11:22:33:44:55:66", metadata, nil)
	if state.NoiseControl != "" || state.BatteryPresent || state.BatteryPercent != 0 {
		t.Fatalf("disconnect retained enrichment: %+v", state)
	}
}

func TestNextNoiseModeTracksLatestDesired(t *testing.T) {
	modes := []string{"anc", "transparency", "adaptive", "off"}
	first := nextNoiseMode(modes, "anc", "up")
	second := nextNoiseMode(modes, first, "up")
	if first != "transparency" || second != "adaptive" {
		t.Fatalf("rapid targets = %q then %q", first, second)
	}
	if got := nextNoiseMode(modes, "anc", "down"); got != "off" {
		t.Fatalf("reverse wrap = %q", got)
	}
}

func TestNoiseOperationWaitsForCorrectiveTargetConfirmation(t *testing.T) {
	bluez := BluetoothState{Status: "connected"}
	metadata := &librePodsState{
		Owner:             ":1.42",
		Generation:        "123-4",
		Address:           "AA:BB:CC:DD:EE:FF",
		Readiness:         "ready",
		NoiseControl:      "anc",
		NoiseControlModes: []string{"anc", "transparency"},
	}
	noise := &noiseOperation{owner: metadata.Owner, generation: metadata.Generation, desired: "transparency"}
	first := noise.launch(1, 1)
	if first.target != "transparency" || !noise.inFlight {
		t.Fatalf("first command = %+v, operation = %+v", first, noise)
	}

	noise.desired = "anc"
	if noise.observe(bluez, metadata.Address, metadata, 2) {
		t.Fatal("an unrelated ANC update cleared the pending corrective action")
	}
	if clear, relaunch := noise.completed(nil); clear || !relaunch {
		t.Fatalf("first completion = clear:%v relaunch:%v", clear, relaunch)
	}

	corrective := noise.launch(2, 2)
	if corrective.target != "anc" || noise.confirmed {
		t.Fatalf("corrective command = %+v, operation = %+v", corrective, noise)
	}
	metadata.NoiseControl = "transparency"
	if noise.observe(bluez, metadata.Address, metadata, 3) {
		t.Fatal("old command confirmation cleared the corrective action")
	}
	if clear, relaunch := noise.completed(nil); clear || relaunch {
		t.Fatalf("corrective completion before confirmation = clear:%v relaunch:%v", clear, relaunch)
	}

	metadata.NoiseControl = "anc"
	if !noise.observe(bluez, metadata.Address, metadata, 4) {
		t.Fatal("confirmed corrective target did not clear the pending action")
	}
}

func TestNoiseOperationAcceptsConfirmationBeforeCommandReply(t *testing.T) {
	bluez := BluetoothState{Status: "connected"}
	metadata := &librePodsState{
		Owner:             ":1.42",
		Generation:        "123-4",
		Address:           "AA:BB:CC:DD:EE:FF",
		Readiness:         "ready",
		NoiseControl:      "anc",
		NoiseControlModes: []string{"anc", "transparency"},
	}
	noise := &noiseOperation{owner: metadata.Owner, generation: metadata.Generation, desired: "transparency"}
	noise.launch(1, 4)
	metadata.NoiseControl = "transparency"
	if noise.observe(bluez, metadata.Address, metadata, 5) {
		t.Fatal("signal confirmation cleared before the command reply")
	}
	if clear, relaunch := noise.completed(nil); !clear || relaunch {
		t.Fatalf("completion after confirmation = clear:%v relaunch:%v", clear, relaunch)
	}
}

func TestTrackedBluezDisconnectedSignal(t *testing.T) {
	path := dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_FF")
	signal := &dbus.Signal{
		Path: path,
		Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
		Body: []any{bluezDeviceInterface, map[string]dbus.Variant{"Connected": dbus.MakeVariant(false)}, []string{}},
	}
	if !trackedBluezDisconnected(signal, path) {
		t.Fatal("tracked disconnect signal was rejected")
	}
	state := disconnectedBluetoothState(BluetoothState{Status: "connected", Name: "AirPods Max", BatteryPresent: true, BatteryPercent: 61})
	if state.Status != "disconnected" || state.Name != "AirPods Max" || state.BatteryPresent || state.BatteryPercent != 0 {
		t.Fatalf("disconnect state = %+v", state)
	}

	signal.Path = "/org/bluez/hci0/dev_OTHER"
	if trackedBluezDisconnected(signal, path) {
		t.Fatal("foreign device disconnect signal was accepted")
	}
	signal.Path = path
	signal.Body[1] = map[string]dbus.Variant{"Connected": dbus.MakeVariant(true)}
	if trackedBluezDisconnected(signal, path) {
		t.Fatal("connected signal was treated as disconnect")
	}
}

func TestBluetoothScanFencesNewerSignal(t *testing.T) {
	scan := bluetoothScan{generation: 3, revision: 7}
	if !currentBluetoothScan(scan, 3, 7) {
		t.Fatal("current scan was rejected")
	}
	if currentBluetoothScan(scan, 3, 8) {
		t.Fatal("scan that predates a tracked signal was accepted")
	}
	if currentBluetoothScan(scan, 4, 7) {
		t.Fatal("scan from an old owner generation was accepted")
	}
}

func TestBluetoothStateOmitsUnavailableEnrichment(t *testing.T) {
	state := mergeBluetoothState(BluetoothState{Status: "connected", BatteryPresent: true, BatteryPercent: 61}, "AA:BB:CC:DD:EE:FF", nil, nil)
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"battery_charging", "noise_control", "noise_desired", "wear_state", "conversation_awareness", "personalized_volume", "hires_mic"} {
		if _, present := fields[name]; present {
			t.Errorf("unavailable field %q was serialized in %s", name, data)
		}
	}
}
