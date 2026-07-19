package providers

import (
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
