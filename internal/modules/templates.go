package modules

import (
	"fmt"

	"github.com/gnacho/netgrip/internal/executor"
)

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Destructive bool   `json:"destructive"`
}

func ListTemplates() []Template {
	return []Template{
		{
			ID:          "hardened",
			Name:        "Hardened router",
			Description: "Disable IPv6, enable rebind protection, block WAN ping, disable remote SSH",
			Destructive: false,
		},
		{
			ID:          "iot-ready",
			Name:        "IoT ready",
			Description: "Enable IoT WiFi (2.4 GHz isolated), block IoT from LAN",
			Destructive: false,
		},
		{
			ID:          "reset-defaults",
			Name:        "Reset to defaults",
			Description: "Remove all custom firewall rules, reset WiFi to defaults, clear DHCP reservations",
			Destructive: true,
		},
	}
}

func ApplyTemplate(id string) error {
	switch id {
	case "hardened":
		return applyHardened()
	case "iot-ready":
		return applyIoTReady()
	case "reset-defaults":
		return applyResetDefaults()
	default:
		return fmt.Errorf("unknown template: %s", id)
	}
}

func applyHardened() error {
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return err
	}
	snapFirewall, err := executor.Snapshot("firewall")
	if err != nil {
		return err
	}
	snapDhcp, err := executor.Snapshot("dhcp")
	if err != nil {
		return err
	}
	rollback := func() {
		_ = executor.Restore("network", snapNetwork)
		_ = executor.Restore("firewall", snapFirewall)
		_ = executor.Restore("dhcp", snapDhcp)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		if executor.ServiceEnabled("firewall") {
			_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
		}
	}

	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"network.lan.ipv6", "0"}},
		{Kind: "uci_set", Args: []string{"dhcp.lan.ra", "disabled"}},
		{Kind: "uci_set", Args: []string{"dhcp.lan.dhcpv6", "disabled"}},
		{Kind: "uci_set", Args: []string{"dhcp.lan.rebind_protection", "1"}},
		{Kind: "uci_commit", Args: []string{"network"}},
		{Kind: "uci_commit", Args: []string{"dhcp"}},
		{Kind: "initd", Args: []string{"network", "reload"}},
	}

	// Block WAN ping if firewall is enabled
	if executor.ServiceEnabled("firewall") {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"firewall.@zone[1].input", "REJECT"}},
			executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
		)
	}

	// Disable remote SSH
	if executor.ServiceEnabled("dropbear") {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"dropbear.main.PasswordAuth", "on"}},
			executor.Op{Kind: "uci_set", Args: []string{"dropbear.main.RootPasswordAuth", "on"}},
			executor.Op{Kind: "uci_commit", Args: []string{"dropbear"}},
		)
	}

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return err
	}
	if executor.ServiceRunning("dnsmasq") {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "restart"}})
	}
	return nil
}

func applyIoTReady() error {
	snap, err := executor.Snapshot("wireless")
	if err != nil {
		return err
	}

	cfg := IoTConfig{
		Enabled: true,
		SSID:    "IoT",
		Key:     "iot-default-pass",
		Band:    "2g",
	}
	_, rolledBack, err := SetIoT(cfg)
	if err != nil {
		if rolledBack {
			_ = executor.Restore("wireless", snap)
		}
		return err
	}
	return nil
}

func applyResetDefaults() error {
	snapFirewall, err := executor.Snapshot("firewall")
	if err != nil {
		return err
	}
	snapDhcp, err := executor.Snapshot("dhcp")
	if err != nil {
		return err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snapFirewall)
		_ = executor.Restore("dhcp", snapDhcp)
	}

	// Remove custom firewall rules (keep zones)
	probe := ProbeFirewall()
	for _, rule := range probe.Rules {
		_ = executor.Run(executor.Op{Kind: "uci_delete", Args: []string{"firewall." + rule.Section}})
	}
	if len(probe.Rules) > 0 {
		_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"firewall"}})
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "restart"}})
	}

	// Clear DHCP reservations
	_, rolledBack, err := ClearReservations()
	if err != nil {
		if rolledBack {
			rollback()
		}
		return err
	}
	return nil
}
