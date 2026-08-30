package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// PkgManager is the router's package manager.
type PkgManager string

const (
	PkgManagerAPK  PkgManager = "apk"
	PkgManagerOPKG PkgManager = "opkg"
)

// SetupGroup is one package group offered during the first-run wizard.
type SetupGroup struct {
	ID       string   `json:"id"`
	TitleKey string   `json:"title_key"`
	Packages []string `json:"packages"`
}

var setupGroups = []SetupGroup{
	{
		ID:       "core",
		TitleKey: "wizard.setup.required",
		Packages: []string{"curl", "ca-certificates", "rpcd-mod-file"},
	},
	{
		ID:       "netpulse",
		TitleKey: "wizard.setup.netpulse",
		Packages: []string{"tailscale"},
	},
	{
		ID:       "diagnostics",
		TitleKey: "wizard.setup.diagnostics",
		Packages: []string{"ethtool-full", "tcpdump-mini"},
	},
	{
		ID:       "extras",
		TitleKey: "wizard.setup.extras",
		Packages: []string{"sqm-scripts", "nlbwmon"},
	},
}

// DetectPkgManager returns the package manager available on this router.
func DetectPkgManager() PkgManager {
	if _, err := exec.LookPath("apk"); err == nil {
		return PkgManagerAPK
	}
	return PkgManagerOPKG
}

// ProbeSetupPackages returns the groups with each package marked as installed or not.
func ProbeSetupPackages() []SetupGroup {
	out := make([]SetupGroup, len(setupGroups))
	for i, g := range setupGroups {
		out[i] = SetupGroup{ID: g.ID, TitleKey: g.TitleKey, Packages: []string{}}
		for _, p := range g.Packages {
			if !pkgInstalled(p) {
				out[i].Packages = append(out[i].Packages, p)
			}
		}
	}
	return out
}

// SetupMode selects which groups are installed.
type SetupMode string

const (
	SetupModeFull    SetupMode = "full"
	SetupModeMinimal SetupMode = "minimal"
	SetupModeCustom  SetupMode = "custom"
)

// InstallSetupPackages installs the packages for the selected mode.
// It returns the package names that were actually installed.
func InstallSetupPackages(mode SetupMode, customIDs []string) ([]string, error) {
	var groups []SetupGroup
	switch mode {
	case SetupModeMinimal:
		groups = []SetupGroup{setupGroups[0]} // core only
	case SetupModeFull:
		groups = setupGroups
	case SetupModeCustom:
		for _, id := range customIDs {
			for _, g := range setupGroups {
				if g.ID == id {
					groups = append(groups, g)
				}
			}
		}
	}

	var toInstall []string
	for _, g := range groups {
		for _, p := range g.Packages {
			if !pkgInstalled(p) {
				toInstall = append(toInstall, p)
			}
		}
	}
	if len(toInstall) == 0 {
		return []string{}, nil
	}

	if err := executor.Run(executor.Op{Kind: "pkg_add", Args: toInstall}); err != nil {
		return nil, err
	}

	installed := []string{}
	for _, p := range toInstall {
		if pkgInstalled(p) {
			installed = append(installed, p)
		}
	}
	return installed, nil
}

// WizardSetupChoice stores the user's choice so the step is not shown again.
func WizardSetupChoice(mode SetupMode) error {
	if !uciSectionExists("netgrip.wizard") {
		_ = CompleteWizard() // ensures the section exists
	}
	modeStr := strings.TrimSpace(string(mode))
	if modeStr == "" {
		modeStr = "skipped"
	}
	out, err := exec.Command("uci", "set", "netgrip.wizard.setup_choice="+modeStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("uci set: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return exec.Command("uci", "commit", "netgrip").Run()
}
