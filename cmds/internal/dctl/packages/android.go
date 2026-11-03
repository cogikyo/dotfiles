package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/setup"
)

var androidPackages = []string{"platform-tools", "platforms/android-37.0", "build-tools/36.0.0", "emulator"}

const androidAVD = "medium_phone"

func AndroidSDK() (string, error) {
	if _, err := exec.LookPath("android"); err != nil {
		return "", fmt.Errorf("android-cli is not installed: %w", err)
	}
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		return "", errors.New("ANDROID_HOME is unset; start a new shell so config/shell/env.sh exports it")
	}
	return sdk, nil
}

// Android uses ~/.androidrc for the metrics and crash-report opt-out.
func Android(ctx context.Context, run execx.Runner, sdk string, args ...string) (string, error) {
	return run.Output(ctx, "", "android", androidArgs(sdk, args...)...)
}

func androidArgs(sdk string, args ...string) []string {
	return slices.Concat([]string{"--sdk=" + sdk}, args)
}

func androidItem(run execx.Runner) setup.Item {
	return setup.Item{
		Name: "extra-android",
		Check: func(ctx context.Context) error {
			sdk, err := AndroidSDK()
			if err != nil {
				return err
			}
			var missing []string
			for _, p := range androidPackages {
				if _, err := os.Stat(filepath.Join(sdk, p, "package.xml")); err != nil {
					missing = append(missing, p)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("%d SDK packages missing: %s", len(missing), strings.Join(missing, " "))
			}
			return avd(ctx, run, sdk)
		},
		Fix: func(ctx context.Context) error {
			sdk, err := AndroidSDK()
			if err != nil {
				return err
			}
			if err := run.Run(ctx, "", "android", androidArgs(sdk, slices.Concat([]string{"sdk", "install"}, androidPackages)...)...); err != nil {
				return err
			}
			// emulator create exits 0 for an existing AVD, so check the list first.
			if avd(ctx, run, sdk) == nil {
				return nil
			}
			return run.Run(ctx, "", "android", androidArgs(sdk, "emulator", "create", androidAVD)...)
		},
	}
}

func avd(ctx context.Context, run execx.Runner, sdk string) error {
	out, err := Android(ctx, run, sdk, "emulator", "list")
	if err != nil {
		return err
	}
	for l := range strings.Lines(out) {
		if f := strings.Fields(l); len(f) > 0 && f[0] == androidAVD {
			return nil
		}
	}
	return fmt.Errorf("AVD %s missing", androidAVD)
}
