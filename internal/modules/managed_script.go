package modules

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// managedScript is a file the binary owns and converges on every start and
// before a self-update restart. The package ships the same file, so the two
// paths have to agree: the binary is the one that arrives first on a device
// that only ever self-updates, and its embedded copy is the newest one.
type managedScript struct {
	path    string
	desired []byte
	// marker identifies a file written by us. A file without it is only
	// rewritten when legacyMatch proves it is a pristine packaged variant.
	marker      string
	legacyMatch func(content []byte) bool
}

// ensureManagedScript converges one script, following three rules: an intact
// copy is never touched, a script of ours (marker or pristine packaged
// variant) is replaced with a versioned backup, and a customized script is
// left alone.
func ensureManagedScript(s managedScript, reason, currentVersion string) {
	existing, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := writeManagedScript(s); err != nil {
			log.Printf("netgrip: %s: install %s: %v", reason, s.path, err)
			return
		}
		log.Printf("netgrip: %s: installed missing %s", reason, s.path)
	case err != nil:
		log.Printf("netgrip: %s: read %s: %v", reason, s.path, err)
	case bytes.Equal(existing, s.desired):
		// Already current: do not rewrite, so the mtime stays.
	case !strings.Contains(string(existing), s.marker) && !(s.legacyMatch != nil && s.legacyMatch(existing)):
		log.Printf("netgrip: %s: %s has no %q marker and matches no packaged variant, leaving the customized script untouched",
			reason, s.path, s.marker)
	default:
		backup := fmt.Sprintf("%s.bak-%s", s.path, strings.TrimPrefix(currentVersion, "v"))
		if err := os.WriteFile(backup, existing, 0755); err != nil {
			log.Printf("netgrip: %s: backup %s: %v", reason, s.path, err)
			return
		}
		if err := writeManagedScript(s); err != nil {
			log.Printf("netgrip: %s: update %s: %v", reason, s.path, err)
			return
		}
		log.Printf("netgrip: %s: %s updated (backup at %s)", reason, s.path, backup)
	}
}

// writeManagedScript replaces the file in one step. These scripts run from
// boot, so a half-written copy is not an option: write a sibling temporary
// file, set the mode and rename it into place.
func writeManagedScript(s managedScript) error {
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(s.desired); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
