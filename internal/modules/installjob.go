package modules

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/gnacho/netgrip/internal/executor"
)

// InstallPhase is the lifecycle phase of the async package install job.
type InstallPhase string

const (
	InstallPhaseIdle       InstallPhase = "idle"
	InstallPhaseUpdating   InstallPhase = "updating"
	InstallPhaseInstalling InstallPhase = "installing"
	InstallPhaseDone       InstallPhase = "done"
	InstallPhaseError      InstallPhase = "error"
)

// InstallProgress is a point-in-time snapshot of the install job (polled by
// the UI). One job is active at a time (single-router panel).
type InstallProgress struct {
	Phase     InstallPhase `json:"phase"`
	Total     int          `json:"total"`
	Done      int          `json:"done"`
	Current   string       `json:"current,omitempty"`
	Installed []string     `json:"installed,omitempty"`
	Error     string       `json:"error,omitempty"`
}

var installJob struct {
	mu    sync.Mutex
	state InstallProgress
}

// JobStatus returns a copy of the current install progress.
func JobStatus() InstallProgress {
	installJob.mu.Lock()
	defer installJob.mu.Unlock()
	s := installJob.state
	s.Installed = append([]string(nil), installJob.state.Installed...)
	return s
}

func setInstallJob(s InstallProgress) {
	installJob.mu.Lock()
	installJob.state = s
	installJob.mu.Unlock()
}

// StartInstallPackages launches an async install of the given packages (one at
// a time) and returns immediately. On opkg it refreshes the indexes first.
func StartInstallPackages(pkgs []string) InstallProgress {
	total := len(pkgs)
	setInstallJob(InstallProgress{Phase: InstallPhaseUpdating, Total: total, Installed: []string{}})

	go func() {
		// opkg needs fresh feed indexes; apk does not.
		if _, err := exec.LookPath("apk"); err != nil {
			setInstallJob(InstallProgress{Phase: InstallPhaseUpdating, Total: total, Current: "opkg update", Installed: []string{}})
			if out, err := exec.Command("opkg", "update").CombinedOutput(); err != nil {
				failInstall(fmt.Sprintf("opkg update: %s", strings.TrimSpace(string(out))))
				return
			}
		}

		setInstallJob(InstallProgress{Phase: InstallPhaseInstalling, Total: total, Installed: []string{}})
		for i, pkg := range pkgs {
			setInstallJob(InstallProgress{Phase: InstallPhaseInstalling, Total: total, Done: i, Current: pkg, Installed: installedSnapshot()})
			if err := executor.Run(executor.Op{Kind: "pkg_add", Args: []string{pkg}}); err != nil {
				failInstall(fmt.Sprintf("install %s: %s", pkg, err))
				return
			}
			if pkgInstalled(pkg) {
				installJob.mu.Lock()
				installJob.state.Done = i + 1
				installJob.state.Installed = append(installJob.state.Installed, pkg)
				installJob.mu.Unlock()
			} else {
				setInstallJob(InstallProgress{Phase: InstallPhaseInstalling, Total: total, Done: i + 1, Current: pkg, Installed: installedSnapshot()})
			}
		}

		setInstallJob(InstallProgress{Phase: InstallPhaseDone, Total: total, Done: total, Installed: installedSnapshot()})
	}()

	return JobStatus()
}

func installedSnapshot() []string {
	installJob.mu.Lock()
	defer installJob.mu.Unlock()
	return append([]string(nil), installJob.state.Installed...)
}

func failInstall(msg string) {
	setInstallJob(InstallProgress{Phase: InstallPhaseError, Error: msg, Installed: installedSnapshot()})
}
