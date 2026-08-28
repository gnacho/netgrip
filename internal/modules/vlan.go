package modules

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type VLANPort struct {
	Port   string `json:"port"`
	Tagged bool   `json:"tagged"`
}

type VLAN struct {
	VID      int        `json:"vid"`
	Name     string     `json:"name"`
	Device   string     `json:"device"`
	Ports    []VLANPort `json:"ports"`
	Default  bool       `json:"default"`
}

type VLANProbe struct {
	Applicable bool   `json:"applicable"`
	Bridge     string `json:"bridge"`
	VLANs      []VLAN `json:"vlans"`
	Ports      []string `json:"ports"`
}

func ProbeVLANs() *VLANProbe {
	ports := bridgePortList()
	if len(ports) == 0 {
		return &VLANProbe{Applicable: false}
	}
	bridge := "br-lan"
	out, err := exec.Command("uci", "show", "network").Output()
	if err != nil {
		return &VLANProbe{Applicable: false, Bridge: bridge, Ports: ports}
	}
	sections := parseBridgeVlans(string(out), bridge)
	sort.Slice(sections, func(i, j int) bool { return sections[i].VID < sections[j].VID })
	return &VLANProbe{
		Applicable: true,
		Bridge:     bridge,
		VLANs:      sections,
		Ports:      ports,
	}
}

func parseBridgeVlans(show, bridge string) []VLAN {
	type rawVLAN struct {
		section string
		vid     int
		ports   []string
	}
	sectionType := map[string]string{}
	sectionVID := map[string]int{}
	sectionPorts := map[string][]string{}

	for _, line := range strings.Split(show, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "network.") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "'")

		if strings.HasSuffix(key, ".type") && val == "bridge-vlan" {
			sec := strings.TrimPrefix(strings.TrimSuffix(key, ".type"), "network.")
			sectionType[sec] = "bridge-vlan"
		}
		if strings.HasSuffix(key, ".device") {
			sec := strings.TrimPrefix(strings.TrimSuffix(key, ".device"), "network.")
			if sectionType[sec] == "bridge-vlan" && val == bridge {
				sectionVID[sec] = sectionVID[sec]
			}
		}
		if strings.HasSuffix(key, ".vlan") {
			sec := strings.TrimPrefix(strings.TrimSuffix(key, ".vlan"), "network.")
			if v, err := strconv.Atoi(val); err == nil {
				sectionVID[sec] = v
			}
		}
		if strings.Contains(key, ".ports=") || strings.HasSuffix(key, ".ports") {
			sec := strings.TrimPrefix(key, "network.")
			sec = strings.TrimSuffix(sec, ".ports")
			sectionPorts[sec] = append(sectionPorts[sec], val)
		}
	}

	var vlans []VLAN
	for sec, typ := range sectionType {
		if typ != "bridge-vlan" {
			continue
		}
		vid := sectionVID[sec]
		if vid == 0 {
			continue
		}
		var vports []VLANPort
		for _, p := range sectionPorts[sec] {
			tagged := strings.HasSuffix(p, ":t")
			name := strings.TrimSuffix(p, ":t")
			vports = append(vports, VLANPort{Port: name, Tagged: tagged})
		}
		vlans = append(vlans, VLAN{
			VID:     vid,
			Name:    fmt.Sprintf("VLAN %d", vid),
			Device:  bridge,
			Ports:   vports,
			Default: vid == 1,
		})
	}
	return vlans
}

func bridgePortList() []string {
	ports := bridgePorts()
	var list []string
	for p := range ports {
		if strings.HasPrefix(p, "phy") || strings.HasPrefix(p, "wlan") {
			continue
		}
		list = append(list, p)
	}
	sort.Strings(list)
	return list
}

type VLANEdit struct {
	VID   int        `json:"vid"`
	Ports []VLANPort `json:"ports"`
}

func SetVLAN(edit VLANEdit) (*VLANProbe, bool, error) {
	if edit.VID < 1 || edit.VID > 4094 {
		return nil, false, fmt.Errorf("VLAN ID must be 1-4094")
	}
	snap, err := executor.Snapshot("network")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot network: %w", err)
	}

	existing := findVLANSection(edit.VID)
	if existing == "" {
		return createVLAN(edit, snap)
	}
	return updateVLAN(existing, edit, snap)
}

func findVLANSection(vid int) string {
	out, err := exec.Command("uci", "show", "network").Output()
	if err != nil {
		return ""
	}
	sections := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".type='bridge-vlan'") {
			sec := strings.TrimPrefix(line, "network.")
			sec = strings.TrimSuffix(sec, ".type='bridge-vlan'")
			sections[sec] = 0
		}
	}
	for _, line := range strings.Split(string(out), "\n") {
		for sec := range sections {
			if strings.HasSuffix(line, ".vlan='"+strconv.Itoa(vid)+"'") &&
				strings.Contains(line, "network."+sec+".vlan=") {
				return sec
			}
		}
	}
	return ""
}

func createVLAN(edit VLANEdit, snap string) (*VLANProbe, bool, error) {
	cmd := exec.Command("uci", "add", "network", "bridge-vlan")
	out, err := cmd.Output()
	if err != nil {
		return ProbeVLANs(), false, fmt.Errorf("uci add bridge-vlan: %w", err)
	}
	section := strings.TrimSpace(string(out))

	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"network." + section + ".device", "br-lan"}},
		{Kind: "uci_set", Args: []string{"network." + section + ".vlan", strconv.Itoa(edit.VID)}},
	}
	for _, p := range edit.Ports {
		val := p.Port
		if p.Tagged {
			val += ":t"
		}
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + section + ".ports", val}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return ProbeVLANs(), true, err
	}

	executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	return ProbeVLANs(), false, nil
}

func updateVLAN(section string, edit VLANEdit, snap string) (*VLANProbe, bool, error) {
	delOps := []executor.Op{
		{Kind: "uci_delete", Args: []string{"network." + section + ".ports"}},
		{Kind: "uci_commit", Args: []string{"network"}},
	}
	if err := executor.Apply(delOps, nil); err != nil {
		return ProbeVLANs(), false, fmt.Errorf("clear ports: %w", err)
	}

	var ops []executor.Op
	for _, p := range edit.Ports {
		val := p.Port
		if p.Tagged {
			val += ":t"
		}
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + section + ".ports", val}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return ProbeVLANs(), true, err
	}

	executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	return ProbeVLANs(), false, nil
}

func DeleteVLAN(vid int) (*VLANProbe, bool, error) {
	if vid == 1 {
		return nil, false, fmt.Errorf("cannot delete default VLAN")
	}
	section := findVLANSection(vid)
	if section == "" {
		return nil, false, fmt.Errorf("VLAN %d not found", vid)
	}
	snap, err := executor.Snapshot("network")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot network: %w", err)
	}
	ops := []executor.Op{
		{Kind: "uci_delete", Args: []string{"network." + section}},
		{Kind: "uci_commit", Args: []string{"network"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return ProbeVLANs(), true, err
	}
	executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	return ProbeVLANs(), false, nil
}
