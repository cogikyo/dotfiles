package health

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

const (
	cpuCores     = 0.5
	memoryGrowth = 1 << 30
	memoryRatio  = 1.5
	memorySteady = 0.7
	memoryLate   = 0.25
	memoryPoints = 4
	restartLoops = 10
	zombieCount  = 3
	journalNoise = 2000
	ramOpen      = 0.80
	ramClear     = 0.75
	ramPoints    = 10
	ramSpacing   = int64((interval - 5*time.Second) / time.Second)
)

func (h *history) findings(ctx context.Context, now time.Time, window time.Duration, procs []proc) ([]Finding, map[string]bool) {
	cut := now.Add(-window).Unix()
	start := slices.IndexFunc(h.Minutes, func(m minute) bool { return m.At > cut })
	var buckets []minute
	if start >= 0 {
		buckets = h.Minutes[start:]
	}

	var found []Finding
	found = append(found, cpuFindings(buckets, procs)...)
	found = append(found, h.memoryFindings(cut)...)
	found = append(found, h.restartFindings(buckets)...)
	found = append(found, h.zombieFindings(now, procs)...)
	found = append(found, journalFindings(buckets)...)
	ram, hold := ramFindings(buckets, procs)
	found = append(found, ram...)

	units, err := failedFindings(ctx)
	found = append(found, units...)
	return found, map[string]bool{"ram": hold, "failed": err != nil}
}

func span(buckets []minute) (int64, time.Duration) {
	var secs int64
	for _, b := range buckets {
		secs += b.Secs
	}
	return secs, (time.Duration(secs) * time.Second).Round(time.Minute)
}

func cpuFindings(buckets []minute, procs []proc) []Finding {
	secs, over := span(buckets)
	if secs == 0 {
		return nil
	}
	ticks := map[string]uint64{}
	for _, b := range buckets {
		for group, n := range b.CPU {
			ticks[group] += n
		}
	}
	var found []Finding
	for _, group := range slices.Sorted(maps.Keys(ticks)) {
		cpuSecs := float64(ticks[group]) / userHZ
		cores := cpuSecs / float64(secs)
		if cores < cpuCores {
			continue
		}
		var busy []string
		for _, p := range procs {
			if p.group == group {
				busy = append(busy, fmt.Sprintf("%d %s", p.pid, p.comm))
			}
		}
		found = append(found, Finding{
			Check:    "cpu",
			Subject:  group,
			Summary:  fmt.Sprintf("%s averaged %.2f cores over %s", group, cores, over),
			Evidence: fmt.Sprintf("%.0f CPU-seconds in %s; busy in the last sample: %s", cpuSecs, over, list(busy, "none")),
		})
	}
	return found
}

func (h *history) memoryFindings(cut int64) []Finding {
	var found []Finding
	for _, key := range slices.Sorted(maps.Keys(h.Memory)) {
		s := h.Memory[key]
		i := slices.IndexFunc(s.Points, func(p point) bool { return p.At > cut })
		if i < 0 || len(s.Points)-i < memoryPoints {
			continue
		}
		pts := s.Points[i:]
		first, last := int64(pts[0].RSS), int64(pts[len(pts)-1].RSS)
		net := last - first
		if net < memoryGrowth || float64(last) < memoryRatio*float64(first) {
			continue
		}
		var up int64
		for j := 1; j < len(pts); j++ {
			if d := int64(pts[j].RSS) - int64(pts[j-1].RSS); d > 0 {
				up += d
			}
		}
		if float64(net) < memorySteady*float64(up) {
			continue
		}
		midAt := (pts[0].At + pts[len(pts)-1].At) / 2
		m := slices.IndexFunc(pts, func(p point) bool { return p.At > midAt })
		if float64(last-int64(pts[max(m-1, 0)].RSS)) < memoryLate*float64(net) {
			continue
		}
		over := time.Duration(pts[len(pts)-1].At-pts[0].At) * time.Second
		subject := fmt.Sprintf("%s (pid %d)", s.Exe, s.Pid)
		found = append(found, Finding{
			Check:   "memory",
			Subject: subject,
			Summary: fmt.Sprintf("%s grew from %s to %s RSS over %s", subject, gib(first), gib(last), over),
			Evidence: fmt.Sprintf("%d samples, +%.0f%%, %.0f%% of the rise kept; cmdline: %s",
				len(pts), 100*float64(net)/float64(max(first, 1)), 100*float64(net)/float64(up), cmdline(s.Pid)),
		})
	}
	return found
}

