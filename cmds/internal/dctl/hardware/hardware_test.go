package hardware

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestWLAN(t *testing.T) {
	sys := t.TempDir()
	mkdir(t, filepath.Join(sys, "class", "net", "lo"))
	mkdir(t, filepath.Join(sys, "class", "net", "enp1s0", "device"))
	if err := noWLAN(sys); err != nil {
		t.Fatalf("wired only: %v", err)
	}
	wlan := filepath.Join(sys, "class", "net", "wlp3s0")
	mkdir(t, filepath.Join(wlan, "wireless"))
	mkdir(t, filepath.Join(wlan, "device"))
	if err := os.Symlink("../../../bus/pci/drivers/mt7925e", filepath.Join(wlan, "device", "driver")); err != nil {
		t.Fatal(err)
	}
	err := noWLAN(sys)
	if err == nil || !strings.Contains(err.Error(), "wlp3s0 (driver mt7925e)") {
		t.Fatalf("err %v", err)
	}
}

func TestBluetooth(t *testing.T) {
	sys := t.TempDir()
	mkdir(t, filepath.Join(sys, "class", "bluetooth", "hci0:256"))
	if bluetooth(sys) == nil {
		t.Fatal("connection entry counted as controller")
	}
	mkdir(t, filepath.Join(sys, "class", "bluetooth", "hci0"))
	if err := bluetooth(sys); err != nil {
		t.Fatal(err)
	}
}

func TestFirmwareFailures(t *testing.T) {
	log := `Spectre V2 : Enabling Restricted Speculation for firmware calls
amdgpu 0000:74:00.0: [drm] Loading DMUB firmware via PSP: version=0x0400004C
bluetooth hci0: Direct firmware load for mediatek/BT_RAM_CODE_MT7925_1_1_hdr.bin failed with error -2
mt7925e 0000:03:00.0: Failed to load firmware
`
	got := firmwareFailures(log)
	want := []string{
		"bluetooth hci0: Direct firmware load for mediatek/BT_RAM_CODE_MT7925_1_1_hdr.bin failed with error -2",
		"mt7925e 0000:03:00.0: Failed to load firmware",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestPending(t *testing.T) {
	got, err := pending([]byte(`{"Devices":[{"Name":"BIOS","Releases":[{"Version":"3.07"}]},{"Name":"SSD","Releases":[]}]}`), nil)
	if err != nil || !slices.Equal(got, []string{"BIOS 3.07"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestFresh(t *testing.T) {
	const remotes = `{
  "Remotes" : [
    {
      "Id" : "lvfs",
      "Kind" : "download",
      "MetadataUri" : "https://cdn.fwupd.org/downloads/firmware.xml.zst",
      "MetadataUriSig" : "https://cdn.fwupd.org/downloads/firmware.xml.zst.jcat",
      "Title" : "Linux Vendor Firmware Service",
      "FilenameCache" : "/var/lib/fwupd/remotes.d/lvfs/firmware.xml.zst",
      "FilenameSource" : "/etc/fwupd/remotes.d/lvfs.conf",
      "Flags" : 9,
      "Enabled" : true,
      "ApprovalRequired" : false,
      "AutomaticReports" : false,
      "AutomaticSecurityReports" : true,
      "Priority" : 0,
      "Mtime" : %d,
      "RefreshInterval" : 86400,
      "RemotesDir" : "/var/lib/fwupd/remotes.d"
    }
  ]
}`
	now := time.Unix(1_790_000_000, 0)
	if err := fresh(fmt.Appendf(nil, remotes, now.Add(-24*time.Hour).Unix()), now); err != nil {
		t.Fatalf("day-old metadata: %v", err)
	}
	for _, mtime := range []int64{0, -1, now.Add(-maxAge - time.Hour).Unix()} {
		if fresh(fmt.Appendf(nil, remotes, mtime), now) == nil {
			t.Errorf("mtime %d reported fresh", mtime)
		}
	}
}
