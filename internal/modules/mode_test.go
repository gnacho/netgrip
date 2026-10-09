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

func TestIsGatewayByUplinkNames(t *testing.T) {
	cases := []struct {
		name                string
		ifaceNames          []string
		activeName          string
		activeSectionExists bool
		want                bool
	}{
		{"wan section settles it even when lan carries the route", []string{"loopback", "lan", "wan"}, "lan", true, true},
		{"wan6-only uplink still a gateway", []string{"lan", "wan6"}, "lan", true, true},
		{"multiwan wanb counts", []string{"lan", "wan", "wanb"}, "wan", true, true},
		{"renamed uplink without wan section wins via route", []string{"lan", "isp"}, "isp", true, true},
		{"dhcp-managed lan is not a gateway (switch)", []string{"loopback", "lan"}, "lan", true, false},
		{"dhcp-managed lan with radios is not a gateway (ap)", []string{"lan"}, "lan", true, false},
		{"no uplink at all", []string{"lan"}, "wan", false, false},
		{"empty active name is not a gateway", []string{"lan"}, "", false, false},
	}
	for _, c := range cases {
		if got := isGatewayByUplinkNames(c.ifaceNames, c.activeName, c.activeSectionExists); got != c.want {
			t.Errorf("%s: isGatewayByUplinkNames = %v, want %v", c.name, got, c.want)
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
