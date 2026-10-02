package installer

import (
	"fmt"
	"io/fs"
	"strings"
)

type File struct {
	Path string
	Mode fs.FileMode
	Data string
}

func (p Plan) Files() []File {
	return []File{
		{"etc/fstab", 0o644, p.fstab()},
		{"etc/default/limine", 0o644, p.limine()},
		{"etc/mkinitcpio.conf.d/dotfiles.conf", 0o644, "HOOKS=(base systemd autodetect microcode modconf kms keyboard sd-vconsole block sd-encrypt filesystems sd-btrfs-overlayfs fsck)\n"},
		{"etc/locale.gen", 0o644, "en_US.UTF-8 UTF-8\n"},
		{"etc/sudoers.d/wheel", 0o440, "%wheel ALL=(ALL:ALL) ALL\n"},
	}
}

func (p Plan) Cmdline() string {
	return fmt.Sprintf("rd.luks.name=%[1]s=%[2]s rd.luks.options=%[1]s=fido2-device=auto root=/dev/mapper/%[2]s rootflags=subvol=/%[3]s rw", p.LUKSID, Mapper, rootfs.name)
}

func (p Plan) fstab() string {
	var b strings.Builder
	for _, s := range append(mounts, snapshots) {
		fmt.Fprintf(&b, "UUID=%s\t%s\tbtrfs\t%s\t0 0\n", p.RootID, s.path, s.options())
	}
	fmt.Fprintf(&b, "UUID=%s\t%s\tvfat\t%s\t0 2\n", p.ESPID, esp, espOptions)
	return b.String()
}

func (p Plan) limine() string {
	lines := []string{
		"ESP_PATH=" + esp,
		"KERNEL_CMDLINE[default]=" + p.Cmdline(),
		"ENABLE_UKI=yes",
		"SNAPPER_CONFIG_NAME=root",
		"RESTORE_METHOD=replace",
	}
	if p.SecureBoot {
		lines = append(lines, "ENABLE_ENROLL_LIMINE_CONFIG=yes", "ENABLE_LIMINE_FALLBACK=no")
	}
	return strings.Join(lines, "\n") + "\n"
}
