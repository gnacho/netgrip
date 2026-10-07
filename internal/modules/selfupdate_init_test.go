package modules

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// useTempInitScript points initScriptPath at a file inside a t.TempDir()
// and restores it on cleanup.
func useTempInitScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "netgrip")
	old := initScriptPath
	initScriptPath = path
	t.Cleanup(func() { initScriptPath = old })
	return path
}

// TestSelfUpdateSyncInitRewritesStaleManaged: an init script that is ours
// (marker present) but differs from the embedded one is rewritten and the
// old content lands in a versioned backup (#433).
func TestSelfUpdateSyncInitRewritesStaleManaged(t *testing.T) {
	path := useTempInitScript(t)

	stale := []byte("#!/bin/sh /etc/rc.common\n" + initMarker + "\n# old version without https flags\n")
	if err := os.WriteFile(path, stale, 0755); err != nil {
		t.Fatal(err)
	}

	syncInitScript("v0.72.0")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalInitScript()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("init script not rewritten to embedded canonical:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	backup, err := os.ReadFile(path + ".bak-0.72.0")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if !bytes.Equal(backup, stale) {
		t.Errorf("backup does not hold the previous content:\n%s", backup)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("rewritten init script lost its executable bit: %v", info.Mode())
	}
}

// TestSelfUpdateSyncInitKeepsCustomScript: a personalized script without
// our marker must be left untouched (#433).
func TestSelfUpdateSyncInitKeepsCustomScript(t *testing.T) {
	path := useTempInitScript(t)

	custom := []byte("#!/bin/sh /etc/rc.common\n# hand-written local tweaks\nexit 0\n")
	if err := os.WriteFile(path, custom, 0755); err != nil {
		t.Fatal(err)
	}

	syncInitScript("v0.72.0")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, custom) {
		t.Errorf("custom init script was modified:\n%s", got)
	}
	if _, err := os.Stat(path + ".bak-0.72.0"); !os.IsNotExist(err) {
		t.Errorf("backup created for a custom script, err=%v", err)
	}
}

// TestSelfUpdateSyncInitInstallsMissing: with no init script at all (the
// binary was installed by hand, outside any package), it is installed.
func TestSelfUpdateSyncInitInstallsMissing(t *testing.T) {
	path := useTempInitScript(t)

	syncInitScript("v0.72.0")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("init script not installed: %v", err)
	}
	want, err := canonicalInitScript()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("installed init script differs from embedded canonical")
	}
}

// TestSelfUpdateSyncInitIdempotent: running the sync twice must not
// rewrite an already-current script (mtime stays, no pointless backups).
func TestSelfUpdateSyncInitIdempotent(t *testing.T) {
	path := useTempInitScript(t)

	syncInitScript("v0.72.0")
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	syncInitScript("v0.72.0")
	second, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ModTime().Equal(second.ModTime()) {
		t.Errorf("unchanged init script was rewritten (mtime changed)")
	}
	if entries, _ := filepath.Glob(path + ".bak-*"); len(entries) > 0 {
		t.Errorf("unexpected backups for an up-to-date script: %v", entries)
	}
}

// TestSelfUpdateEmbeddedInitMatchesPackaged guards the single source of
// truth: the embedded copy must stay byte-identical to the init script the
// apk/ipk package ships.
func TestSelfUpdateEmbeddedInitMatchesPackaged(t *testing.T) {
	packaged, err := os.ReadFile("../../deploy/openwrt/netgrip/files/netgrip.init")
	if err != nil {
		t.Skipf("packaged init script not reachable from test dir: %v", err)
	}
	embedded, err := canonicalInitScript()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packaged, embedded) {
		t.Errorf("embedded init script diverged from deploy/openwrt/netgrip/files/netgrip.init;\nrefresh internal/modules/selfupdate_init/netgrip.init")
	}
}
