package modules

import "testing"

func TestComputeRole(t *testing.T) {
	cases := []struct {
		name string
		p    *ModeProbe
		want string
	}{
		{"wan uplink is a router", &ModeProbe{WanConfigured: true}, "router"},
		{"wan in bridge is not a router", &ModeProbe{WanConfigured: true, WanInBridge: true, HasWifi: true}, "ap"},
		{"no wan with radios is an ap", &ModeProbe{HasWifi: true}, "ap"},
		{"no wan no radios is a switch regardless of port count", &ModeProbe{PortCount: 5}, "switch"},
		{"no wan no radios few ports still a switch", &ModeProbe{PortCount: 2}, "switch"},
	}
	for _, c := range cases {
		if got := computeRole(c.p); got != c.want {
			t.Errorf("%s: computeRole = %q, want %q", c.name, got, c.want)
		}
	}
}

// ProbeMode wires the computed role into the probe; on a host without ubus
// it degrades to the ap/switch default, which must still carry a role.
func TestProbeModeAlwaysSetsRole(t *testing.T) {
	if p := ProbeMode(); p.Role == "" {
		t.Errorf("ProbeMode role is empty (probe: %+v)", p)
	}
}
