package modules

import (
	"fmt"
	"os/exec"
	"strings"
)

type WizardState struct {
	Completed bool   `json:"completed"`
	Mode      string `json:"mode"`
}

func ProbeWizard() *WizardState {
	completed := false
	if out, err := exec.Command("uci", "-q", "get", "netgrip.wizard.completed").Output(); err == nil {
		completed = strings.TrimSpace(string(out)) == "1"
	}
	mode := ProbeMode().Mode
	return &WizardState{Completed: completed, Mode: mode}
}

func CompleteWizard() error {
	if !uciSectionExists("netgrip.wizard") {
		if !uciSectionExists("netgrip.main") {
			cmd := exec.Command("uci", "import", "netgrip")
			cmd.Stdin = strings.NewReader("config panel 'main'\n\toption panel 'panel'\nconfig wizard 'wizard'\n\toption completed '0'\n")
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("init netgrip wizard config: %s", strings.TrimSpace(string(out)))
			}
		} else {
			cmd := exec.Command("uci", "import", "netgrip")
			cmd.Stdin = strings.NewReader("config wizard 'wizard'\n\toption completed '0'\n")
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("add wizard section: %s", strings.TrimSpace(string(out)))
			}
		}
	}
	out, err := exec.Command("uci", "set", "netgrip.wizard.completed=1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("set wizard completed: %s", strings.TrimSpace(string(out)))
	}
	return exec.Command("uci", "commit", "netgrip").Run()
}
