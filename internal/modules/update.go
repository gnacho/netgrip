package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// UpdateCheck is the parsed result of `owut check`.
type UpdateCheck struct {
	Available         bool     `json:"available"`
	SameVersion       bool     `json:"same_version"`
	OwutPresent       bool     `json:"owut_present"`
	VersionFrom       string   `json:"version_from"`
	VersionTo         string   `json:"version_to"`
	OutOfDatePkgs     int      `json:"out_of_date_packages"`
	Warnings          []string `json:"warnings"`
	SafeToProceed     bool     `json:"safe_to_proceed"`
	MissingPackages   []string `json:"missing_packages"`
	SafeWithReinstall bool     `json:"safe_with_reinstall"`
	UpgradeStarted    bool     `json:"upgrade_started"`
}

var (
	reVersionFrom  = regexp.MustCompile(`(?m)^Version-from\s+(.+)$`)
	reVersionTo    = regexp.MustCompile(`(?m)^Version-to\s+(.+)$`)
	reOutOfDate    = regexp.MustCompile(`(?m)^(\d+) packages are out-of-date$`)
	reSafe         = regexp.MustCompile(`(?m)^It is safe to proceed`)
	reMissingPkg   = regexp.MustCompile(`(?m)^\s*(\S+)\s+\S+\s+missing to-version$`)
	reMissingCount = regexp.MustCompile(`(?m)^(\d+) packages missing in target version`)
)

// CheckUpdate runs `owut check -v` (read-only) and parses the report.
// The verbose variant also lists the packages missing in the target
// version, which is what lets us detect the "only netgrip is missing"
// case (locally installed package, unknown to the ASU server).
func CheckUpdate() *UpdateCheck {
	check := &UpdateCheck{Warnings: []string{}, MissingPackages: []string{}}
	out, err := exec.Command("owut", "check", "-v").CombinedOutput()
	if err != nil && len(out) == 0 {
		check.OwutPresent = false
		return check
	}
	check.OwutPresent = true
	text := string(out)
	if m := reVersionFrom.FindStringSubmatch(text); m != nil {
		check.VersionFrom = strings.TrimSpace(m[1])
	}
	if m := reVersionTo.FindStringSubmatch(text); m != nil {
		check.VersionTo = strings.TrimSpace(m[1])
	}
	if m := reOutOfDate.FindStringSubmatch(text); m != nil {
		check.OutOfDatePkgs, _ = strconv.Atoi(m[1])
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "WARNING:") {
			check.Warnings = append(check.Warnings, strings.TrimSpace(strings.TrimPrefix(line, "WARNING:")))
		}
	}
	for _, m := range reMissingPkg.FindAllStringSubmatch(text, -1) {
		check.MissingPackages = append(check.MissingPackages, m[1])
	}
	check.SafeToProceed = reSafe.MatchString(text)
	// Safe with reinstall: the ONLY packages unknown to the ASU server are
	// netgrip itself. Those can be excluded from the image build
	// (owut -r netgrip) and reinstalled on first boot by the rc.local hook.
	check.SafeWithReinstall = !check.SafeToProceed && len(check.MissingPackages) > 0 && onlyNetgrip(check.MissingPackages)
	check.SameVersion = check.VersionFrom == "" || check.VersionTo == "" || coreVersion(check.VersionFrom) == coreVersion(check.VersionTo)
	check.Available = !check.SameVersion || check.OutOfDatePkgs > 0
	return check
}

func onlyNetgrip(pkgs []string) bool {
	if len(pkgs) == 0 {
		return false
	}
	for _, p := range pkgs {
		if p != "netgrip" {
			return false
		}
	}
	return true
}

// coreVersion strips the trailing metadata (" (kernel ...)") so two
// revisions of the same release compare equal.
func coreVersion(v string) string {
	if i := strings.Index(v, " ("); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return strings.TrimSpace(v)
}

// StartUpgrade launches `owut upgrade -q` detached. When the only blocker
// is the locally installed netgrip package (unknown to the ASU server),
// it is excluded from the build with -r netgrip and restored from the
// files preserved by sysupgrade (see EnsureSurvival).
func StartUpgrade(withReinstall bool) error {
	if withReinstall {
		if err := EnsureSurvival(); err != nil {
			return fmt.Errorf("ensure survival: %w", err)
		}
	}
	args := []string{"owut", "upgrade", "-q"}
	if withReinstall {
		args = append(args, "-r", "netgrip")
	}
	cmd := exec.Command("setsid", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start owut upgrade: %w", err)
	}
	return nil
}

const sysupgradeConf = "/etc/sysupgrade.conf"

// survivalFiles are the netgrip files preserved across a sysupgrade so the
// panel comes back on first boot even though the apk package registry does
// not survive: the binary, the init script and the autostart symlink.
// Fully local: no download, no feed, works regardless of repo visibility.
var survivalFiles = []string{
	"/usr/sbin/netgrip",
	"/etc/init.d/netgrip",
	"/etc/rc.d/S99netgrip",
}

// EnsureSurvival lists the netgrip files in /etc/sysupgrade.conf so a
// sysupgrade (manual or via owut/ASU) preserves them and procd starts the
// panel on first boot. Idempotent; safe to call any time. The apk package
// registry itself does not survive, so a later `apk add` of a newer
// release reinstalls cleanly on top.
func EnsureSurvival() error {
	for _, f := range survivalFiles {
		if err := ensureLine(sysupgradeConf, f, ""); err != nil {
			return fmt.Errorf("sysupgrade.conf: %w", err)
		}
	}
	return nil
}

// ensureLine appends a line to a file when not already present. When
// beforeMarker is set and found, the line is inserted before it (rc.local
// ends with "exit 0" and the hook must run before that).
func ensureLine(path, line, beforeMarker string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	var b strings.Builder
	inserted := false
	for _, l := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		if beforeMarker != "" && !inserted && strings.TrimSpace(l) == beforeMarker {
			b.WriteString(line + "\n")
			inserted = true
		}
		if l != "" || inserted {
			b.WriteString(l + "\n")
		}
	}
	if !inserted {
		b.WriteString(line + "\n")
		if beforeMarker != "" {
			b.WriteString(beforeMarker + "\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
