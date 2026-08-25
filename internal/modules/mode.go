package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/owpanel/internal/executor"
)

// ModeProbe reports whether the router is a gateway/router or an access
// point, using the real state: a WAN interface inside the LAN bridge
// (dnsmasq and firewall off) is an AP reconverting the WAN into a LAN
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

// SetMode converts the router between Router (gateway) and AP modes. This is
// destructive: it changes the network identity and may drop the current
// management path, so it snapshots network/dhcp/firewall, runs a healthcheck
// and rolls back on failure. The caller must confirm explicitly.
func SetMode(target string) (*ModeProbe, bool, error) {
	probe := ProbeMode()
	if target != "router" && target != "ap" {
		return probe, false, fmt.Errorf("target must be router or ap")
	}
	if probe.Mode == target {
		return probe, false, fmt.Errorf("router is already in %s mode", target)
	}

	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return probe, false, fmt.Errorf("snapshot network: %w", err)
	}
	snapDhcp, err := executor.Snapshot("dhcp")
	if err != nil {
		return probe, false, fmt.Errorf("snapshot dhcp: %w", err)
	}
	snapFirewall, err := executor.Snapshot("firewall")
	if err != nil {
		return probe, false, fmt.Errorf("snapshot firewall: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("network", snapNetwork)
		_ = executor.Restore("dhcp", snapDhcp)
		_ = executor.Restore("firewall", snapFirewall)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "restart"}})
		if executor.ServiceEnabled("firewall") {
			_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
		}
	}

	ops, err := modeOps(target, probe)
	if err != nil {
		return probe, false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeMode(), true, err
	}

	// Healthcheck: the router must remain reachable on the LAN. On Router
	// mode the lan keeps its static IP; on AP mode it becomes a DHCP client,
	// so we only require the bridge to have an address (from either source).
	if !routerAlive() {
		rollback()
		return ProbeMode(), true, fmt.Errorf("router unreachable after mode change, rolled back")
	}
	after := ProbeMode()
	if after.Mode != target {
		rollback()
		return ProbeMode(), true, fmt.Errorf("mode did not change as expected, rolled back")
	}
	return after, false, nil
}

// routerAlive checks the router is still on the network after a mode change:
// br-lan must be up and carry an IPv4 address.
func routerAlive() bool {
	resolved, err := exec.Command("sh", "-c", "ip -4 -o addr show br-lan | grep -q 'inet '").Output()
	return err == nil || len(resolved) > 0
}

func modeOps(target string, probe *ModeProbe) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	add := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{key, value}})
	}
	del := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{key, value}})
	}

	if target == "router" {
		// Gate: restore WAN as a real interface (DHCP client), pull it out of
		// the LAN bridge, and enable dnsmasq + firewall.
		del("network.@device[0].ports", "wan")
		set("network.wan", "interface")
		set("network.wan.device", "wan")
		set("network.wan.proto", "dhcp")
		ops = append(ops,
			executor.Op{Kind: "initd", Args: []string{"dnsmasq", "enable"}},
			executor.Op{Kind: "initd", Args: []string{"dnsmasq", "start"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "enable"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "restart"}},
		)
	} else { // ap
		// Drop the WAN interface and merge the WAN port back into the bridge;
		// the lan becomes a DHCP client of the new gateway.
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network.wan"}})
		add("network.@device[0].ports", "wan")
		set("network.lan.device", "br-lan")
		if uciGet("network.lan.proto") == "static" {
			set("network.lan.proto", "dhcp")
		}
		ops = append(ops,
			executor.Op{Kind: "initd", Args: []string{"dnsmasq", "disable"}},
			executor.Op{Kind: "initd", Args: []string{"dnsmasq", "stop"}},
			executor.Op{Kind: "initd", Args: []string{"dnsmasq", "restart"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "disable"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "stop"}},
		)
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"network", "restart"}},
	)
	return ops, nil
}

// WanPortActive reports whether the port named "wan" is a real WAN port
// (i.e. the router is in router mode). On AP mode the WAN port is a LAN
// port and must not be presented as WAN.
func WanPortActive() bool {
	return ProbeMode().Mode == "router"
}
