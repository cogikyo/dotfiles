package installer

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	Target = "/mnt"
	Mapper = "root"

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

type Plan struct {
	Disk       Disk
	User       string
	Host       string
	Zone       string
	LUKSID     string
	RootID     string
	ESPID      string
	SecureBoot bool
}

var (
	userPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	hostPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
)

func New(d Disk, user, host, zone string) (Plan, error) {
	var errs []error
	if !userPattern.MatchString(user) || user == "root" {
		errs = append(errs, fmt.Errorf("username %q: want a lowercase POSIX name other than root", user))
	}
	if !hostPattern.MatchString(host) {
		errs = append(errs, fmt.Errorf("hostname %q: want one lowercase DNS label", host))
	}
	if _, err := time.LoadLocation(zone); err != nil || zone == "" || zone == "Local" {
		errs = append(errs, fmt.Errorf("timezone %q: not a known zone", zone))
	}
	if err := errors.Join(errs...); err != nil {
		return Plan{}, err
	}
	return Plan{Disk: d, User: user, Host: host, Zone: zone, LUKSID: uuid(), RootID: uuid(), ESPID: fatID()}, nil
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

type Cmd struct {
	Args []string
	Key  bool
}

func run(args ...string) Cmd { return Cmd{Args: args} }

func (p Plan) Partition() []Cmd {
	return []Cmd{
		run("wipefs", "--all", "--force", p.Disk.Path),
		run("sgdisk", "--new=1:0:"+espSize, "--typecode=1:ef00", "--new=2:0:0", "--typecode=2:8309", p.Disk.Path),
		run("udevadm", "settle"),
	}
}

func (p Plan) Format() []Cmd {
	luks := p.Disk.Part(2)
	return []Cmd{
		{Args: []string{"cryptsetup", "luksFormat", "--type=luks2", "--batch-mode", "--uuid=" + p.LUKSID, "--key-file=-", luks}, Key: true},
		{Args: []string{"cryptsetup", "open", "--allow-discards", "--persistent", "--key-file=-", luks, Mapper}, Key: true},
		run("mkfs.fat", "-F", "32", "-i", strings.ReplaceAll(p.ESPID, "-", ""), p.Disk.Part(1)),
		run("mkfs.btrfs", "--uuid="+p.RootID, "/dev/mapper/"+Mapper),
	}
}

func (p Plan) Subvolumes() []Cmd {
	cmds := []Cmd{run("mount", "/dev/mapper/"+Mapper, Target)}
	for _, s := range append(mounts, snapshots) {
		cmds = append(cmds, run("btrfs", "subvolume", "create", filepath.Join(Target, s.name)))
	}
	return append(cmds, run("umount", Target))
}

func (p Plan) Mount() []Cmd {
	var cmds []Cmd
	for _, s := range mounts {
		cmds = append(cmds, run("mount", "--mkdir", "-o", s.options(), "/dev/mapper/"+Mapper, filepath.Join(Target, s.path)))
	}
	return append(cmds, run("mount", "--mkdir", "-o", espOptions, p.Disk.Part(1), filepath.Join(Target, esp)))
}

func (p Plan) Firstboot() []Cmd {
	return []Cmd{run(
		"systemd-firstboot", "--root="+Target, "--force",
		"--locale=en_US.UTF-8", "--keymap=us", "--timezone="+p.Zone, "--hostname="+p.Host,
		"--setup-machine-id",
	)}
}

func (p Plan) Snapshots() []Cmd {
	dir := filepath.Join(Target, snapshots.path)
	return []Cmd{
		run("arch-chroot", Target, "snapper", "--no-dbus", "-c", "root", "create-config", "/"),
		run("arch-chroot", Target, "snapper", "--no-dbus", "-c", "root", "set-config",
			"TIMELINE_LIMIT_HOURLY=5", "TIMELINE_LIMIT_DAILY=7", "TIMELINE_LIMIT_WEEKLY=0",
			"TIMELINE_LIMIT_MONTHLY=0", "TIMELINE_LIMIT_YEARLY=0"),
		run("btrfs", "subvolume", "delete", dir),
		run("mount", "--mkdir", "-o", snapshots.options(), "/dev/mapper/"+Mapper, dir),
		run("chmod", "750", dir),
	}
}
