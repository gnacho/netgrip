package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const owutVerboseSample = `ASU-Server     https://sysupgrade.openwrt.org
Upstream       https://downloads.openwrt.org
Target         qualcommax/ipq807x
Profile        redmi_ax6
Package-arch   aarch64_cortex-a53
Version-from   25.12.5 r33051-f5dae5ece4 (kernel 6.12.94)
Version-to     25.12.5 r33051-f5dae5ece4 (kernel 6.12.94)
33 packages are out-of-date
  netgrip                             0.2.0-r1                       missing to-version
1 packages missing in target version, cannot upgrade
WARNING: There are 1 missing default packages, confirm this is expected before proceeding
ERROR: Checks reveal errors, do not upgrade
`

func TestParseOwutVerboseNetgripMissing(t *testing.T) {
	missing := []string{}
	for _, m := range reMissingPkg.FindAllStringSubmatch(owutVerboseSample, -1) {
		missing = append(missing, m[1])
	}
	if len(missing) != 1 || missing[0] != "netgrip" {
		t.Fatalf("unexpected missing packages: %v", missing)
	}
	if !onlyNetgrip(missing) {
		t.Fatal("onlyNetgrip should be true")
	}
	if !reMissingCount.MatchString(owutVerboseSample) {
		t.Fatal("missing count not matched")
	}
	if reSafe.MatchString(owutVerboseSample) {
		t.Fatal("sample should NOT be safe to proceed")
	}
}

func TestOnlyNetgrip(t *testing.T) {
	if onlyNetgrip([]string{"netgrip", "luci-app-dawn"}) {
		t.Fatal("should be false with other packages")
	}
	if onlyNetgrip([]string{}) {
		t.Fatal("should be false when empty")
	}
}

func TestCoreVersion(t *testing.T) {
	a := "25.12.5 r33051-f5dae5ece4 (kernel 6.12.94)"
	b := "25.12.5 r33051-f5dae5ece4"
	if coreVersion(a) != coreVersion(b) {
		t.Fatalf("core versions differ: %q vs %q", coreVersion(a), coreVersion(b))
	}
}

func TestEnsureLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc.local")

	// New file with marker: line goes before the marker.
	if err := ensureLine(path, "hook-line", "exit 0"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "hook-line\nexit 0") {
		t.Fatalf("hook not before marker: %q", content)
	}

	// Idempotent.
	if err := ensureLine(path, "hook-line", "exit 0"); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(path)
	if strings.Count(string(content), "hook-line") != 1 {
		t.Fatalf("duplicated line: %q", content)
	}

	// Existing file without marker: append at the end.
	path2 := filepath.Join(dir, "sysupgrade.conf")
	if err := os.WriteFile(path2, []byte("/etc/config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureLine(path2, "/root/netgrip.apk", ""); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(path2)
	if string(content) != "/etc/config\n/root/netgrip.apk\n" {
		t.Fatalf("unexpected content: %q", content)
	}
}