func ramFindings(buckets []minute, procs []proc) ([]Finding, bool) {
	var obs []minute
	for _, b := range slices.Backward(buckets) {
		if b.RAM > 0 && (len(obs) == 0 || obs[len(obs)-1].At-b.At >= ramSpacing) {
			obs = append(obs, b)
		}
	}
	slices.Reverse(obs)
	samples := make([]float64, len(obs))
	for i, o := range obs {
		samples[i] = o.RAM
	}
	if len(samples) < ramPoints {
		return nil, true
	}
	recent := samples[len(samples)-ramPoints:]
	if slices.Min(recent) < ramOpen {
		return nil, slices.Max(recent) >= ramClear
	}
	mi, err := readMeminfo()
	if err != nil {
		return nil, true
	}
	run := len(samples) - 1
	for run > 0 && samples[run-1] >= ramOpen {
		run--
	}
	over := (time.Duration(obs[len(obs)-1].At-obs[run].At) * time.Second).Round(time.Minute)
	summary, evidence := ramReport(mi, procs, samples, over)
	return []Finding{{Check: "ram", Subject: "system", Summary: summary, Evidence: evidence}}, false
}

func (h *history) restartFindings(buckets []minute) []Finding {
	_, over := span(buckets)
	counts := map[string]int{}
	for _, b := range buckets {
		for unit, n := range b.Restarts {
			counts[unit] += n
		}
	}
	var found []Finding
	for _, unit := range slices.Sorted(maps.Keys(counts)) {
		if counts[unit] < restartLoops {
			continue
		}
		found = append(found, Finding{
			Check:    "restarts",
			Subject:  unit,
			Summary:  fmt.Sprintf("%s restarted %d times in %s", unit, counts[unit], over),
			Evidence: fmt.Sprintf("NRestarts is now %d", h.Restarts[unit]),
		})
	}
	return found
}

func (h *history) zombieFindings(now time.Time, procs []proc) []Finding {
	comms := map[int]string{}
	children := map[int][]string{}
	for _, p := range procs {
		comms[p.pid] = p.comm
		if z, ok := h.Zombies[p.key]; ok && z.Since < now.Unix() {
			children[z.PPID] = append(children[z.PPID], fmt.Sprintf("%d %s", p.pid, p.comm))
		}
	}
	var found []Finding
	for _, ppid := range slices.Sorted(maps.Keys(children)) {
		kids := children[ppid]
		if len(kids) < zombieCount {
			continue
		}
		subject := fmt.Sprintf("%s (pid %d)", cmp.Or(comms[ppid], "?"), ppid)
		found = append(found, Finding{
			Check:    "zombies",
			Subject:  subject,
			Summary:  fmt.Sprintf("%s holds %d defunct children", subject, len(kids)),
			Evidence: fmt.Sprintf("unreaped for at least one sample: %s", list(kids, "")),
		})
	}
	return found
}

func journalFindings(buckets []minute) []Finding {
	_, over := span(buckets)
	counts := map[string]int{}
	for _, b := range buckets {
		for unit, n := range b.Lines {
			counts[unit] += n
		}
	}
	var found []Finding
	for _, unit := range slices.Sorted(maps.Keys(counts)) {
		if counts[unit] < journalNoise {
			continue
		}
		scope, name, _ := strings.Cut(unit, ":")
		found = append(found, Finding{
			Check:    "journal",
			Subject:  unit,
			Summary:  fmt.Sprintf("%s logged %d journal lines in %s", unit, counts[unit], over),
			Evidence: fmt.Sprintf("journalctl %s-u %s --since=-%s", userFlag(scope), name, over),
		})
	}
	return found
}

func failedFindings(ctx context.Context) ([]Finding, error) {
	var found []Finding
	for _, scope := range scopes {
		units, err := failedUnits(ctx, scope)
		if err != nil {
			return nil, err
		}
		for _, u := range units {
			subject := scope + ":" + u.Unit
			found = append(found, Finding{
				Check:    "failed",
				Subject:  subject,
				Summary:  fmt.Sprintf("%s is %s", subject, u.Sub),
				Evidence: fmt.Sprintf("%s; systemctl %sstatus %s", u.Description, userFlag(scope), u.Unit),
			})
		}
	}
	return found, nil
}

func gib(bytes int64) string {
	return fmt.Sprintf("%.2f GiB", float64(bytes)/(1<<30))
}

func list(items []string, empty string) string {
	if len(items) == 0 {
		return empty
	}
	if len(items) > 8 {
		items = append(items[:8:8], fmt.Sprintf("… %d more", len(items)-8))
	}
	return strings.Join(items, ", ")
}

func userFlag(scope string) string {
	if scope == "user" {
		return "--user "
	}
	return ""
}
