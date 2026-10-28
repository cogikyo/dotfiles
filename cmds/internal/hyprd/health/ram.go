package health

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type meminfo struct {
	total, available, shmem, swapTotal, swapFree uint64
}

func readMeminfo() (meminfo, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return meminfo{}, err
	}
	sizes := map[string]uint64{}
	for line := range strings.Lines(string(data)) {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if kb, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64); err == nil {
			sizes[key] = kb << 10
		}
	}
	if _, ok := sizes["MemAvailable"]; !ok || sizes["MemTotal"] == 0 {
		return meminfo{}, errors.New("/proc/meminfo lacks MemTotal or MemAvailable")
	}
	return meminfo{
		total:     sizes["MemTotal"],
		available: sizes["MemAvailable"],
		shmem:     sizes["Shmem"],
		swapTotal: sizes["SwapTotal"],
		swapFree:  sizes["SwapFree"],
	}, nil
}

func (mi meminfo) used() float64 {
	return float64(mi.total-mi.available) / float64(mi.total)
}

type consumer struct {
	comm      string
	count     int
	rss, swap uint64
}

func consumers(procs []proc) []*consumer {
	by := map[string]*consumer{}
	for _, p := range procs {
		if p.state == 'Z' {
			continue
		}
		c := by[p.comm]
		if c == nil {
			c = &consumer{comm: p.comm}
			by[p.comm] = c
		}
		c.count++
		c.rss += p.rss
		c.swap += vmSwap(p.pid)
	}
	return slices.SortedFunc(maps.Values(by), func(a, b *consumer) int {
		return cmp.Or(cmp.Compare(b.rss+b.swap, a.rss+a.swap), strings.Compare(a.comm, b.comm))
	})
}

type tmpfs struct {
	path       string
	used, size uint64
}

var unmangle = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)

func tmpfsMounts() []tmpfs {
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return nil
	}
	var mounts []tmpfs
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "tmpfs" {
			continue
		}
		path := unmangle.Replace(fields[1])
		var st syscall.Statfs_t
		if syscall.Statfs(path, &st) != nil {
			continue
		}
		block := uint64(st.Bsize)
		if used := (st.Blocks - st.Bfree) * block; used > 0 {
			mounts = append(mounts, tmpfs{path: path, used: used, size: st.Blocks * block})
		}
	}
	slices.SortFunc(mounts, func(a, b tmpfs) int { return cmp.Compare(b.used, a.used) })
	return mounts
}

func ramReport(mi meminfo, procs []proc, samples []float64, over time.Duration) (summary, evidence string) {
	top := consumers(procs)
	mounts := tmpfsMounts()
	swap := mi.swapTotal - mi.swapFree

	brief := make([]string, 0, 3)
	for _, c := range top[:min(len(top), 3)] {
		brief = append(brief, fmt.Sprintf("%s ×%d %s", c.comm, c.count, gib(int64(c.rss+c.swap))))
	}
	var largest []string
	for _, m := range mounts[:min(len(mounts), 2)] {
		largest = append(largest, fmt.Sprintf("%s %s", m.path, gib(int64(m.used))))
	}
	summary = fmt.Sprintf("RAM %.0f%% used (%s of %s), at or above %.0f%% for %s; swap %s used; top by RSS+swap: %s; largest tmpfs: %s",
		100*mi.used(), gib(int64(mi.total-mi.available)), gib(int64(mi.total)), 100*ramOpen, over, gib(int64(swap)),
		list(brief, "none"), list(largest, "none"))

	full := make([]string, len(top))
	for i, c := range top {
		full[i] = fmt.Sprintf("%s ×%d %s RSS + %s swap", c.comm, c.count, gib(int64(c.rss)), gib(int64(c.swap)))
	}
	used := make([]string, len(mounts))
	for i, m := range mounts {
		used[i] = fmt.Sprintf("%s %s of %s", m.path, gib(int64(m.used)), gib(int64(m.size)))
	}
	percents := make([]string, len(samples))
	for i, s := range samples {
		percents[i] = fmt.Sprintf("%.0f%%", 100*s)
	}
	evidence = fmt.Sprintf("used = (MemTotal - MemAvailable) / MemTotal; MemAvailable %s, Shmem %s, swap %s of %s; samples this window, oldest first: %s; by command (processes, summed RSS double-counts shared pages): %s; tmpfs used of size: %s",
		gib(int64(mi.available)), gib(int64(mi.shmem)), gib(int64(swap)), gib(int64(mi.swapTotal)),
		strings.Join(percents, " "), list(full, "none"), list(used, "none"))
	return summary, evidence
}
