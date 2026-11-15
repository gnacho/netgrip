package modules

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// useTempHelperDir points helperScriptDir at a t.TempDir() and restores it on
// cleanup.
func useTempHelperDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := helperScriptDir
	helperScriptDir = dir
	t.Cleanup(func() { helperScriptDir = old })
	return dir
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// TestSyncHelperInstallsMissing: a device that only has the binary (manual
// copy, no package) gets both helper scripts, executable.
func TestSyncHelperInstallsMissing(t *testing.T) {
	dir := useTempHelperDir(t)

	syncHelperScripts("startup", "v0.79.0")

	for _, name := range helperScripts {
		path := filepath.Join(dir, name)
		got := readFile(t, path)
		want, err := canonicalHelperScript(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s does not match the embedded copy", name)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&0111 == 0 {
			t.Errorf("%s is not executable: %v", name, info.Mode())
		}
	}
}

// TestSyncHelperRewritesStaleManaged: a marked script that differs from the
// embedded one is replaced and the old content is kept in a versioned
// backup.
func TestSyncHelperRewritesStaleManaged(t *testing.T) {
	dir := useTempHelperDir(t)
	path := filepath.Join(dir, "netgrip-heal-register")
	stale := []byte("#!/bin/sh\n" + helperMarker + "\n# old heal without the version fix\n")

	if err := os.WriteFile(path, stale, 0755); err != nil {
		t.Fatal(err)
	}

	syncHelperScripts("startup", "v0.78.0")

	want, err := canonicalHelperScript("netgrip-heal-register")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); !bytes.Equal(got, want) {
		t.Errorf("stale helper not rewritten:\n%s", got)
	}
	if backup := readFile(t, path+".bak-0.78.0"); !bytes.Equal(backup, stale) {
		t.Errorf("backup does not hold the previous content:\n%s", backup)
	}
}

// TestSyncHelperKeepsCustomScript: no marker and no byte match means a local
// edit, which is never touched.
func TestSyncHelperKeepsCustomScript(t *testing.T) {
	dir := useTempHelperDir(t)
	path := filepath.Join(dir, "netgrip-restore-rules")
	custom := []byte("#!/bin/sh\n# local tweaks\nexit 0\n")
	if err := os.WriteFile(path, custom, 0755); err != nil {
		t.Fatal(err)
	}

	syncHelperScripts("startup", "v0.78.0")

	if got := readFile(t, path); !bytes.Equal(got, custom) {
		t.Errorf("custom helper was modified:\n%s", got)
	}
	if _, err := os.Stat(path + ".bak-0.78.0"); !os.IsNotExist(err) {
		t.Errorf("backup created for a custom script: %v", err)
	}
}

// TestSyncHelperIdempotent: a script that already matches is not rewritten,
// so its mtime survives repeated starts.
func TestSyncHelperIdempotent(t *testing.T) {
	dir := useTempHelperDir(t)
	syncHelperScripts("startup", "v0.79.0")

	path := filepath.Join(dir, "netgrip-heal-register")
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	syncHelperScripts("startup", "v0.79.0")

	second, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ModTime().Equal(second.ModTime()) {
		t.Errorf("unchanged helper was rewritten (mtime changed)")
	}
	if entries, _ := filepath.Glob(path + ".bak-*"); len(entries) > 0 {
		t.Errorf("unexpected backups for an up-to-date script: %v", entries)
	}
}

