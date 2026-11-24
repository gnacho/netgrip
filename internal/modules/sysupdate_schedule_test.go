package modules

import "testing"

func TestSysUpdateConfigWindow(t *testing.T) {
	c := SysUpdateConfig{Enabled: true, IntervalH: 168, WindowStart: 3, WindowEnd: 5}
	for h, want := range map[int]bool{2: false, 3: true, 4: true, 5: false, 23: false} {
		if got := c.inWindow(h); got != want {
			t.Errorf("inWindow(%d) = %v, want %v", h, got, want)
		}
	}
	// Ventana que cruza medianoche.
	w := SysUpdateConfig{WindowStart: 23, WindowEnd: 2}
	for h, want := range map[int]bool{22: false, 23: true, 0: true, 1: true, 2: false} {
		if got := w.inWindow(h); got != want {
			t.Errorf("noche inWindow(%d) = %v, want %v", h, got, want)
		}
	}
}

// La decisión es la parte peligrosa: un upgrade de firmware reinicia el
// router, así que solo se aplica cuando owut lo da por seguro.
func TestSysUpdateAction(t *testing.T) {
	cases := []struct {
		name      string
		check     *UpdateCheck
		action    string
		reinstall bool
	}{
		{"sin datos", nil, "error", false},
		{"al dia", &UpdateCheck{Available: false}, "up-to-date", false},
		{"misma version", &UpdateCheck{Available: true, SameVersion: true}, "up-to-date", false},
		{"seguro", &UpdateCheck{Available: true, SafeToProceed: true}, "apply", false},
		{"solo con reinstall", &UpdateCheck{Available: true, SafeWithReinstall: true}, "apply", true},
		{"seguro y reinstall", &UpdateCheck{Available: true, SafeToProceed: true, SafeWithReinstall: true}, "apply", false},
		{"bloqueado", &UpdateCheck{Available: true}, "blocked", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			action, reinstall := sysUpdateAction(tc.check)
			if action != tc.action || reinstall != tc.reinstall {
				t.Fatalf("got (%s, %v), want (%s, %v)", action, reinstall, tc.action, tc.reinstall)
			}
		})
	}
}
