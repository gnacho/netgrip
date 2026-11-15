package openwrt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The heal script runs on the router and picks the release asset by package
// architecture. It ships inside the package, and self-update only replaces
// /usr/sbin/netgrip, so a mismatch here survives until the package is
// reinstalled by hand. Run the real script with the router paths relocated
// and stub every command it talks to.
const healScriptPath = "netgrip/files/usr/libexec/netgrip-heal-register"

type healStubs struct {
	version    string
	arch       string
	registered bool
	opkg       bool
}

type healCase struct {
	name        string
	archFile    string // contents of /etc/apk/arch, empty for the default
	stubs       healStubs
	assets      []string
	wantURL     string
	wantLog     string
	wantExit    int
	wantNoFetch bool
}

func healReleaseJSON(assets []string) string {
	entries := make([]string, 0, len(assets))
	for _, name := range assets {
		entries = append(entries, fmt.Sprintf(`{"browser_download_url": "https://github.com/gnacho/netgrip/releases/download/v0.78.0/%s"}`, name))
	}
	return `{"tag_name": "v0.78.0", "assets": [` + strings.Join(entries, ", ") + `]}`
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
}

// relocations maps the absolute router paths in the script to the sandbox.
func healSandbox(t *testing.T, c healCase) (script, logPath string, env []string) {
	t.Helper()
	dir := t.TempDir()

	data, err := os.ReadFile(healScriptPath)
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "heal-register")
	logPath = filepath.Join(dir, "stub.log")
	jsonPath := filepath.Join(dir, "release.json")
	if err := os.WriteFile(jsonPath, []byte(healReleaseJSON(c.assets)), 0644); err != nil {
		t.Fatal(err)
	}

	src := string(data)
	replacements := map[string]string{
		"/usr/sbin/netgrip":         filepath.Join(dir, "netgrip"),
		"/var/run/netgrip-heal.pid": filepath.Join(dir, "heal.pid"),
		"/etc/apk/arch":             filepath.Join(dir, "apk-arch"),
		"/tmp/netgrip-heal.apk":     filepath.Join(dir, "heal.apk"),
		"/tmp/netgrip-heal.ipk":     filepath.Join(dir, "heal.ipk"),
	}
	for from, to := range replacements {
		if n := strings.Count(src, from); n != 1 {
			t.Fatalf("%s appears %d times in the heal script, want exactly 1", from, n)
		}
		src = strings.Replace(src, from, to, 1)
	}
	if err := os.WriteFile(scriptPath, []byte(src), 0755); err != nil {
		t.Fatal(err)
	}

	archPath := filepath.Join(dir, "apk-arch")
	if c.archFile != "" {
		if err := os.WriteFile(archPath, []byte(c.archFile+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(dir, "netgrip"), "#!/bin/sh\necho "+c.stubs.version+"\n")
	writeExecutable(t, filepath.Join(bin, "logger"), "#!/bin/sh\nprintf 'LOG %s\\n' \"$*\" >> \"$STUB_LOG\"\n")
	writeExecutable(t, filepath.Join(bin, "wget"), `#!/bin/sh
out=
url=
while [ $# -gt 0 ]; do
	case "$1" in
		-O) out=$2; shift 2 ;;
		-T) shift 2 ;;
		-q) shift ;;
		*) url=$1; shift ;;
	esac
done
if [ "$url" = "https://api.github.com" ]; then
	exit 0
fi
if [ "$out" = "-" ]; then
	cat "$STUB_JSON"
	exit $?
fi
printf 'GET %s\n' "$url" >> "$STUB_LOG"
printf 'package' > "$out"
`)

	if c.stubs.opkg {
		writeExecutable(t, filepath.Join(bin, "opkg"), `#!/bin/sh
case "$1" in
	list-installed) exit 0 ;;
	status) printf 'Architecture: `+c.stubs.arch+`\n' ;;
	install) printf 'ADD %s\n' "$2" >> "$STUB_LOG" ;;
esac
exit 0
`)
		// The opkg branch is selected by the absence of apk on PATH, and a
		// developer machine may well have a real apk installed, so this
		// sandbox exposes only the tools the script needs.
		for _, name := range []string{"awk", "sed", "grep", "tr", "cut", "head", "rm", "cat", "sleep"} {
			real, err := exec.LookPath(name)
			if err != nil {
				t.Fatalf("%s not found: %v", name, err)
			}
			if err := os.Symlink(real, filepath.Join(bin, name)); err != nil {
				t.Fatal(err)
			}
		}
		env = []string{
			"PATH=" + bin,
			"HOME=" + dir,
			"STUB_LOG=" + logPath,
			"STUB_JSON=" + jsonPath,
		}
		return scriptPath, logPath, env
	}
	{
		registered := "1"
		if c.stubs.registered {
			registered = "0"
		}
		writeExecutable(t, filepath.Join(bin, "apk"), `#!/bin/sh
case "$1" in
	info) exit `+registered+` ;;
	--print-arch) printf '`+c.stubs.arch+`\n' ;;
	add)
		shift
		for arg in "$@"; do
			case "$arg" in
				-*) ;;
				*) printf 'ADD %s\n' "$arg" >> "$STUB_LOG" ;;
			esac
		done
		;;
esac
exit 0
`)
	}

	env = []string{
		"PATH=" + bin + ":/usr/bin:/bin",
		"HOME=" + dir,
		"STUB_LOG=" + logPath,
		"STUB_JSON=" + jsonPath,
	}
	return scriptPath, logPath, env
}

func TestHealRegisterSelectsReleaseAsset(t *testing.T) {
	releaseAssets := []string{
		"netgrip-0.78.0-r1-amd64.apk",
		"netgrip-0.78.0-r1-arm64.apk",
		"netgrip-0.78.0-r1-mipsle.apk",
		"netgrip_0.78.0-1_aarch64_cortex-a53.ipk",
	}

	cases := []healCase{
		{
			name:     "native apk on a Raspberry Pi 4",
			archFile: "aarch64_cortex-a72",
			stubs:    healStubs{version: "v0.78.0"},
			assets: append([]string{
				"netgrip-0.78.0-r1-aarch64_cortex-a72.apk",
				"netgrip-0.78.0-r1-aarch64_cortex-a76.apk",
			}, releaseAssets...),
			wantURL: "netgrip-0.78.0-r1-aarch64_cortex-a72.apk",
		},
		{
			name:     "generic apk fallback when the release has no native package",
			archFile: "aarch64_cortex-a53",
			stubs:    healStubs{version: "v0.78.0"},
			assets:   releaseAssets,
			wantURL:  "netgrip-0.78.0-r1-arm64.apk",
		},
		{
			name:     "native apk on a Raspberry Pi 5",
			archFile: "aarch64_cortex-a76",
			stubs:    healStubs{version: "v0.78.0"},
			assets: append([]string{
				"netgrip-0.78.0-r1-aarch64_cortex-a76.apk",
			}, releaseAssets...),
			wantURL: "netgrip-0.78.0-r1-aarch64_cortex-a76.apk",
		},
		{
			name:    "arch comes from apk --print-arch when /etc/apk/arch is empty",
			stubs:   healStubs{version: "v0.78.0", arch: "aarch64"},
			assets:  releaseAssets,
			wantURL: "netgrip-0.78.0-r1-arm64.apk",
		},
		{
			name:     "no asset for the package architecture",
			archFile: "mips",
			stubs:    healStubs{version: "v0.78.0"},
			assets:   releaseAssets,
			wantLog:  "no release asset matching",
			wantExit: 1,
		},
		{
			name:        "dev build newer than the release is left alone",
			archFile:    "aarch64_cortex-a53",
			stubs:       healStubs{version: "v0.99.0-rc1"},
			assets:      releaseAssets,
			wantLog:     "does not match running v0.99.0-rc1",
			wantNoFetch: true,
		},
		{
			name:        "registry entry intact",
			archFile:    "aarch64_cortex-a53",
			stubs:       healStubs{version: "v0.78.0", registered: true},
			assets:      releaseAssets,
			wantNoFetch: true,
		},
		{
			name:    "opkg on OpenWrt 24",
			stubs:   healStubs{version: "v0.78.0", arch: "aarch64_cortex-a53", opkg: true},
			assets:  releaseAssets,
			wantURL: "netgrip_0.78.0-1_aarch64_cortex-a53.ipk",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			script, logPath, env := healSandbox(t, c)
			cmd := exec.Command("sh", script)
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			exit := 0
			if ee, ok := err.(*exec.ExitError); ok {
				exit = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("run: %v", err)
			}
			if exit != c.wantExit {
				t.Fatalf("exit %d, want %d (output: %s)", exit, c.wantExit, out)
			}
			raw, readErr := os.ReadFile(logPath)
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			logs := string(raw)
			if c.wantURL != "" {
				if !strings.Contains(logs, "GET https://github.com/gnacho/netgrip/releases/download/v0.78.0/"+c.wantURL) {
					t.Fatalf("did not fetch %s, log:\n%s", c.wantURL, logs)
				}
				if !strings.Contains(logs, "restored from") {
					t.Fatalf("install step did not report success, log:\n%s", logs)
				}
			}
			if c.wantNoFetch && strings.Contains(logs, "GET ") {
				t.Fatalf("unexpected download, log:\n%s", logs)
			}
			if c.wantLog != "" && !strings.Contains(logs, c.wantLog) {
				t.Fatalf("log does not contain %q:\n%s", c.wantLog, logs)
			}
		})
	}
}
