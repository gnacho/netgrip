package modules

import "testing"

func TestValidAdGuardActions(t *testing.T) {
	for _, a := range []string{"start", "stop"} {
		if !validAdGuardActions[a] {
			t.Errorf("action %q must be valid", a)
		}
	}
	for _, a := range []string{"", "restart", "reload", "enable", "rm -rf", "START"} {
		if validAdGuardActions[a] {
			t.Errorf("action %q must be rejected", a)
		}
	}
}
