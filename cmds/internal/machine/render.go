package machine

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

const lastCPU = 1023

type File struct {
	Path string
	Data []byte
}

var lanes = []struct {
	scope, unit string
	weight      int
}{
	{"system", "bluetooth", 1000},
	{"system", "rtkit-daemon", 1000},
	{"user", "pipewire", 1000},
	{"user", "pipewire-pulse", 1000},
	{"user", "wireplumber", 1000},
	{"user", "hyprd", 500},
	{"user", "ewwd", 500},
}

func (p Profile) Files() []File {
	var manager, blacklist []byte
	if len(p.Lane) > 0 {
		manager = []byte("[Manager]\nCPUAffinity=" + strings.Join(pool(p.Lane), " ") + "\n")
	}
	if !p.WiFi {
		for _, m := range p.WLAN {
			blacklist = append(blacklist, "blacklist "+m+"\n"...)
		}
	}
	files := []File{
		{"etc/systemd/system.conf.d/cpu-lanes.conf", manager},
		{"etc/modprobe.d/blacklist-wifi.conf", blacklist},
	}
	for _, l := range lanes {
		var service []byte
		if len(p.Lane) > 0 {
			service = fmt.Appendf(nil, "[Service]\nCPUAffinity=%s\nCPUWeight=%d\n", cpus(p.Lane, " "), l.weight)
		}
		files = append(files, File{"etc/systemd/" + l.scope + "/" + l.unit + ".service.d/cpu-lane.conf", service})
	}
	return files
}

func Pooled(name string, args ...string) *exec.Cmd {
	p, err := Read(Path)
	if err != nil || len(p.Lane) == 0 {
		return exec.Command(name, args...)
	}
	return exec.Command("taskset", slices.Concat([]string{"-c", strings.Join(pool(p.Lane), ","), name}, args)...)
}

func pool(lane []int) []string {
	if len(lane) == 0 {
		return nil
	}
	var spans []string
	span := func(a, b int) {
		if a == b {
			spans = append(spans, strconv.Itoa(a))
		} else if a < b {
			spans = append(spans, strconv.Itoa(a)+"-"+strconv.Itoa(b))
		}
	}
	start := 0
	for _, c := range slices.Sorted(slices.Values(lane)) {
		span(start, c-1)
		start = max(start, c+1)
	}
	span(start, lastCPU)
	return spans
}
