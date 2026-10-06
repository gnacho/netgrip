package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// Advanced mode (#441): the opt-in flag that unlocks the Advanced section.
// The flag only gates UI and API visibility; it never changes how the
// router works, so applying it is a plain netgrip config write with the
// standard snapshot-and-restore of /etc/config/netgrip.
type AdvancedProbe struct {
	Advanced bool `json:"advanced"`
}

// ProbeAdvanced reports whether advanced mode is on. A missing option reads
// as off, so routers that never opted in see no change at all.
func ProbeAdvanced() *AdvancedProbe {
	out, err := exec.Command("uci", "-q", "get", "netgrip.main.advanced").Output()
	if err != nil {
		return &AdvancedProbe{Advanced: false}
	}
	return &AdvancedProbe{Advanced: strings.TrimSpace(string(out)) == "1"}
}

// SetAdvanced turns advanced mode on or off. There is nothing to healthcheck
// (no service restarts on a visibility flag), so the safety net is the
// snapshot-and-restore around the config write, same as the other panel
// settings in netgrip.main.
func SetAdvanced(enabled bool) (*AdvancedProbe, error) {
	if err := applyNetgripConfig([]executor.Op{
		{Kind: "uci_set", Args: []string{"netgrip.main.advanced", boolOption(enabled)}},
	}); err != nil {
		return nil, fmt.Errorf("set advanced mode: %w", err)
	}
	return ProbeAdvanced(), nil
}
