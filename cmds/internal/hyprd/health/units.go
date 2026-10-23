package health

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var scopes = []string{"system", "user"}

func systemctl(ctx context.Context, scope string, args ...string) *exec.Cmd {
	if scope == "user" {
		args = append([]string{"--user"}, args...)
	}
	return exec.CommandContext(ctx, "systemctl", args...)
}

func restartCounts(ctx context.Context, scope string) (map[string]int, error) {
	out, err := systemctl(ctx, scope, "show", "--type=service", "-p", "Id", "-p", "NRestarts", "*").Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl %s show: %w", scope, err)
	}
	counts := map[string]int{}
	for block := range strings.SplitSeq(string(out), "\n\n") {
		var id string
		n := -1
		for line := range strings.Lines(block) {
			key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
			switch key {
			case "Id":
				id = value
			case "NRestarts":
				n, _ = strconv.Atoi(value)
			}
		}
		if id != "" && n >= 0 {
			counts[scope+":"+id] = n
		}
	}
	return counts, nil
}

type failedUnit struct {
	Unit        string `json:"unit"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
}

func failedUnits(ctx context.Context, scope string) ([]failedUnit, error) {
	out, err := systemctl(ctx, scope, "list-units", "--failed", "-o", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl %s list-units --failed: %w", scope, err)
	}
	var units []failedUnit
	if err := json.Unmarshal(out, &units); err != nil {
		return nil, fmt.Errorf("parse failed units: %w", err)
	}
	return units, nil
}

type journalEntry struct {
	Cursor    string `json:"__CURSOR"`
	Unit      string `json:"_SYSTEMD_UNIT"`
	UserUnit  string `json:"_SYSTEMD_USER_UNIT"`
	About     string `json:"UNIT"`
	AboutUser string `json:"USER_UNIT"`
}

func (e journalEntry) source() string {
	switch {
	case e.AboutUser != "":
		return "user:" + e.AboutUser
	case e.About != "":
		return "system:" + e.About
	case e.UserUnit != "":
		return "user:" + e.UserUnit
	case e.Unit != "":
		return "system:" + e.Unit
	}
	return ""
}

func journalLines(ctx context.Context, cursor string) (map[string]int, string, error) {
	args := []string{"-q", "--show-cursor", "-o", "json",
		"--output-fields=_SYSTEMD_UNIT,_SYSTEMD_USER_UNIT,UNIT,USER_UNIT"}
	if cursor == "" {
		args = append(args, "-n", "0")
	} else {
		args = append(args, "--after-cursor="+cursor)
	}
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, cursor, err
	}
	if err := cmd.Start(); err != nil {
		return nil, cursor, err
	}
	lines := map[string]int{}
	next := cursor
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		line := scan.Bytes()
		if rest, ok := strings.CutPrefix(string(line), "-- cursor: "); ok {
			next = rest
			continue
		}
		var entry journalEntry
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		next = entry.Cursor
		if source := entry.source(); source != "" {
			lines[source]++
		}
	}
	if err := scan.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", fmt.Errorf("read journal: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, "", fmt.Errorf("journalctl: %w", err)
	}
	return lines, next, nil
}
