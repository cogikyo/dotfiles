package machine

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	minCores = 4
	kernels  = "/usr/lib/modules"
)

var passthrough = []string{"vfio_pci", "pci_stub"}

type Facts struct {
	Vendor    string
	Product   string
	Board     string
	CPU       string
	GPUs      []string
	Battery   bool
	Backlight bool
	WLAN      []string
	Cores     int
	Lane      []int
}

type sysfs string

func (r sysfs) path(p ...string) string {
	return filepath.Join(append([]string{string(r)}, p...)...)
}

func (r sysfs) read(p ...string) string {
	data, _ := os.ReadFile(r.path(p...))
	return strings.TrimSpace(string(data))
}

func (r sysfs) glob(pattern string) []string {
	matches, _ := filepath.Glob(r.path(pattern))
	return matches
}

func Detect(root string) (Facts, error) {
	r := sysfs(root)
	wlan, err := r.wlan()
	if err != nil {
		return Facts{}, err
	}
	f := Facts{
		Vendor:    r.read("/sys/class/dmi/id/sys_vendor"),
		Product:   r.read("/sys/class/dmi/id/product_name"),
		Board:     r.read("/sys/class/dmi/id/board_name"),
		CPU:       r.cpu(),
		GPUs:      r.gpus(),
		Battery:   r.battery(),
		Backlight: len(r.glob("/sys/class/backlight/*")) > 0,
		WLAN:      wlan,
	}
	f.Cores, f.Lane = r.lane()
	return f, nil
}

func (f Facts) String() string {
	parts := []string{strings.TrimSpace(f.Vendor + " " + f.Product)}
	parts = append(parts, "cpu "+cmp.Or(f.CPU, "unknown"), "gpu "+cmp.Or(strings.Join(f.GPUs, ","), "unknown"))
	parts = append(parts, flag(f.Battery, Laptop, Desktop), fmt.Sprintf("%d cores", f.Cores))
	if len(f.Lane) > 0 {
		parts = append(parts, "lane "+cpus(f.Lane, ","))
	}
	if len(f.WLAN) > 0 {
		parts = append(parts, "Wi-Fi "+strings.Join(f.WLAN, ","))
	}
	if f.Backlight {
		parts = append(parts, "backlight")
	}
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), " · ")
}

func (r sysfs) cpu() string {
	file, err := os.Open(r.path("/proc/cpuinfo"))
	if err != nil {
		return ""
	}
	defer file.Close()
	s := bufio.NewScanner(file)
	for s.Scan() {
		key, value, ok := strings.Cut(s.Text(), ":")
		if !ok || strings.TrimSpace(key) != "vendor_id" {
			continue
		}
		switch strings.TrimSpace(value) {
		case "AuthenticAMD":
			return AMD
		case "GenuineIntel":
			return Intel
		}
		return ""
	}
	return ""
}

var gpuVendors = map[string]string{"0x1002": AMD, "0x8086": Intel, "0x10de": NVIDIA}

func (r sysfs) gpus() []string {
	var out []string
	for _, card := range r.glob("/sys/class/drm/card[0-9]*") {
		if strings.Contains(filepath.Base(card), "-") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(card, "device", "vendor"))
		if v, ok := gpuVendors[strings.TrimSpace(string(data))]; ok && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}

func (r sysfs) battery() bool {
	for _, supply := range r.glob("/sys/class/power_supply/*") {
		kind, _ := os.ReadFile(filepath.Join(supply, "type"))
		scope, _ := os.ReadFile(filepath.Join(supply, "scope"))
		if strings.TrimSpace(string(kind)) == "Battery" && strings.TrimSpace(string(scope)) != "Device" {
			return true
		}
	}
	return false
}

func (r sysfs) wlan() ([]string, error) {
	var out []string
	add := func(m string) {
		if m != "" && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	for _, w := range r.glob("/sys/class/net/*/wireless") {
		add(driver(filepath.Join(filepath.Dir(w), "device")))
	}
	var aliases [][2]string
	for _, dev := range r.glob("/sys/bus/pci/devices/*") {
		class, _ := os.ReadFile(filepath.Join(dev, "class"))
		if !strings.HasPrefix(string(class), "0x0280") {
			continue
		}
		if m := driver(dev); m != "" && !slices.Contains(passthrough, strings.ReplaceAll(m, "-", "_")) {
			add(m)
			continue
		}
		if aliases == nil {
			var err error
			if aliases, err = r.aliases(); err != nil {
				return nil, err
			}
		}
		data, _ := os.ReadFile(filepath.Join(dev, "modalias"))
		modalias := strings.TrimSpace(string(data))
		for _, a := range aliases {
			if ok, _ := path.Match(a[0], modalias); ok {
				add(a[1])
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

func driver(dev string) string {
	if link, err := os.Readlink(filepath.Join(dev, "driver", "module")); err == nil {
		return filepath.Base(link)
	}
	if link, err := os.Readlink(filepath.Join(dev, "driver")); err == nil {
		return filepath.Base(link)
	}
	return ""
}

func (r sysfs) aliases() ([][2]string, error) {
	out := [][2]string{}
	for _, name := range r.glob(kernels + "/*/modules.alias") {
		file, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		s := bufio.NewScanner(file)
		for s.Scan() {
			fields := strings.Fields(s.Text())
			if len(fields) == 3 && fields[0] == "alias" && strings.HasPrefix(fields[1], "pci:") {
				out = append(out, [2]string{fields[1], fields[2]})
			}
		}
		if err := errors.Join(s.Err(), file.Close()); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no PCI module aliases in %s", r.path(kernels, "*", "modules.alias"))
	}
	return out, nil
}

func (r sysfs) lane() (int, []int) {
	type core struct{ pkg, id int }
	threads := map[core][]int{}
	for _, dir := range r.glob("/sys/devices/system/cpu/cpu[0-9]*") {
		n, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(dir), "cpu"))
		if err != nil {
			continue
		}
		pkg, perr := strconv.Atoi(sysfs(dir).read("topology", "physical_package_id"))
		id, cerr := strconv.Atoi(sysfs(dir).read("topology", "core_id"))
		if perr != nil || cerr != nil {
			continue
		}
		threads[core{pkg, id}] = append(threads[core{pkg, id}], n)
	}
	if len(threads) < minCores {
		return len(threads), nil
	}
	last := slices.MaxFunc(slices.Collect(maps.Keys(threads)), func(a, b core) int {
		return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.id, b.id))
	})
	lane := threads[last]
	slices.Sort(lane)
	return len(threads), lane
}
