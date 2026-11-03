package packages

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAndroid(t *testing.T) {
	bin, sdk := t.TempDir(), t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv("ANDROID_HOME", sdk)
	if _, err := AndroidSDK(); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("err %v, want android not found", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "android"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDROID_HOME", "")
	if _, err := AndroidSDK(); err == nil || !strings.Contains(err.Error(), "ANDROID_HOME") {
		t.Fatalf("err %v, want ANDROID_HOME unset", err)
	}
	t.Setenv("ANDROID_HOME", sdk)

	ctx := context.Background()
	list := "android --sdk=" + sdk + " emulator list"
	install := "android --sdk=" + sdk + " sdk install platform-tools platforms/android-37.0 build-tools/36.0.0 emulator"
	create := "android --sdk=" + sdk + " emulator create medium_phone"
	run := &fake{out: map[string]string{list: "AVD ID  AVD Name  API Level  Status  Serial\n"}}
	it := androidItem(run)
	if err := it.Check(ctx); err == nil || !strings.Contains(err.Error(), "4 SDK packages missing") {
		t.Fatalf("check %v, want 4 packages missing", err)
	}
	if err := it.Fix(ctx); err != nil {
		t.Fatal(err)
	}
	if want := []string{install, list, create}; !slices.Equal(run.calls, want) {
		t.Fatalf("calls %q, want %q", run.calls, want)
	}

	for _, p := range androidPackages {
		if err := os.MkdirAll(filepath.Join(sdk, p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sdk, p, "package.xml"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := it.Check(ctx); err == nil || !strings.Contains(err.Error(), "AVD medium_phone missing") {
		t.Fatalf("check %v, want AVD missing", err)
	}
	run.out[list] += "medium_phone  Medium Phone  android-36  Offline\n"
	run.calls = nil
	if err := it.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if err := it.Fix(ctx); err != nil {
		t.Fatal(err)
	}
	if want := []string{list, install, list}; !slices.Equal(run.calls, want) {
		t.Fatalf("calls %q, want %q", run.calls, want)
	}
}
