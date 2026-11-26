package openwrt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The feed layout matters: a v3 index records no filenames, apk derives
// "<name>-<version>.apk" beside the index, and it needs one directory per
// package architecture. The signing step matters as much: apk reads
// signatures inside the adb, so a usign sidecar is invisible to it. Run the
// real script against stubbed apk commands to keep both honest without an
// OpenWrt SDK in the test environment.
type apkStub struct {
	pkgs   map[string][3]string // file base name -> {name, version, arch}
	verify string               // output of "apk verify"
}

// feedSandbox creates the feed directory with the given packages and a
// scripted apk on PATH.
func feedSandbox(t *testing.T, stub apkStub) (feedDir, logPath string) {
	t.Helper()
	dir := t.TempDir()
	feedDir = filepath.Join(dir, "feed")
	bin := filepath.Join(dir, "bin")
	for _, d := range []string{feedDir, bin} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	logPath = filepath.Join(dir, "stub.log")

	for base := range stub.pkgs {
		if err := os.WriteFile(filepath.Join(feedDir, base), []byte("package"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("cmd=$1; shift\n")
	script.WriteString("case \"$cmd\" in\n")
	script.WriteString("adbdump)\n")
	script.WriteString("  base=${1##*/}\n")
	script.WriteString("  case \"$base\" in\n")
	for base, fields := range stub.pkgs {
		script.WriteString("  " + base + ") printf 'info:\\n  name: " + fields[0] +
			"\\n  version: " + fields[1] + "\\n  arch: " + fields[2] + "\\n'; exit 0;;\n")
	}
	script.WriteString("  esac\n")
	script.WriteString("  exit 1;;\n")
	script.WriteString("mkndx|verify)\n")
	script.WriteString("  printf '%s %s\\n' \"$cmd\" \"$*\" >> \"$STUB_LOG\"\n")
	script.WriteString("  [ \"$cmd\" = verify ] && printf '" + stub.verify + "\\n'\n")
	script.WriteString("  exit 0;;\n")
	script.WriteString("esac\n")
	script.WriteString("exit 0\n")
	if err := os.WriteFile(filepath.Join(bin, "apk.real"), []byte(script.String()), 0755); err != nil {
		t.Fatal(err)
	}

	// The script calls apk as "<global flags> <command>", so drop the flags
	// (and the value of --keys-dir) before dispatching.
	wrapper := `#!/bin/sh
while [ $# -gt 0 ]; do
	case "$1" in
		--keys-dir) shift 2; continue ;;
		--*) shift; continue ;;
	esac
	break
done
exec "$STUB_APK" "$@"
`
	if err := os.WriteFile(filepath.Join(bin, "apk"), []byte(wrapper), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("STUB_LOG", logPath)
	t.Setenv("STUB_APK", filepath.Join(bin, "apk.real"))
	return feedDir, logPath
}

func runSignAPK(t *testing.T, feedDir string) (string, error) {
	t.Helper()
	keyDir := t.TempDir()
	priv := filepath.Join(keyDir, "priv.pem")
	pub := filepath.Join(keyDir, "pub.pem")
	for _, p := range []string{priv, pub} {
		if err := os.WriteFile(p, []byte("key"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.Command("sh", "sign-apk.sh", feedDir, priv, pub).CombinedOutput()
	return string(out), err
}

func TestSignAPKLaysOutFeedByArchitecture(t *testing.T) {
	stub := apkStub{
		pkgs: map[string][3]string{
			"netgrip-0.79.0-r1-arm64.apk":              {"netgrip", "0.79.0-r1", "aarch64_cortex-a53"},
			"netgrip-0.79.0-r1-aarch64_cortex-a72.apk": {"netgrip", "0.79.0-r1", "aarch64_cortex-a72"},
			"netgrip-extra-0.79.0-r1.apk":             {"netgrip-extra", "0.79.0-r1", "noarch"},
		},
		verify: "packages.adb: OK",
	}
	feedDir, logPath := feedSandbox(t, stub)

	out, err := runSignAPK(t, feedDir)
	if err != nil {
		t.Fatalf("sign-apk failed: %v\n%s", err, out)
	}

	want := []string{
		"aarch64_cortex-a53/netgrip-0.79.0-r1.apk",
		"aarch64_cortex-a53/netgrip-extra-0.79.0-r1.apk",
		"aarch64_cortex-a72/netgrip-0.79.0-r1.apk",
		"aarch64_cortex-a72/netgrip-extra-0.79.0-r1.apk",
		"aarch64_cortex-a53/netgrip-feed.pem",
		"netgrip-feed.pem",
	}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(feedDir, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	for _, gone := range []string{"netgrip-0.79.0-r1-arm64.apk", "netgrip-extra-0.79.0-r1.apk"} {
		if _, err := os.Stat(filepath.Join(feedDir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should have been moved or removed from the feed root", gone)
		}
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(raw)
	if n := strings.Count(log, "mkndx "); n != 2 {
		t.Errorf("expected one mkndx per architecture, got %d:\n%s", n, log)
	}
	if !strings.Contains(log, "--sign-key") {
		t.Errorf("indexes were not signed:\n%s", log)
	}
	if strings.Contains(log, "-arm64.apk") || strings.Contains(log, "-aarch64_cortex-a72.apk") {
		t.Errorf("index built from an architecture-suffixed name, apk cannot find it:\n%s", log)
	}
}

func TestSignAPKFailsWhenIndexDoesNotVerify(t *testing.T) {
	stub := apkStub{
		pkgs: map[string][3]string{
			"netgrip-0.79.0-r1-arm64.apk": {"netgrip", "0.79.0-r1", "aarch64_cortex-a53"},
		},
		verify: "packages.adb: UNTRUSTED signature",
	}
	feedDir, logPath := feedSandbox(t, stub)

	out, err := runSignAPK(t, feedDir)
	if err == nil {
		t.Fatalf("sign-apk accepted an untrusted index:\n%s", out)
	}
	if !strings.Contains(out, "does not verify") {
		t.Errorf("unexpected failure message:\n%s", out)
	}
	_ = logPath
}

func TestSignAPKFailsWithoutPackages(t *testing.T) {
	feedDir, _ := feedSandbox(t, apkStub{pkgs: map[string][3]string{}, verify: "packages.adb: OK"})

	out, err := runSignAPK(t, feedDir)
	if err == nil {
		t.Fatalf("sign-apk accepted an empty feed:\n%s", out)
	}
	if !strings.Contains(out, "no packages to index") {
		t.Errorf("unexpected failure message:\n%s", out)
	}
}
