package home

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const selection = "Profile=Costello\nProfileDir=Costello\nSceneCollection=Costello\nSceneCollectionFile=Costello.json\n"

func TestMergeOBS(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"missing", "", "[Basic]\n" + selection},
		{
			"merge",
			"[General]\nProfile=keep\n[Basic]\n; note\nProfile=Old\nWidth=1920\nProfile=Dup\nSceneCollection=Costello\n[Video]\nx=1\n",
			"[General]\nProfile=keep\n[Basic]\nProfileDir=Costello\nSceneCollectionFile=Costello.json\n; note\nProfile=Costello\nWidth=1920\nSceneCollection=Costello\n[Video]\nx=1\n",
		},
		{"append", "[Video]\nx=1", "[Video]\nx=1\n\n[Basic]\n" + selection},
		{
			"repeated",
			"[Basic]\n" + selection + "Width=1\n[Basic]\nProfile=Other\n",
			"[Basic]\nWidth=1\n[Basic]\nProfileDir=Costello\nSceneCollection=Costello\nSceneCollectionFile=Costello.json\nProfile=Costello\n",
		},
		{
			"repeated suffixed",
			"[Basic]\n" + selection + "Width=1\n[Basic] # later\nProfile=Other\n",
			"[Basic]\nWidth=1\n[Basic] # later\nProfileDir=Costello\nSceneCollection=Costello\nSceneCollectionFile=Costello.json\nProfile=Costello\n",
		},
	} {
		got := string(mergeOBS([]byte(tc.in)))
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
		if drift := obsDrift([]byte(got)); len(drift) > 0 {
			t.Errorf("%s: drift after merge: %v", tc.name, drift)
		}
	}
	if drift := obsDrift([]byte("[Basic]\n" + selection + "[Basic]\nProfile=Other\n")); len(drift) != 4 {
		t.Errorf("later [Basic] section not reported: %v", drift)
	}
	if drift := obsDrift([]byte("[Basic]\n" + selection + "[Basic] # later\nProfile=Other\n")); len(drift) != 4 {
		t.Errorf("later suffixed [Basic] section not reported: %v", drift)
	}
	if drift := obsDrift([]byte("[Basic]\n" + selection + "Profile=Costello\n")); len(drift) != 1 {
		t.Errorf("duplicate key not reported: %v", drift)
	}
}

func TestOBSLegacyDir(t *testing.T) {
	r := setup(t)
	dir := filepath.Join(r.Home, ".config", "obs-studio")
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.Config("obs-studio"), dir); err != nil {
		t.Fatal(err)
	}
	for _, c := range obs(r) {
		if err := c.Fix(t.Context()); err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if err := c.Check(t.Context()); err != nil {
			t.Fatalf("%s after fix: %v", c.Name, err)
		}
	}
	if _, err := os.Stat(r.Config("obs-studio", "user.ini")); err == nil {
		t.Error("user.ini written into the checkout")
	}
	foreign := setup(t)
	if err := os.MkdirAll(filepath.Join(foreign.Home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(foreign.Home, ".config", "obs-studio")); err != nil {
		t.Fatal(err)
	}
	if obs(foreign)[0].Fix(t.Context()) == nil {
		t.Error("foreign obs-studio symlink accepted")
	}
}

func TestLibrepods(t *testing.T) {
	r := setup(t)
	write(t, r.Packages("librepods-max2", "app_settings.json"), `{"theme":"Dark","stem_control":false}`)
	write(t, r.Config("systemd", "user", "librepods.service"), "[Unit]")
	if err := os.MkdirAll(filepath.Join(r.Home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(r.Config("systemd"), filepath.Join(r.Home, ".config", "systemd")); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(r.Home, ".config", "librepods")
	data := filepath.Join(r.Home, ".local", "share", "librepods")
	settings := filepath.Join(cfg, "app_settings.json")
	write(t, settings, `{"theme":"Light","volume":7}`)
	write(t, filepath.Join(data, "devices.json"), `{}`)
	c := librepods(r)
	if c.Check(t.Context()) == nil {
		t.Fatal("open modes and drifted settings reported clean")
	}
	if err := c.Fix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := c.Check(t.Context()); err != nil {
		t.Fatalf("after fix: %v", err)
	}
	if target, _ := os.Readlink(r.Config("systemd", "user", "hyprland-session.target.wants", "librepods.service")); target != "../librepods.service" {
		t.Errorf("wants link %q not restored", target)
	}
	raw, _ := os.ReadFile(settings)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil || got["volume"] != 7.0 || got["theme"] != "Dark" || got["stem_control"] != false {
		t.Fatalf("settings %s, %v", raw, err)
	}
	for _, bad := range []string{`[1]`, `null`, `{`} {
		if err := os.WriteFile(settings, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if c.Check(t.Context()) == nil {
			t.Errorf("settings %s reported clean", bad)
		}
		if c.Fix(t.Context()) == nil {
			t.Errorf("settings %s overwritten", bad)
		}
	}
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(settings, 0o600); err != nil {
		t.Fatal(err)
	}
	if c.Check(t.Context()) == nil || c.Fix(t.Context()) == nil {
		t.Error("FIFO settings accepted")
	}
}
