package openwrt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInstallerSelectsNativeAPK(t *testing.T) {
	for _, tc := range []struct {
		name, arch, asset string
		legacy            bool
	}{
		{"raspberry-pi", "aarch64_cortex-a72", "aarch64_cortex-a72", false},
		{"older-release", "aarch64_cortex-a53", "arm64", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			log := filepath.Join(bin, "downloads")
			tools := map[string]string{
				"id": "echo 0\n",
				"awk": `if [ "$2" = /etc/apk/arch ]; then
  echo "$TEST_ARCH"
else
  exec /usr/bin/awk "$@"
fi
`,
				"apk": `case "$1" in
  --print-arch) echo aarch64 ;;
  add) exit 1 ;;
esac
`,
				"wget": `while [ "$#" -gt 0 ]; do
  case "$1" in
    -O) dest=$2; shift 2 ;;
    *) url=$1; shift ;;
  esac
done
printf '%s\n' "$url" >> "$TEST_DOWNLOADS"
case "$url" in
  *"$TEST_ARCH.apk") [ "$TEST_LEGACY" = false ] || exit 1 ;;
esac
printf 'package' > "$dest"
`,
				"curl": "exit 1\n",
			}
			for name, body := range tools {
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("sh", "../../install.sh")
			legacy := "false"
			if tc.legacy {
				legacy = "true"
			}
			cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin",
				"TEST_ARCH="+tc.arch, "TEST_DOWNLOADS="+log, "TEST_LEGACY="+legacy,
				"NETGRIP_VERSION=v0.78.0", "NETGRIP_ASSET_BASE=https://example.test/release")
			out, err := cmd.CombinedOutput()
			// Stop at apk add so the test never installs a package or starts a service.
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 5 || !strings.Contains(string(out), "apk add failed") {
				t.Fatalf("expected to reach apk add, got %v:\n%s", err, out)
			}
			downloads, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			urls := strings.Fields(string(downloads))
			want := "https://example.test/release/netgrip-0.78.0-r1-" + tc.asset + ".apk"
			if urls[len(urls)-1] != want {
				t.Fatalf("downloaded %v, want %s", urls, want)
			}
		})
	}
}

// Execute the release upload step with local packages and a stubbed GitHub CLI.
// Existing heal scripts require the generic suffix even in new releases.
func TestAPKReleaseNames(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/openwrt-package.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Strategy struct {
				Matrix struct{ Include []map[string]string }
			}
			Steps []struct{ Name, Run string }
		}
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["package"]
	var upload string
	for _, step := range job.Steps {
		if step.Name == "Upload to release" {
			upload = step.Run
		}
	}
	if upload == "" {
		t.Fatal("missing release upload step")
	}
	for _, row := range job.Strategy.Matrix.Include {
		if row["format"] != "apk" {
			continue
		}
		t.Run(row["pkgarch"], func(t *testing.T) {
			dir := t.TempDir()
			packages := filepath.Join(dir, "dist", "packages")
			if err := os.MkdirAll(packages, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(packages, "netgrip-0.78.0-r1.apk"), []byte("package"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
				t.Fatal(err)
			}
			script := upload
			for key, value := range row {
				script = strings.ReplaceAll(script, "${{ matrix."+key+" }}", value)
			}
			script = strings.ReplaceAll(script, "${{ github.repository }}", "example/netgrip")
			// GitHub's expression defaults to the binary when no native suffix is set.
			suffix := row["asset-suffix"]
			if suffix == "" {
				suffix = row["binary"]
			}
			script = strings.ReplaceAll(script, "${{ matrix.asset-suffix || matrix.binary }}", suffix)
			cmd := exec.Command("sh", "-eu", "-c", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin", "TAG=v0.78.0")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			want := strings.TrimPrefix(row["binary"], "linux-")
			if row["sdk-target"] == "bcm27xx/bcm2711" {
				want = "aarch64_cortex-a72"
			}
			if _, err := os.Stat(filepath.Join(packages, "netgrip-0.78.0-r1-"+want+".apk")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
