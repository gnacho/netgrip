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
	// PVID marks the port's ingress VLAN - the "*" in UCI's "lan2:u*".
	// Writing an untagged port without it strands every untagged frame the
	// port receives, which silently kills that port's network (#328), so it
	// is round-tripped instead of being dropped on write.
	PVID bool `json:"pvid,omitempty"`
}

type VLAN struct {
	VID     int        `json:"vid"`
	Name    string     `json:"name"`
	Device  string     `json:"device"`
	Ports   []VLANPort `json:"ports"`
	Default bool       `json:"default"`
}

type VLANProbe struct {
	Applicable bool     `json:"applicable"`
	Bridge     string   `json:"bridge"`
	VLANs      []VLAN   `json:"vlans"`
	Ports      []string `json:"ports"`
}

func ProbeVLANs() *VLANProbe {
	ports := bridgePortList()
	if len(ports) == 0 {
		return &VLANProbe{Applicable: false, VLANs: []VLAN{}, Ports: []string{}}
	}
	bridge := LANBridge()
	show, err := uciShowNetwork()
	if err != nil {
		return &VLANProbe{Applicable: false, Bridge: bridge, VLANs: []VLAN{}, Ports: ports}
	}
	sections := parseUCIShow(show, "network")
	vlans := collectVLANs(sections, bridge)
	if vlans == nil {
		vlans = []VLAN{}
	}
	sort.Slice(vlans, func(i, j int) bool { return vlans[i].VID < vlans[j].VID })
	return &VLANProbe{
		Applicable: true,
		Bridge:     bridge,
		VLANs:      vlans,
		Ports:      ports,
	}
}

// parseVlanPort parses one bridge-vlan `ports` entry: "<port>[:u|:t][*]",
// where ":t" is tagged, ":u" untagged and a trailing "*" marks the port's
// PVID (its ingress VLAN) - e.g. "lan2:u*", "lan2:t", "lan2".
func parseVlanPort(raw string) VLANPort {
	name := raw
	pvid := false
	if strings.HasSuffix(name, "*") {
		pvid = true
		name = strings.TrimSuffix(name, "*")
	}
	switch {
	case strings.HasSuffix(name, ":t"):
		return VLANPort{Port: strings.TrimSuffix(name, ":t"), Tagged: true, PVID: pvid}
	case strings.HasSuffix(name, ":u"):
		return VLANPort{Port: strings.TrimSuffix(name, ":u"), PVID: pvid}
	default:
		return VLANPort{Port: name, PVID: pvid}
	}
}

// formatVlanPort renders a port back into UCI's "<port>[:u|:t][*]" form.
func formatVlanPort(p VLANPort) string {
	if p.Tagged {
		return p.Port + ":t"
	}
	if p.PVID {
		return p.Port + ":u*"
	}
	return p.Port + ":u"
}

// vlanSections returns the bridge-vlan sections attached to bridge, keyed
// by UCI section name, with their VLAN id and parsed ports.
func vlanSections(sections map[string]uciSection, bridge string) map[string]VLAN {
	out := map[string]VLAN{}
	for name, s := range sections {
		if s.Type != "bridge-vlan" || s.Option("device") != bridge {
			continue
		}
		vid, err := strconv.Atoi(s.Option("vlan"))
		if err != nil || vid == 0 {
			continue
		}
		ports := make([]VLANPort, 0, len(s.Options["ports"]))
		for _, raw := range s.Options["ports"] {
			// Los bridges DSA escriben los puertos como UN solo valor
			// separado por espacios ("option ports 'lan1 lan2 ...'"), no
			// como entradas de lista; sin el split la VLAN entera colapsaba
			// en un pseudo-puerto (#485, verificado en un switch de 52
			// bocas). Cada token sigue el formato "<port>[:u|:t][*]".
			for _, one := range strings.Fields(raw) {
				ports = append(ports, parseVlanPort(one))
			}
		}
		out[name] = VLAN{VID: vid, Name: fmt.Sprintf("VLAN %d", vid), Device: bridge, Ports: ports}
	}
	return out
}

func collectVLANs(sections map[string]uciSection, bridge string) []VLAN {
	protected := protectedVIDs(sections, bridge)
	var vlans []VLAN
	for _, v := range vlanSections(sections, bridge) {
		v.Default = protected[v.VID]
		vlans = append(vlans, v)
	}
	return vlans
}

// protectedVIDs returns the VLANs that carry the router's own LAN, which
// must not be edited or deleted through the VLAN manager: touching them
// re-tags or strands the port the admin's session arrives on and cuts
// access to the router (#327). Derived, never hardcoded - the LAN is on
// VLAN 10 here, VLAN 1 elsewhere, and on a differently named bridge again
// elsewhere:
//   - lan sits on "<bridge>.<vid>"  -> that vid
//   - lan sits on the bridge itself -> every VLAN holding a port's PVID,
//     since those are the untagged VLANs carrying it
//   - lan is not on this bridge     -> nothing to protect here
func protectedVIDs(sections map[string]uciSection, bridge string) map[int]bool {
	protected := map[int]bool{}
	lan, ok := sections["lan"]
	if !ok {
		return protected
	}
	device := lan.Option("device")
	if vid, err := strconv.Atoi(strings.TrimPrefix(device, bridge+".")); err == nil &&
		strings.HasPrefix(device, bridge+".") {
		protected[vid] = true
		return protected
	}
	if device != bridge {
		return protected
	}
	for _, v := range vlanSections(sections, bridge) {
		for _, p := range v.Ports {
			if !p.Tagged && p.PVID {
				protected[v.VID] = true
				break
			}
		}
	}
	return protected
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
	bridge := LANBridge()
	show, err := uciShowNetwork()
	if err != nil {
		return nil, false, fmt.Errorf("read network config: %w", err)
	}
	sections := parseUCIShow(show, "network")
	if protectedVIDs(sections, bridge)[edit.VID] {
		return nil, false, fmt.Errorf("VLAN %d carries this router's own LAN and cannot be edited here", edit.VID)
	}
	ports, err := normalizeVLANPorts(edit, sections, bridge)
	if err != nil {
		return nil, false, err
	}
	if err := checkNotStranding(edit.VID, ports, sections, bridge); err != nil {
		return nil, false, err
	}

	snap, err := executor.Snapshot("network")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot network: %w", err)
	}

	section := findVLANSection(sections, bridge, edit.VID)
	if section == "" {
		return createVLAN(bridge, edit.VID, ports, snap)
	}
	return updateVLAN(section, ports, len(sections[section].Options["ports"]) > 0, snap)
}

