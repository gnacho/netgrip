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

func ensureWizardSection() error {
	if uciSectionExists("netgrip.wizard") {
		return nil
	}
	if !uciSectionExists("netgrip.main") {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config panel 'main'\n\toption panel 'panel'\nconfig wizard 'wizard'\n\toption completed '0'\n")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("init netgrip wizard config: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	cmd := exec.Command("uci", "import", "netgrip")
	cmd.Stdin = strings.NewReader("config wizard 'wizard'\n\toption completed '0'\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add wizard section: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func setWizardCompleted(v string) error {
	if err := ensureWizardSection(); err != nil {
		return err
	}
	out, err := exec.Command("uci", "set", "netgrip.wizard.completed="+v).CombinedOutput()
	if err != nil {
		return fmt.Errorf("set wizard completed: %s", strings.TrimSpace(string(out)))
	}
	return exec.Command("uci", "commit", "netgrip").Run()
}

func CompleteWizard() error {
	return setWizardCompleted("1")
}

// ResetWizard marks the first-run wizard as pending again so it can be
// relaunched from the panel (#415). No other setting is touched.
func ResetWizard() error {
	return setWizardCompleted("0")
}
