package modules

import (
	"bytes"
	"embed"
	"log"
	"os"
	"path/filepath"
)

// The helper scripts travel inside the apk/ipk package, and self-update only
// replaces the binary, so a router that updates from its own panel kept them
// frozen at the version of the last package install. The heal script is the
// case that bit: it restores the package registration after an attended
// sysupgrade, and a fix in it could not reach the devices it exists for
// (#577). Embedding them puts the same convergence the init script already
// has (#433, #492, #544) behind both scripts.
//
// The embedded copies are kept in sync with the packaged originals
// (deploy/openwrt/netgrip/files/) by a unit test. The legacy directories hold
// every variant the package shipped, so a pristine old copy is recognized as
// ours and rewritten, while a hand-edited one matches nothing and is left
// untouched.

//go:embed selfupdate_helpers
var helperFS embed.FS

// helperMarker identifies a helper script written by us.
const helperMarker = "# netgrip helper - managed"

// helperScripts are the packaged helpers that must converge with the binary.
var helperScripts = []string{"netgrip-heal-register", "netgrip-restore-rules"}

// helperScriptDir is a package-level variable so tests can point it at a
// t.TempDir() directory.
var helperScriptDir = "/usr/libexec"

func canonicalHelperScript(name string) ([]byte, error) {
	return helperFS.ReadFile("selfupdate_helpers/" + name)
}

// isPackagedLegacyHelper reports whether content byte-matches a variant the
// package shipped before the managed marker existed.
func isPackagedLegacyHelper(name string, content []byte) bool {
	entries, err := helperFS.ReadDir("selfupdate_helpers/legacy/" + name)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		legacy, err := helperFS.ReadFile("selfupdate_helpers/legacy/" + name + "/" + entry.Name())
		if err != nil {
			continue
		}
		if bytes.Equal(content, legacy) {
			return true
		}
	}
	return false
}

// SyncHelperScriptsAtBoot converges the packaged helpers on every process
// start, so a device that only ever receives manually copied binaries or
// self-updates its binary still runs the current scripts.
func SyncHelperScriptsAtBoot(currentVersion string) {
	if _, err := os.Stat(openwrtReleasePath); err != nil {
		return
	}
	syncHelperScripts("startup", currentVersion)
}

// syncHelperScripts refreshes the helpers before the post-update restart.
// Best effort: a failure here must not brick an update whose binary is
// already in place.
func syncHelperScripts(reason, currentVersion string) {
	for _, name := range helperScripts {
		desired, err := canonicalHelperScript(name)
		if err != nil {
			log.Printf("netgrip: %s: embedded %s unreadable: %v", reason, name, err)
			continue
		}
		ensureManagedScript(managedScript{
			path:    filepath.Join(helperScriptDir, name),
			desired: desired,
			marker:  helperMarker,
			legacyMatch: func(content []byte) bool {
				return isPackagedLegacyHelper(name, content)
			},
		}, reason, currentVersion)
	}
}
