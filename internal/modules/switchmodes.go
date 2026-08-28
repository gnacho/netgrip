package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type SwitchMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func ListSwitchModes() []SwitchMode {
	return []SwitchMode{
		{
			ID:          "trunk-uplink",
			Name:        "Router-on-a-stick (trunk)",
			Description: "First port as tagged trunk, all others as access on VLAN 1",
		},
		{
			ID:          "extender",
			Name:        "Extender (flat bridge)",
			Description: "All ports bridged on VLAN 1, no routing or VLAN segmentation",
		},
		{
			ID:          "segmented-home",
			Name:        "Segmented home",
			Description: "Predefined VLANs: LAN(1), IoT(10), Guest(20), Camera(30). Ports 1-2 LAN, 3 IoT, 4 guest, uplink trunk",
		},
	}
}

func ApplySwitchMode(id string, uplinkPort string) error {
	switch id {
	case "trunk-uplink":
		return applyTrunkUplink(uplinkPort)
	case "extender":
		return applyExtender()
	case "segmented-home":
		return applySegmentedHome(uplinkPort)
	default:
		return fmt.Errorf("unknown switch mode: %s", id)
	}
}

func applyTrunkUplink(uplink string) error {
	if uplink == "" {
		return fmt.Errorf("uplink port required")
	}
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return err
	}
	ports := bridgePortList()

	var ops []executor.Op
	// Remove all existing bridge-vlan sections
	existing := parseBridgeVlanSections()
	for _, sec := range existing {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + sec}})
	}

	// VLAN 1: all ports except uplink as untagged, uplink as tagged
	for _, p := range ports {
		val := p
		if p == uplink {
			val = p + ":t"
		}
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network.vlan1_default.ports", val}})
	}

	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snapNetwork)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return err
	}
	return nil
}

func applyExtender() error {
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return err
	}
	ports := bridgePortList()

	var ops []executor.Op
	existing := parseBridgeVlanSections()
	for _, sec := range existing {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + sec}})
	}

	// Single VLAN with all ports untagged
	for _, p := range ports {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network.vlan1_default.ports", p}})
	}

	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snapNetwork)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return err
	}
	return nil
}

func applySegmentedHome(uplink string) error {
	if uplink == "" {
		return fmt.Errorf("uplink port required")
	}
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return err
	}
	ports := bridgePortList()

	var ops []executor.Op
	existing := parseBridgeVlanSections()
	for _, sec := range existing {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + sec}})
	}

	// VLAN 1 (LAN): ports 1-2 + uplink tagged
	// VLAN 10 (IoT): port 3 + uplink tagged
	// VLAN 20 (Guest): port 4 + uplink tagged
	vlanAssign := map[string][]string{
		"1":  {},
		"10": {},
		"20": {},
	}
	for i, p := range ports {
		if p == uplink {
			continue
		}
		switch {
		case i < 2:
			vlanAssign["1"] = append(vlanAssign["1"], p)
		case i == 2:
			vlanAssign["10"] = append(vlanAssign["10"], p)
		case i == 3:
			vlanAssign["20"] = append(vlanAssign["20"], p)
		default:
			vlanAssign["1"] = append(vlanAssign["1"], p)
		}
	}

	for vid, members := range vlanAssign {
		secName := "netgrip_vlan_" + vid
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network." + secName, "bridge-vlan"}})
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network." + secName + ".device", "br-lan"}})
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network." + secName + ".vlan", vid}})
		for _, p := range members {
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + secName + ".ports", p}})
		}
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + secName + ".ports", uplink + ":t"}})
	}

	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snapNetwork)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return err
	}
	return nil
}

func parseBridgeVlanSections() []string {
	out, err := exec.Command("uci", "show", "network").Output()
	if err != nil {
		return nil
	}
	sections := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".type='bridge-vlan'") {
			sec := strings.TrimPrefix(line, "network.")
			sec = strings.TrimSuffix(sec, ".type='bridge-vlan'")
			sections[sec] = true
		}
	}
	var result []string
	for s := range sections {
		result = append(result, s)
	}
	return result
}
