package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// IPv6Probe is the read-only state of IPv6 on the router.
type IPv6Probe struct {
	State          string `json:"state"` // enabled | disabled | partial
	LanIPv6        string `json:"lan_ipv6"`
	WanIPv6        string `json:"wan_ipv6"`
	OdhcpdEnabled  bool   `json:"odhcpd_enabled"`
	OdhcpdRunning  bool   `json:"odhcpd_running"`
	RaMode         string `json:"ra_mode"`
	Dhcpv6Mode     string `json:"dhcpv6_mode"`
	DnsmasqRunning bool   `json:"dnsmasq_running"`
	HasWan         bool   `json:"has_wan"`
}

func uciGet(key string) string {
	out, err := exec.Command("uci", "-q", "get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func uciSectionExists(key string) bool {
	return exec.Command("uci", "-q", "get", key).Run() == nil
}

// ProbeIPv6 reads the current IPv6 state. State is "disabled" only when the
// relevant knobs are consistently off; "enabled" when consistently on;
// anything else is "partial" so the user sees the real mix.
func ProbeIPv6() *IPv6Probe {
	p := &IPv6Probe{
		LanIPv6:        uciGet("network.lan.ipv6"),
		OdhcpdEnabled:  executor.ServiceEnabled("odhcpd"),
		OdhcpdRunning:  executor.ServiceRunning("odhcpd"),
		RaMode:         uciGet("dhcp.lan.ra"),
		Dhcpv6Mode:     uciGet("dhcp.lan.dhcpv6"),
		DnsmasqRunning: executor.ServiceRunning("dnsmasq"),
		HasWan:         uciSectionExists("network.wan"),
	}
	if p.HasWan {
		p.WanIPv6 = uciGet("network.wan.ipv6")
	}
	raOff := p.RaMode == "disabled" || p.RaMode == ""
	dhcpv6Off := p.Dhcpv6Mode == "disabled" || p.Dhcpv6Mode == ""
	switch {
	case p.LanIPv6 == "0" && !p.OdhcpdEnabled && raOff && dhcpv6Off:
		p.State = "disabled"
	case p.LanIPv6 != "0" && p.OdhcpdEnabled:
		p.State = "enabled"
	default:
		p.State = "partial"
	}
	return p
}

// SetIPv6 applies the desired state with snapshot + healthcheck + rollback.
// Returns the resulting probe, whether a rollback happened, and any error.
func SetIPv6(enable bool) (*IPv6Probe, bool, error) {
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot network: %w", err)
	}
	snapDhcp, err := executor.Snapshot("dhcp")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot dhcp: %w", err)
	}
	wasOdhcpdEnabled := executor.ServiceEnabled("odhcpd")
	wasOdhcpdRunning := executor.ServiceRunning("odhcpd")
	wasDnsmasqRunning := executor.ServiceRunning("dnsmasq")

	ops := ipv6Ops(enable)
	if err := executor.Apply(ops, nil); err != nil {
		rollbackIPv6(snapNetwork, snapDhcp, wasOdhcpdEnabled, wasOdhcpdRunning, wasDnsmasqRunning)
		return ProbeIPv6(), true, err
	}

	// Healthcheck: the probe must reflect the desired state.
	probe := ProbeIPv6()
	if enable && probe.State != "enabled" || !enable && probe.State != "disabled" {
		rollbackIPv6(snapNetwork, snapDhcp, wasOdhcpdEnabled, wasOdhcpdRunning, wasDnsmasqRunning)
		return ProbeIPv6(), true, fmt.Errorf("healthcheck failed after apply (state=%s), rolled back", probe.State)
	}
	return probe, false, nil
}

func ipv6Ops(enable bool) []executor.Op {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	if enable {
		set("network.lan.ipv6", "1")
		if uciSectionExists("network.wan") {
			set("network.wan.ipv6", "1")
		}
		set("dhcp.lan.ra", "server")
		set("dhcp.lan.dhcpv6", "server")
		ops = append(ops,
			executor.Op{Kind: "uci_commit", Args: []string{"network"}},
			executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
			executor.Op{Kind: "initd", Args: []string{"odhcpd", "enable"}},
			executor.Op{Kind: "initd", Args: []string{"odhcpd", "start"}},
			executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
		)
	} else {
		set("network.lan.ipv6", "0")
		if uciSectionExists("network.wan") {
			set("network.wan.ipv6", "0")
		}
		set("dhcp.lan.ra", "disabled")
		set("dhcp.lan.dhcpv6", "disabled")
		ops = append(ops,
			executor.Op{Kind: "uci_commit", Args: []string{"network"}},
			executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
			executor.Op{Kind: "initd", Args: []string{"odhcpd", "disable"}},
			executor.Op{Kind: "initd", Args: []string{"odhcpd", "stop"}},
			executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
		)
	}
	if executor.ServiceRunning("dnsmasq") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"dnsmasq", "restart"}})
	}
	return ops
}

func rollbackIPv6(snapNetwork, snapDhcp string, odhcpdEnabled, odhcpdRunning, dnsmasqRunning bool) {
	_ = executor.Restore("network", snapNetwork)
	_ = executor.Restore("dhcp", snapDhcp)
	if odhcpdEnabled {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"odhcpd", "enable"}})
	} else {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"odhcpd", "disable"}})
	}
	if odhcpdRunning {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"odhcpd", "start"}})
	} else {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"odhcpd", "stop"}})
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	if dnsmasqRunning {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "restart"}})
	}
}
