package machine

import (
	"cmp"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const Path = "/etc/dctl/machine"

const (
	Framework = "framework"
	Custom    = "custom"

	AMD    = "amd"
	Intel  = "intel"
	NVIDIA = "nvidia"

	Desktop = "desktop"
	Laptop  = "laptop"
)

type Profile struct {
	Preset    string
	CPU       string
	GPUs      []string
	Form      string
	Backlight bool
	WiFi      bool
	WLAN      []string
	YubiKeys  int
	Lane      []int
}

var module = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (p Profile) Validate() error {
	var errs []error
	if p.Preset != Framework && p.Preset != Custom {
		errs = append(errs, fmt.Errorf("preset %q: want %s or %s", p.Preset, Framework, Custom))
	}
	if p.CPU != AMD && p.CPU != Intel {
		errs = append(errs, fmt.Errorf("cpu %q: want %s or %s", p.CPU, AMD, Intel))
	}
	for _, g := range p.GPUs {
		if g != AMD && g != Intel && g != NVIDIA {
			errs = append(errs, fmt.Errorf("gpu %q: want %s, %s, or %s", g, AMD, Intel, NVIDIA))
		}
	}
	if p.Form != Desktop && p.Form != Laptop {
		errs = append(errs, fmt.Errorf("form %q: want %s or %s", p.Form, Desktop, Laptop))
	}
	for _, m := range p.WLAN {
		if !module.MatchString(m) {
			errs = append(errs, fmt.Errorf("wlan module %q", m))
		}
	}
	if p.YubiKeys < 0 || p.YubiKeys > 2 {
		errs = append(errs, fmt.Errorf("yubikeys %d: want 0, 1, or 2", p.YubiKeys))
	}
	for _, c := range p.Lane {
		if c < 0 || c > lastCPU {
			errs = append(errs, fmt.Errorf("lane CPU %d: want 0-%d", c, lastCPU))
		}
	}
	return errors.Join(errs...)
}

func (p Profile) Vendors() []string {
	out := []string{p.CPU}
	for _, g := range p.GPUs {
		if !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	return out
}

func (p Profile) String() string {
	return strings.Join([]string{
		p.Preset,
		"cpu " + p.CPU,
		"gpu " + cmp.Or(strings.Join(p.GPUs, ","), "none"),
		p.Form,
		"Wi-Fi " + flag(p.WiFi, "on", "off"),
		fmt.Sprintf("%d YubiKeys", p.YubiKeys),
	}, " · ")
}

func (p Profile) Format() []byte {
	var b strings.Builder
	for _, kv := range [][2]string{
		{"preset", p.Preset},
		{"cpu", p.CPU},
		{"gpus", strings.Join(p.GPUs, ",")},
		{"form", p.Form},
		{"backlight", flag(p.Backlight, "yes", "no")},
		{"wifi", flag(p.WiFi, "on", "off")},
		{"wlan", strings.Join(p.WLAN, ",")},
		{"yubikeys", strconv.Itoa(p.YubiKeys)},
		{"lane", cpus(p.Lane, ",")},
	} {
		fmt.Fprintf(&b, "%s=%s\n", kv[0], kv[1])
	}
	return []byte(b.String())
}

func Parse(data []byte) (Profile, error) {
	var p Profile
	seen := map[string]bool{}
	var errs []error
	for n, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			errs = append(errs, fmt.Errorf("line %d: want key=value, got %q", n+1, line))
			continue
		}
		if seen[key] {
			errs = append(errs, fmt.Errorf("line %d: duplicate %s", n+1, key))
		}
		seen[key] = true
		var err error
		switch key {
		case "preset":
			p.Preset = value
		case "cpu":
			p.CPU = value
		case "gpus":
			p.GPUs = list(value)
		case "form":
			p.Form = value
		case "backlight":
			p.Backlight, err = parseFlag(value, "yes", "no")
		case "wifi":
			p.WiFi, err = parseFlag(value, "on", "off")
		case "wlan":
			p.WLAN = list(value)
		case "yubikeys":
			p.YubiKeys, err = strconv.Atoi(value)
		case "lane":
			p.Lane, err = parseCPUs(value)
		default:
			err = errors.New("unknown key")
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %s: %w", n+1, key, err))
		}
	}
	for _, key := range []string{"preset", "cpu", "gpus", "form", "backlight", "wifi", "wlan", "yubikeys", "lane"} {
		if !seen[key] {
			errs = append(errs, fmt.Errorf("missing %s", key))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return p, err
	}
	return p, p.Validate()
}

func Read(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Profile{}, fmt.Errorf("no machine profile at %s; run dctl setup machine: %w", path, err)
	}
	if err != nil {
		return Profile{}, err
	}
	p, err := Parse(data)
	if err != nil {
		return p, fmt.Errorf("invalid machine profile at %s; run dctl setup machine: %w", path, err)
	}
	return p, nil
}

func Write(path string, p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "."+filepath.Base(path)+"-"+rand.Text()[:8])
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(p.Format())
	err = errors.Join(err, f.Chmod(0o644), f.Sync(), f.Close())
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

func flag(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

func parseFlag(v, yes, no string) (bool, error) {
	switch v {
	case yes:
		return true, nil
	case no:
		return false, nil
	}
	return false, fmt.Errorf("%q: want %s or %s", v, yes, no)
}

func list(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

func cpus(lane []int, sep string) string {
	out := make([]string, len(lane))
	for i, c := range lane {
		out[i] = strconv.Itoa(c)
	}
	return strings.Join(out, sep)
}

func parseCPUs(v string) ([]int, error) {
	var out []int
	for _, s := range list(v) {
		c, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
