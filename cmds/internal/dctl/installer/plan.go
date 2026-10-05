package installer

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	target  = "/mnt"
	mapper  = "root"
	login   = "cullyn"
	machine = "costello"

	espSize    = "+4G"
	options    = "noatime,compress=zstd"
	esp        = "/boot"
	espOptions = "noatime,fmask=0077,dmask=0077"
)

type subvolume struct{ name, path string }

var (
	rootfs    = subvolume{"@", "/"}
	mounts    = []subvolume{rootfs, {"@home", "/home"}, {"@log", "/var/log"}, {"@pkg", "/var/cache/pacman/pkg"}}
	snapshots = subvolume{"@snapshots", "/.snapshots"}
)

func (s subvolume) options() string { return options + ",subvol=/" + s.name }

type plan struct {
	Disk   disk
	Zone   string
	LUKSID string
	RootID string
	ESPID  string
}

func newPlan(d disk, zone string) plan {
	return plan{Disk: d, Zone: zone, LUKSID: uuid(), RootID: uuid(), ESPID: fatID()}
}

func uuid() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func fatID() string {
	var b [4]byte
	rand.Read(b[:])
	return fmt.Sprintf("%X-%X", b[0:2], b[2:4])
}

type cmd struct {
	Args []string
	Key  bool
}

func run(args ...string) cmd { return cmd{Args: args} }

func (p plan) partition() []cmd {
	return []cmd{
		run("wipefs", "--all", "--force", p.Disk.Path),
		run("sgdisk", "--new=1:0:"+espSize, "--typecode=1:ef00", "--new=2:0:0", "--typecode=2:8309", p.Disk.Path),
		run("udevadm", "settle"),
	}
}

func (p plan) format() []cmd {
	luks := p.Disk.part(2)
	return []cmd{
		{Args: []string{"cryptsetup", "luksFormat", "--type=luks2", "--batch-mode", "--uuid=" + p.LUKSID, "--key-file=-", luks}, Key: true},
		{Args: []string{"cryptsetup", "open", "--allow-discards", "--persistent", "--key-file=-", luks, mapper}, Key: true},
		run("mkfs.fat", "-F", "32", "-i", strings.ReplaceAll(p.ESPID, "-", ""), p.Disk.part(1)),
		run("mkfs.btrfs", "--uuid="+p.RootID, "/dev/mapper/"+mapper),
	}
}

func (p plan) subvolumes() []cmd {
	cmds := []cmd{run("mount", "/dev/mapper/"+mapper, target)}
	for _, s := range append(mounts, snapshots) {
		cmds = append(cmds, run("btrfs", "subvolume", "create", filepath.Join(target, s.name)))
	}
	return append(cmds, run("umount", target))
}

func (p plan) mount() []cmd {
	var cmds []cmd
	for _, s := range mounts {
		cmds = append(cmds, run("mount", "--mkdir", "-o", s.options(), "/dev/mapper/"+mapper, filepath.Join(target, s.path)))
	}
	return append(cmds, run("mount", "--mkdir", "-o", espOptions, p.Disk.part(1), filepath.Join(target, esp)))
}

func (p plan) firstboot() []cmd {
	return []cmd{run(
		"systemd-firstboot", "--root="+target, "--force",
		"--locale=en_US.UTF-8", "--keymap=us", "--timezone="+p.Zone, "--hostname="+machine,
		"--setup-machine-id",
	)}
}

func (p plan) snapshots() []cmd {
	dir := filepath.Join(target, snapshots.path)
	return []cmd{
		run(chroot("snapper", "--no-dbus", "-c", "root", "create-config", "/")...),
		run(chroot("snapper", "--no-dbus", "-c", "root", "set-config",
			"TIMELINE_LIMIT_HOURLY=5", "TIMELINE_LIMIT_DAILY=7", "TIMELINE_LIMIT_WEEKLY=0",
			"TIMELINE_LIMIT_MONTHLY=0", "TIMELINE_LIMIT_YEARLY=0")...),
		run("btrfs", "subvolume", "delete", dir),
		run("mount", "--mkdir", "-o", snapshots.options(), "/dev/mapper/"+mapper, dir),
		run("chmod", "750", dir),
	}
}
