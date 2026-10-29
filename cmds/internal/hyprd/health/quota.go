package health

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	qGetQuota  = 0x800007
	usrQuota   = 0
	quotaBlock = 1024 // bhardlimit is in KiB; curspace is in bytes.
)

var quotaMounts = []string{"/tmp", "/dev/shm"}

type dqblk struct {
	bhardlimit, bsoftlimit, curspace  uint64
	ihardlimit, isoftlimit, curinodes uint64
	btime, itime                      uint64
	valid                             uint32
}

type quota struct {
	path        string
	used, limit uint64
}

func (q quota) share() float64 {
	return float64(q.used) / float64(q.limit)
}

func readQuota(path string) (quota, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return quota{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer unix.Close(fd)
	var dq dqblk
	_, _, errno := unix.Syscall6(unix.SYS_QUOTACTL_FD, uintptr(fd), qGetQuota<<8|usrQuota, uintptr(os.Getuid()), uintptr(unsafe.Pointer(&dq)), 0, 0)
	if errno != 0 {
		return quota{}, fmt.Errorf("quotactl_fd %s: %w", path, errno)
	}
	return quota{path: path, used: dq.curspace, limit: dq.bhardlimit * quotaBlock}, nil
}
