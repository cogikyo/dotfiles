package health

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	userHZ     = 100
	kthreadBit = 0x00200000
)

type proc struct {
	key   string
	pid   int
	ppid  int
	comm  string
	state byte
	ticks uint64
	start int64
	rss   uint64
	group string
}

func readProcs() ([]proc, error) {
	boot, err := bootTime()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	page := uint64(os.Getpagesize())
	procs := make([]proc, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		p, ok := parseStat(pid, data, boot, page)
		if ok {
			procs = append(procs, p)
		}
	}
	return procs, nil
}

func parseStat(pid int, data []byte, boot int64, page uint64) (proc, bool) {
	lp := bytes.IndexByte(data, '(')
	rp := bytes.LastIndexByte(data, ')')
	if lp < 0 || rp < lp {
		return proc{}, false
	}
	fields := strings.Fields(string(data[rp+1:]))
	if len(fields) < 22 {
		return proc{}, false
	}
	field := func(n int) uint64 {
		v, _ := strconv.ParseUint(fields[n-3], 10, 64)
		return v
	}
	if field(9)&kthreadBit != 0 {
		return proc{}, false
	}
	start := field(22)
	return proc{
		key:   fmt.Sprintf("%d:%d", pid, start),
		pid:   pid,
		ppid:  int(field(4)),
		comm:  string(data[lp+1 : rp]),
		state: fields[0][0],
		ticks: field(14) + field(15),
		start: boot + int64(start/userHZ),
		rss:   field(24) * page,
	}, true
}

func bootTime() (int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	for line := range strings.Lines(string(data)) {
		if rest, ok := strings.CutPrefix(line, "btime "); ok {
			return strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		}
	}
	return 0, fmt.Errorf("/proc/stat has no btime")
}

func (p *proc) exe() string {
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", p.pid))
	if err != nil {
		return "(" + p.comm + ")"
	}
	return strings.TrimSuffix(exe, " (deleted)")
}

func (p *proc) resolveGroup() string {
	if p.group == "" {
		p.group = p.exe()
		if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", p.pid)); err == nil {
			p.group += " in " + cwd
		}
	}
	return p.group
}

func vmSwap(pid int) uint64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for line := range strings.Lines(string(data)) {
		if rest, ok := strings.CutPrefix(line, "VmSwap:"); ok {
			kb, _ := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
			return kb << 10
		}
	}
	return 0
}

func cmdline(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.ReplaceAll(string(data), "\x00", " "))
	if len(line) > 200 {
		line = line[:200] + "…"
	}
	return line
}
