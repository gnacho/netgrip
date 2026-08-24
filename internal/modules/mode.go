package modules

import (
	"os/exec"
	"strings"

	"github.com/gnacho/owpanel/internal/executor"
)

// ModeProbe reports whether the router is a gateway/router or an access
// point, using the real state: a WAN interface inside the LAN bridge
// (dnsmasq and firewall off) is an AP reconvolving the WAN into a LAN
// port.
type ModeProbe struct {
	Mode          string `json:"mode"` // router | ap
	WanInBridge   bool   `json:"wan_in_bridge"`
	WanConfigured bool   `json:"wan_configured"`
	DnsmasqOn     bool   `json:"dnsmasq_on"`
	FirewallOn    bool   `json:"firewall_on"`
}

// ProbeMode detects the router mode.
func ProbeMode() *ModeProbe {
	p := &ModeProbe{
		WanConfigured: uciSectionExists("network.wan"),
		DnsmasqOn:     executor.ServiceEnabled("dnsmasq"),
		FirewallOn:    executor.ServiceEnabled("firewall"),
	}
	out, err := exec.Command("ip", "-o", "link", "show", "wan").Output()
	if err == nil && strings.Contains(string(out), "master br-lan") {
		p.WanInBridge = true
	}
	switch {
	case p.WanInBridge:
		p.Mode = "ap"
	case p.WanConfigured:
		p.Mode = "router"
	default:
		p.Mode = "ap"
	}
	return p
}

// WanPortActive reports whether the port named "wan" is a real WAN port
// (i.e. the router is in router mode). On AP mode the WAN port is a LAN
// port and must not be presented as WAN.
func WanPortActive() bool {
	return ProbeMode().Mode == "router"
}
