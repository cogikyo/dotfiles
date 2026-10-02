package iso

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	Payload = "/opt/dctl/payload"
	Targets = "/opt/dctl/targets"
	Bundle  = "/opt/dctl/dotfiles.bundle"
	Bin     = "/usr/local/bin"
	Repo    = "dctl"
	Sums    = "SHA256SUMS"
	Label   = "DCTLTEST"
	Greeter = "dctltest-greeter:"

	readers = 4
)

type Answers struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Zone     string `json:"zone"`
	LUKS     string `json:"luks"`
	Serial   string `json:"disk_serial"`
}

type Phase struct {
	Name    string  `json:"name"`
	Seconds float64 `json:"seconds"`
}

type Report struct {
	Event  string  `json:"dctltest"`
	OK     bool    `json:"ok"`
	Error  string  `json:"error,omitempty"`
	Total  float64 `json:"total"`
	Phases []Phase `json:"phases"`
}

type sum struct{ hex, name string }

func parseSums(data []byte) ([]sum, error) {
	var sums []sum
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSuffix(line, "\n")
		n := 2 * sha256.Size
		if len(line) <= n+2 || line[n:n+2] != "  " && line[n:n+2] != " *" {
			return nil, fmt.Errorf("malformed checksum line %q", line)
		}
		sums = append(sums, sum{line[:n], line[n+2:]})
	}
	return sums, nil
}

func formatSums(sums []sum) []byte {
	var b []byte
	for _, s := range sums {
		b = fmt.Appendf(b, "%s  %s\n", s.hex, s.name)
	}
	return b
}

func digest(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func digestAll(ctx context.Context, root *os.Root, files []sum) ([]sum, error) {
	sums := make([]sum, len(files))
	errs := make([]error, len(files))
	slots := make(chan struct{}, readers)
	var wg sync.WaitGroup
	for i, file := range files {
		if ctx.Err() != nil {
			break
		}
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			sums[i].name = file.name
			f, err := root.Open(file.name)
			if err == nil {
				sums[i].hex, err = digest(f)
				f.Close()
			}
			errs[i] = err
		})
	}
	wg.Wait()
	return sums, errors.Join(append(errs, ctx.Err())...)
}

func writeSums(ctx context.Context, dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []sum
	for _, e := range entries {
		if e.Type().IsRegular() {
			files = append(files, sum{name: e.Name()})
		}
	}
	sums, err := digestAll(ctx, root, files)
	if err != nil {
		return err
	}
	return root.WriteFile(Sums, formatSums(sums), 0o644)
}

func VerifyPayload(ctx context.Context, dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := root.ReadFile(Sums)
	if err != nil {
		return err
	}
	want, err := parseSums(data)
	if err != nil {
		return fmt.Errorf("%s: %w", Sums, err)
	}
	if len(want) == 0 {
		return fmt.Errorf("%s lists no files", Sums)
	}
	got, err := digestAll(ctx, root, want)
	if err != nil {
		return fmt.Errorf("payload verification: %w", err)
	}
	var errs []error
	for i, s := range want {
		if got[i].hex != s.hex {
			errs = append(errs, fmt.Errorf("payload file %s does not match its checksum in %s", s.name, Sums))
		}
	}
	return errors.Join(errs...)
}
