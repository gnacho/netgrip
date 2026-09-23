package modules

import (
	"os/exec"
)

// RebootRouter requests a system reboot. The reboot is delayed briefly so the
// caller can finish what it was doing (an MQTT result publish, an HTTP reply)
// before the system goes down. It returns an error only when the request could
// not be started.
//
// Used by POST /api/reboot (#399) and by the MQTT "reboot" command (#398).
func RebootRouter() error {
	cmd := exec.Command("sh", "-c", "sleep 1; reboot")
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the child so it does not stay as a zombie if the reboot somehow
	// does not happen immediately; the subshell outlives our process.
	go func() { _ = cmd.Wait() }()
	return nil
}
