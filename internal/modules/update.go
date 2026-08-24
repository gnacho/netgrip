package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// UpdateCheck is the parsed result of `owut check`.
type UpdateCheck struct {
	Available      bool     `json:"available"`
	SameVersion    bool     `json:"same_version"`
	OwutPresent    bool     `json:"owut_present"`
	VersionFrom    string   `json:"version_from"`
	VersionTo      string   `json:"version_to"`
	OutOfDatePkgs  int      `json:"out_of_date_packages"`
	Warnings       []string `json:"warnings"`
	SafeToProceed  bool     `json:"safe_to_proceed"`
	UpgradeStarted bool     `json:"upgrade_started"`
}

var (
	reVersionFrom = regexp.MustCompile(`(?m)^Version-from\s+(.+)$`)
	reVersionTo   = regexp.MustCompile(`(?m)^Version-to\s+(.+)$`)
	reOutOfDate   = regexp.MustCompile(`(?m)^(\d+) packages are out-of-date$`)
	reSafe        = regexp.MustCompile(`(?m)^It is safe to proceed`)
)

// CheckUpdate runs `owut check` (read-only) and parses the report.
func CheckUpdate() *UpdateCheck {
	check := &UpdateCheck{Warnings: []string{}}
	out, err := exec.Command("owut", "check").CombinedOutput()
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
	check.SafeToProceed = reSafe.MatchString(text)
	check.SameVersion = check.VersionFrom == "" || check.VersionTo == "" || coreVersion(check.VersionFrom) == coreVersion(check.VersionTo)
	check.Available = !check.SameVersion || check.OutOfDatePkgs > 0
	return check
}

// coreVersion strips the trailing metadata (" (kernel ...)") so two
// revisions of the same release compare equal.
func coreVersion(v string) string {
	if i := strings.Index(v, " ("); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return strings.TrimSpace(v)
}

// StartUpgrade launches `owut upgrade -q` detached. The router WILL reboot
// when it finishes; the panel process dies with it (it runs from /tmp during
// the beta, or gets reinstalled as a package with ASU keeping it).
func StartUpgrade() error {
	cmd := exec.Command("setsid", "owut", "upgrade", "-q")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start owut upgrade: %w", err)
	}
	return nil
}