// TestSyncHelperHealsLegacyPackagedScript: every variant the package shipped
// before the marker existed is recognized as ours and replaced, which is how
// a router installed from an older package converges.
func TestSyncHelperHealsLegacyPackagedScript(t *testing.T) {
	for _, name := range helperScripts {
		entries, err := helperFS.ReadDir("selfupdate_helpers/legacy/" + name)
		if err != nil {
			t.Fatalf("no legacy variants embedded for %s: %v", name, err)
		}
		for _, entry := range entries {
			t.Run(name+"/"+entry.Name(), func(t *testing.T) {
				dir := useTempHelperDir(t)
				path := filepath.Join(dir, name)
				legacy, err := helperFS.ReadFile("selfupdate_helpers/legacy/" + name + "/" + entry.Name())
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, legacy, 0755); err != nil {
					t.Fatal(err)
				}

				syncHelperScripts("startup", "v0.79.0")

				want, err := canonicalHelperScript(name)
				if err != nil {
					t.Fatal(err)
				}
				if got := readFile(t, path); !bytes.Equal(got, want) {
					t.Errorf("legacy packaged helper not rewritten:\n%s", got)
				}
				if _, err := os.Stat(path + ".bak-0.79.0"); err != nil {
					t.Errorf("backup not created for a legacy packaged script: %v", err)
				}
			})
		}
	}
}

// TestSyncHelperKeepsTweakedLegacy: a legacy script with a local edit no
// longer byte-matches any packaged variant and is treated as customized.
func TestSyncHelperKeepsTweakedLegacy(t *testing.T) {
	dir := useTempHelperDir(t)
	path := filepath.Join(dir, "netgrip-heal-register")
	legacy, err := helperFS.ReadFile("selfupdate_helpers/legacy/netgrip-heal-register/v1")
	if err != nil {
		t.Fatal(err)
	}
	tweaked := append([]byte("#!/bin/sh\n# local tweak\n"), legacy...)
	if err := os.WriteFile(path, tweaked, 0755); err != nil {
		t.Fatal(err)
	}

	syncHelperScripts("startup", "v0.79.0")

	if got := readFile(t, path); !bytes.Equal(got, tweaked) {
		t.Errorf("tweaked legacy helper was modified:\n%s", got)
	}
}

// TestSyncHelperSurvivesUnwritableDir: a missing or read-only directory must
// not panic or abort the update, and the other scripts still get a chance.
func TestSyncHelperSurvivesMissingDir(t *testing.T) {
	old := helperScriptDir
	helperScriptDir = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { helperScriptDir = old })

	syncHelperScripts("startup", "v0.79.0")
}

// TestBootSyncHelpersSkipOffRouter: without /etc/openwrt_release nothing is
// created.
func TestBootSyncHelpersSkipOffRouter(t *testing.T) {
	useTempOpenWrtRelease(t, false)
	dir := useTempHelperDir(t)

	SyncHelperScriptsAtBoot("v0.79.0")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("off-router startup sync wrote files: %d", len(entries))
	}
}

// TestBootSyncHelpersOnRouter: with /etc/openwrt_release present the startup
// sync installs the scripts.
func TestBootSyncHelpersOnRouter(t *testing.T) {
	useTempOpenWrtRelease(t, true)
	dir := useTempHelperDir(t)

	SyncHelperScriptsAtBoot("v0.79.0")

	for _, name := range helperScripts {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not installed on router startup: %v", name, err)
		}
	}
}

// TestEmbeddedHelpersMatchPackaged guards the single source of truth: the
// embedded copies must stay byte-identical to the scripts the apk/ipk
// package ships.
func TestEmbeddedHelpersMatchPackaged(t *testing.T) {
	packaged := map[string]string{
		"netgrip-heal-register": "../../deploy/openwrt/netgrip/files/usr/libexec/netgrip-heal-register",
		"netgrip-restore-rules": "../../deploy/openwrt/netgrip/files/netgrip-restore-rules",
	}
	for name, path := range packaged {
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("packaged %s not reachable from test dir: %v", name, err)
		}
		embedded, err := canonicalHelperScript(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(onDisk, embedded) {
			t.Errorf("%s diverged: refresh internal/modules/selfupdate_helpers/%s from %s", name, name, path)
		}
	}
}

// TestPackagedHelpersCarryMarker: the marker is what makes future syncs
// recognize a shipped script as ours without a legacy entry.
func TestPackagedHelpersCarryMarker(t *testing.T) {
	for _, name := range helperScripts {
		content, err := canonicalHelperScript(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(content, []byte(helperMarker)) {
			t.Errorf("%s has no managed marker", name)
		}
	}
}