// normalizeVLANPorts validates a requested membership. The caller's
// untagged/tagged/PVID choice is honoured as sent - the UI offers all of
// them, as LuCI does - but a port may be egress-untagged in only one VLAN:
// two untagged VLANs on one port emit indistinguishable frames, and the
// second one silently takes over the port the first was serving.
func normalizeVLANPorts(edit VLANEdit, sections map[string]uciSection, bridge string) ([]VLANPort, error) {
	untaggedOwner := map[string]int{}
	for _, v := range vlanSections(sections, bridge) {
		if v.VID == edit.VID {
			continue
		}
		for _, p := range v.Ports {
			if !p.Tagged {
				untaggedOwner[p.Port] = v.VID
			}
		}
	}
	out := make([]VLANPort, 0, len(edit.Ports))
	for _, p := range edit.Ports {
		if p.Tagged {
			// A tagged member never carries the PVID.
			p.PVID = false
			out = append(out, p)
			continue
		}
		if owner, taken := untaggedOwner[p.Port]; taken {
			return nil, fmt.Errorf("port %s is already untagged in VLAN %d; a port can be untagged in only one VLAN", p.Port, owner)
		}
		out = append(out, p)
	}
	return out, nil
}

// checkNotStranding refuses to remove the last port of a VLAN that still
// has an IP interface bound to it: the network would stay configured and
// addressed but reach nothing (#328 - a single click emptied VLAN 16 and
// took the camera network down).
func checkNotStranding(vid int, ports []VLANPort, sections map[string]uciSection, bridge string) error {
	if len(ports) > 0 {
		return nil
	}
	device := fmt.Sprintf("%s.%d", bridge, vid)
	for name, s := range sections {
		if s.Type == "interface" && s.Option("device") == device {
			return fmt.Errorf("VLAN %d still carries the %q network; remove its last port only after that network is gone", vid, name)
		}
	}
	return nil
}

// findVLANSectionByVID resolves a VLAN's UCI section from the live config,
// for callers that don't already hold a parsed one.
func findVLANSectionByVID(vid int) string {
	show, err := uciShowNetwork()
	if err != nil {
		return ""
	}
	return findVLANSection(parseUCIShow(show, "network"), LANBridge(), vid)
}

func findVLANSection(sections map[string]uciSection, bridge string, vid int) string {
	names := make([]string, 0)
	for name, v := range vlanSections(sections, bridge) {
		if v.VID == vid {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func createVLAN(bridge string, vid int, ports []VLANPort, snap string) (*VLANProbe, bool, error) {
	out, err := exec.Command("uci", "add", "network", "bridge-vlan").Output()
	if err != nil {
		return ProbeVLANs(), false, fmt.Errorf("uci add bridge-vlan: %w", err)
	}
	section := strings.TrimSpace(string(out))

	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"network." + section + ".device", bridge}},
		{Kind: "uci_set", Args: []string{"network." + section + ".vlan", strconv.Itoa(vid)}},
	}
	for _, p := range ports {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + section + ".ports", formatVlanPort(p)}})
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

// vlanPortOps builds the ops that replace a VLAN's port list. The clear is
// emitted only when there is a list to clear: `uci delete` fails outright
// when the option isn't there, and a VLAN created without ports has none -
// which failed the very first edit after creating one.
func vlanPortOps(section string, ports []VLANPort, hasPorts bool) []executor.Op {
	var ops []executor.Op
	if hasPorts {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + section + ".ports"}})
	}
	for _, p := range ports {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + section + ".ports", formatVlanPort(p)}})
	}
	return append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})
}

func updateVLAN(section string, ports []VLANPort, hasPorts bool, snap string) (*VLANProbe, bool, error) {
	ops := vlanPortOps(section, ports, hasPorts)

	// One batch: a failure half way through must not leave the ports list
	// cleared, which is what the old clear-then-refill pair could do.
	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return ProbeVLANs(), true, err
	}

	executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	return ProbeVLANs(), false, nil
}

func DeleteVLAN(vid int) (*VLANProbe, bool, error) {
	bridge := LANBridge()
	show, err := uciShowNetwork()
	if err != nil {
		return nil, false, fmt.Errorf("read network config: %w", err)
	}
	sections := parseUCIShow(show, "network")
	if protectedVIDs(sections, bridge)[vid] {
		return nil, false, fmt.Errorf("VLAN %d carries this router's own LAN and cannot be deleted", vid)
	}
	section := findVLANSection(sections, bridge, vid)
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
