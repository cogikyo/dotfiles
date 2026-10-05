package iso

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/setup"
)

func TestQMP(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	go func() {
		defer server.Close()
		in := bufio.NewScanner(server)
		reply := func(lines ...string) {
			for _, l := range lines {
				server.Write([]byte(l + "\r\n"))
			}
		}
		reply(`{"QMP": {"version": {}, "capabilities": ["oob"]}}`)
		for in.Scan() {
			var req struct {
				Execute   string          `json:"execute"`
				Arguments json.RawMessage `json:"arguments"`
			}
			json.Unmarshal(in.Bytes(), &req)
			switch req.Execute {
			case "qmp_capabilities":
				reply(`{"return": {}}`)
			case "query-blockstats":
				reply(`{"event": "RESET", "data": {}}`, `{"return": [{"device": "disk", "stats": {"rd_bytes": 7}}, {"stats": {"rd_bytes": 5}}]}`)
			case "send-key":
				desc, _ := json.Marshal(string(req.Arguments))
				reply(`{"error": {"class": "GenericError", "desc": ` + string(desc) + `}}`)
			}
		}
	}()
	q, err := dial(t.Context(), client)
	if err != nil {
		t.Fatal(err)
	}
	v := &vm{qmp: q, done: make(chan struct{})}
	if n, err := v.reads(t.Context()); err != nil || n != 12 {
		t.Fatalf("reads = %d, %v", n, err)
	}
	err = v.typeLine(t.Context(), "a")
	if err == nil || !strings.Contains(err.Error(), "GenericError") || !strings.Contains(err.Error(), `{"keys":[{"data":"a","type":"qcode"}]}`) {
		t.Fatalf("send-key error = %v", err)
	}
	if _, err := v.reads(t.Context()); err != nil {
		t.Fatalf("a QMP error reply broke the connection: %v", err)
	}
	gone, cancel := context.WithCancel(t.Context())
	cancel()
	_, dead := v.reads(gone)
	if _, err := v.reads(t.Context()); dead == nil || err != dead {
		t.Fatalf("canceled call = %v, next call = %v; want the connection dead", dead, err)
	}
}

func TestWait(t *testing.T) {
	if err := spawn(t.Context(), []string{"true"}).wait(t.Context()); err != nil {
		t.Fatalf("clean exit: %v", err)
	}
	if err := spawn(t.Context(), []string{"false"}).wait(t.Context()); err == nil {
		t.Fatal("non-zero exit counted as a clean reboot")
	}
}

func TestSerial(t *testing.T) {
	log := []byte("noise\r\nBoot in 1s.\x1b[2J\x1b[001;001H" +
		`{"dctltest":"install","ok":true,"total":61.5,"phases":[{"name":"pacstrap","seconds":40.1}]}` + "\r\n" +
		`[{"stage":"system","state":"done","items":[{"item":"system-files","state":"done"}]},{"stage":"keys","state":"failed","items":[{"item":"keys-luks","state":"failed","detail":"no token"}]}]` + "\r\n" +
		`[{"stage":"home","state":"done","items":[{"item":"home-dirs","state":"done"}]}]` + "\r\n" +
		Greeter + "active\r\n" +
		`[{"stage":"partial`)

	lines := find(log, installPrefix)
	if len(lines) != 1 {
		t.Fatalf("install lines %q", lines)
	}
	r, err := parseInstall(lines[0])
	if err != nil || !r.OK || r.Total != 61.5 || r.Phases[0] != (Phase{"pacstrap", 40.1}) {
		t.Fatalf("install = %+v, %v", r, err)
	}
	if _, err := parseInstall(`{"dctltest":"other"}`); err == nil {
		t.Fatal("accepted a non-install event")
	}

	lines = find(log, setupPrefix)
	if len(lines) != 2 {
		t.Fatalf("setup lines %q, want the root and the user report", lines)
	}
	var rs []setup.Report
	if err := json.Unmarshal([]byte(lines[0]), &rs); err != nil || len(rs) != 2 || rs[1].Items[0] != (setup.Result{Item: "keys-luks", State: setup.Failed, Detail: "no token"}) {
		t.Fatalf("setup = %+v, %v", rs, err)
	}
	if got := find(log, Greeter); len(got) != 1 || got[0] != Greeter+"active" {
		t.Fatalf("greeter lines %q", got)
	}

	if err := accept("1", rs); err == nil || !strings.Contains(err.Error(), "no packages") || strings.Contains(err.Error(), "keys-luks") {
		t.Fatalf("accept = %v", err)
	}
	stage := func(name string, items ...string) setup.Report {
		r := setup.Report{Stage: name, State: setup.Done}
		for _, it := range items {
			r.Items = append(r.Items, setup.Result{Item: it, State: setup.Done})
		}
		return r
	}
	healthy := func() []setup.Report {
		return []setup.Report{
			stage("system", "system-files"),
			stage("packages", "packages-installed"),
			stage("home", "home-links"),
		}
	}
	if err := accept("1", append(healthy(), rs[1])); err != nil {
		t.Fatalf("expected failures outside the required stages: %v", err)
	}
	for i, name := range required {
		missing := slices.Delete(healthy(), i, i+1)
		if err := accept("2", missing); err == nil || !strings.Contains(err.Error(), "no "+name) {
			t.Fatalf("missing %s = %v", name, err)
		}
		bad := healthy()
		bad[i].Items[0].State = setup.ManualState
		if err := accept("2", bad); err == nil || !strings.Contains(err.Error(), bad[i].Items[0].Item+" manual") {
			t.Fatalf("manual %s = %v", name, err)
		}
	}
}

func TestArgv(t *testing.T) {
	tt := &test{dir: "/run", iso: "/x.iso"}
	install := strings.Join(tt.argv(true), " ")
	for _, want := range []string{
		"q35,smm=on,accel=kvm", "driver=cfi.pflash01,property=secure,value=on",
		"unit=0,readonly=on,file=" + ovmfCode, "unit=1,file=/run/vars.fd",
		"serial=DCTLTEST0", "-nic none", "append=on,path=/run/serial.log",
		"-no-reboot", "file=/x.iso", "file=/run/dctltest.img",
	} {
		if !strings.Contains(install, want) {
			t.Errorf("install argv lacks %q", want)
		}
	}
	boot := tt.argv(false)
	if slices.Contains(boot, "-no-reboot") || strings.Contains(strings.Join(boot, " "), ".iso") {
		t.Fatalf("boot argv attaches install media: %v", boot)
	}
}

func TestDrive(t *testing.T) {
	for _, tool := range []string{"mkfs.vfat", "mcopy", "mtype", "mlabel"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool, " not installed")
		}
	}
	dir := t.TempDir()
	answers := filepath.Join(dir, "answers.json")
	if err := os.WriteFile(answers, []byte(`{"user":"ada"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "dctltest.img")
	if err := drive(t.Context(), img, map[string]string{"answers.json": answers, "dctl": answers}); err != nil {
		t.Fatal(err)
	}
	label, err := exec.Command("mlabel", "-s", "-i", img, "::").Output()
	if err != nil || !strings.Contains(string(label), "DCTLTEST") {
		t.Fatalf("label %q, %v", label, err)
	}
	for _, name := range []string{"answers.json", "dctl"} {
		got, err := exec.Command("mtype", "-i", img, "::"+name).Output()
		if err != nil || string(got) != `{"user":"ada"}` {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
}
