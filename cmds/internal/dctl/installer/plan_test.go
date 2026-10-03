package installer

import (
	"dotfiles/cmds/internal/dctl/iso"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var testPlan = plan{
	Disk:       disk{Path: "/dev/nvme0n1", Model: "WD_BLACK SN850X 2000GB", Serial: "24123A800123", Tran: "nvme", Size: 2000398934016, Sector: 512},
	Zone:       "America/Denver",
	LUKSID:     "6f1c2a7e-3b4d-4e5f-8a9b-0c1d2e3f4a5b",
	RootID:     "0d9e8f7a-6b5c-4d3e-9f2a-1b0c9d8e7f6a",
	ESPID:      "5A3C-91E7",
	SecureBoot: true,
}

func phases(p plan) []struct {
	name string
	cmds []cmd
} {
	return []struct {
		name string
		cmds []cmd
	}{
		{"partition", p.partition()},
		{"format", p.format()},
		{"subvolumes", p.subvolumes()},
		{"mount", p.mount()},
		{"firstboot", p.firstboot()},
		{"snapshots", p.snapshots()},
	}
}

func TestGolden(t *testing.T) {
	p := testPlan
	var b strings.Builder
	for _, ph := range phases(p) {
		fmt.Fprintf(&b, "# %s\n", ph.name)
		for _, c := range ph.cmds {
			line := strings.Join(c.Args, " ")
			if c.Key {
				line += " <passphrase"
			}
			fmt.Fprintln(&b, line)
		}
	}
	fmt.Fprintln(&b, "# files")
	for _, f := range p.files() {
		fmt.Fprintf(&b, "%04o %s\n", f.Mode, f.Path)
		golden(t, filepath.Join("testdata", "plan", "root", f.Path), []byte(f.Data))
	}
	golden(t, filepath.Join("testdata", "plan", "commands.golden"), []byte(b.String()))
}

func TestAgreement(t *testing.T) {
	p := testPlan
	files := map[string]string{}
	for _, f := range p.files() {
		files[f.Path] = f.Data
	}

	subvols := map[string]string{}
	opts := map[string]string{}
	var espDev string
	for line := range strings.Lines(files["etc/fstab"]) {
		f := strings.Fields(line)
		if len(f) != 6 {
			t.Fatalf("fstab line %q: want 6 fields", line)
		}
		switch f[2] {
		case "btrfs":
			if f[0] != "UUID="+p.RootID {
				t.Errorf("fstab %s: source %s, want root UUID %s", f[1], f[0], p.RootID)
			}
			subvols[f[1]] = option(f[3], "subvol")
			opts[f[1]] = f[3]
		case "vfat":
			if f[0] != "UUID="+p.ESPID || f[1] != esp {
				t.Errorf("fstab ESP line %q: want UUID=%s on %s", line, p.ESPID, esp)
			}
			opts[f[1]] = f[3]
		default:
			t.Errorf("fstab: unexpected filesystem %s", f[2])
		}
	}

	var created []string
	var luksDev, mapper string
	mounted := map[string]string{}
	for _, ph := range phases(p) {
		for _, c := range ph.cmds {
			a := c.Args
			switch {
			case a[0] == "cryptsetup" && a[1] == "luksFormat":
				luksDev = a[len(a)-1]
				if value(a, "--uuid") != p.LUKSID {
					t.Errorf("luksFormat uuid %q, want %s", value(a, "--uuid"), p.LUKSID)
				}
			case a[0] == "cryptsetup" && a[1] == "open":
				mapper = a[len(a)-1]
				if a[len(a)-2] != luksDev {
					t.Errorf("open %s, formatted %s", a[len(a)-2], luksDev)
				}
			case a[0] == "mkfs.btrfs":
				if value(a, "--uuid") != p.RootID || a[len(a)-1] != "/dev/mapper/"+mapper {
					t.Errorf("mkfs.btrfs %v: want uuid %s on /dev/mapper/%s", a, p.RootID, mapper)
				}
			case a[0] == "mkfs.fat":
				espDev = a[len(a)-1]
				if id := a[slices.Index(a, "-i")+1]; id != strings.ReplaceAll(p.ESPID, "-", "") {
					t.Errorf("mkfs.fat volume id %s, want %s", id, p.ESPID)
				}
			case a[0] == "btrfs" && a[1] == "subvolume" && a[2] == "create":
				created = append(created, "/"+filepath.Base(a[3]))
			case a[0] == "mount" && slices.Contains(a, "-o"):
				o := a[slices.Index(a, "-o")+1]
				path := strings.TrimPrefix(a[len(a)-1], target)
				if path == "" {
					path = "/"
				}
				mounted[path] = o
				want := "/dev/mapper/" + mapper
				if path == esp {
					want = espDev
				}
				if a[len(a)-2] != want {
					t.Errorf("mount %s from %s, want %s", path, a[len(a)-2], want)
				}
			}
		}
	}
	if luksDev != p.Disk.part(2) || espDev != p.Disk.part(1) {
		t.Errorf("partitions: luks %s esp %s, disk %s", luksDev, espDev, p.Disk.Path)
	}
	if !maps.Equal(mounted, opts) {
		t.Errorf("install mounts %v differ from fstab %v", mounted, opts)
	}
	if want := slices.Sorted(maps.Values(subvols)); !slices.Equal(slices.Sorted(slices.Values(created)), want) {
		t.Errorf("created subvolumes %v, fstab subvolumes %v", created, want)
	}

	var line string
	for l := range strings.Lines(files["etc/default/limine"]) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "KERNEL_CMDLINE[default]="); ok {
			line = v
		}
	}
	if line == "" {
		t.Fatal("etc/default/limine: no KERNEL_CMDLINE[default]")
	}
	cmdline := map[string]string{}
	for kv := range strings.FieldsSeq(line) {
		k, v, _ := strings.Cut(kv, "=")
		cmdline[k] = v
	}
	if cmdline["rd.luks.name"] != p.LUKSID+"="+mapper {
		t.Errorf("rd.luks.name=%s, want %s=%s", cmdline["rd.luks.name"], p.LUKSID, mapper)
	}
	if !strings.HasPrefix(cmdline["rd.luks.options"], p.LUKSID+"=") {
		t.Errorf("rd.luks.options=%s does not name LUKS UUID %s", cmdline["rd.luks.options"], p.LUKSID)
	}
	if cmdline["root"] != "/dev/mapper/"+mapper {
		t.Errorf("root=%s, want /dev/mapper/%s", cmdline["root"], mapper)
	}
	if got := option(cmdline["rootflags"], "subvol"); got != subvols["/"] {
		t.Errorf("rootflags subvol %s, fstab / subvol %s", got, subvols["/"])
	}

	hooks := strings.Fields(strings.Trim(strings.TrimPrefix(strings.TrimSpace(files["etc/mkinitcpio.conf.d/dotfiles.conf"]), "HOOKS="), "()"))
	at := func(h string) int { return slices.Index(hooks, h) }
	if at("systemd") < 0 || at("udev") >= 0 || at("encrypt") >= 0 || at("btrfs-overlayfs") >= 0 {
		t.Errorf("HOOKS %v: want systemd hooks only", hooks)
	}
	if !(at("block") < at("sd-encrypt") && at("sd-encrypt") < at("filesystems") && at("filesystems") < at("sd-btrfs-overlayfs")) {
		t.Errorf("HOOKS %v: want block < sd-encrypt < filesystems < sd-btrfs-overlayfs", hooks)
	}
}

