package modules

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

// The canonical init script is embedded in the binary so a self-update can
// refresh it, not just the binary: the script is what translates the
// netgrip.main.https UCI options into -https* flags, and it only used to
// travel inside the apk/ipk package. A router updated from its own panel
// kept the old script forever and could never enable HTTPS (#433).
//
// The embedded copy is kept in sync with the packaged original
// (deploy/openwrt/netgrip/files/netgrip.init) by a unit test.

//go:embed selfupdate_init/netgrip.init
var initScriptFS embed.FS

// Package scripts shipped before the managed marker existed carry no way to
// tell them apart from a hand-customized one, so the marker alone cannot
// heal them (#492): a router that self-updated since then still runs the
// pre-HTTPS script and its UCI https toggle stays a no-op. The exact
// contents of every packaged variant are embedded instead: a script that
// byte-matches one of them is a package script, not a customization, and is
// rewritten like any other stale managed script. A modified script matches
// nothing and is still left untouched.
//
//go:embed selfupdate_init/legacy
var legacyInitFS embed.FS

// initMarker identifies an init script written by us. A customized script
// without the marker is never touched unless it byte-matches a known
// packaged variant (see legacyInitFS).
const initMarker = "# netgrip init - managed"

// initScriptPath is a package-level variable so tests can point it at a
// t.TempDir() directory.
var initScriptPath = "/etc/init.d/netgrip"

// openwrtReleasePath gates the startup sync: without it the process is not
// on a router (dev machine, demo mode) and there is nothing to heal.
var openwrtReleasePath = "/etc/openwrt_release"

func canonicalInitScript() ([]byte, error) {
	return initScriptFS.ReadFile("selfupdate_init/netgrip.init")
}

// SyncInitScriptAtBoot converges the init script on every process start,
// not only before a self-update restart: a device that only ever receives
// manually copied binaries would otherwise keep a pre-HTTPS script forever
// and its UCI https toggle would stay a no-op (#544). Same safety rules as
// the self-update path: customized scripts are never touched.
func SyncInitScriptAtBoot(currentVersion string) {
	if _, err := os.Stat(openwrtReleasePath); err != nil {
		return
	}
	ensureInitScript("startup", currentVersion)
}

// syncInitScript refreshes the init script before the post-update restart.
// Best effort on purpose: a failure here must not brick an update whose
// binary is already in place; the mismatch only means newer UCI-driven
// flags (e.g. HTTPS) stay unavailable until fixed.
func syncInitScript(currentVersion string) {
	ensureInitScript("self-update", currentVersion)
}

func ensureInitScript(reason, currentVersion string) {
	desired, err := canonicalInitScript()
	if err != nil {
		log.Printf("netgrip: %s: embedded init script unreadable: %v", reason, err)
		return
	}

	existing, err := os.ReadFile(initScriptPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := writeInitScript(desired); err != nil {
			log.Printf("netgrip: %s: install init script: %v", reason, err)
			return
		}
		log.Printf("netgrip: %s: installed missing init script %s", reason, initScriptPath)
	case err != nil:
		log.Printf("netgrip: %s: read init script: %v", reason, err)
	case bytes.Equal(existing, desired):
		// Already the current script; do not rewrite so the mtime stays.
	case !strings.Contains(string(existing), initMarker) && !isPackagedLegacyScript(existing):
		log.Printf("netgrip: %s: %s has no %q marker and matches no packaged variant, leaving the customized script untouched",
			reason, initScriptPath, initMarker)
	default:
		rewriteInitScript(existing, desired, reason, currentVersion)
	}
}

// isPackagedLegacyScript reports whether content byte-matches one of the
// init script variants the package shipped before the managed marker
// existed. Only a pristine package script matches: any local edit makes the
// comparison fail, which is what keeps customized scripts safe.
func isPackagedLegacyScript(content []byte) bool {
	entries, err := legacyInitFS.ReadDir("selfupdate_init/legacy")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		legacy, err := legacyInitFS.ReadFile("selfupdate_init/legacy/" + entry.Name())
		if err != nil {
			continue
		}
		if bytes.Equal(content, legacy) {
			return true
		}
	}
	return false
}

// rewriteInitScript replaces the init script with the canonical one, keeping
// a versioned backup of the previous content.
func rewriteInitScript(existing, desired []byte, reason, currentVersion string) {
	backup := fmt.Sprintf("%s.bak-%s", initScriptPath, strings.TrimPrefix(currentVersion, "v"))
	if err := os.WriteFile(backup, existing, 0755); err != nil {
		log.Printf("netgrip: %s: backup init script: %v", reason, err)
		return
	}
	if err := writeInitScript(desired); err != nil {
		log.Printf("netgrip: %s: update init script: %v", reason, err)
		return
	}
	log.Printf("netgrip: %s: init script updated (backup at %s)", reason, backup)
}

func writeInitScript(content []byte) error {
	return os.WriteFile(initScriptPath, content, 0755)
}