func option(opts, key string) string {
	for o := range strings.SplitSeq(opts, ",") {
		if k, v, ok := strings.Cut(o, "="); ok && k == key {
			return v
		}
	}
	return ""
}

func value(args []string, name string) string {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, name+"="); ok {
			return v
		}
	}
	return ""
}

func TestNew(t *testing.T) {
	p := newPlan(disk{Path: "/dev/vda"}, "UTC")
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuid.MatchString(p.LUKSID) || !uuid.MatchString(p.RootID) || p.LUKSID == p.RootID {
		t.Errorf("LUKS %s, root %s: want two distinct v4 UUIDs", p.LUKSID, p.RootID)
	}
	if !regexp.MustCompile(`^[0-9A-F]{4}-[0-9A-F]{4}$`).MatchString(p.ESPID) {
		t.Errorf("ESP %s: want a FAT volume ID", p.ESPID)
	}
}

func TestValid(t *testing.T) {
	s, _ := setup(t)
	ok := iso.Answers{Password: "pw", Zone: "America/Denver", LUKS: "luks", Serial: "24123A800123"}
	if err := s.valid(ok); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []func(*iso.Answers){
		func(a *iso.Answers) { a.Zone = "Mars/Olympus" },
		func(a *iso.Answers) { a.Zone = "../../etc/passwd" },
		func(a *iso.Answers) { a.Zone = "Local" },
		func(a *iso.Answers) { a.Zone = "" },
		func(a *iso.Answers) { a.LUKS = "" },
	} {
		a := ok
		bad(&a)
		if err := s.valid(a); err == nil {
			t.Errorf("valid(%+v) accepted invalid answers", a)
		}
	}
}
