package modules

import "testing"

func TestCollectVLANsFindsAllSections(t *testing.T) {
	vlans := collectVLANs(sectionsForTest(), "br-lan")
	if len(vlans) != 3 {
		t.Fatalf("len(vlans) = %d, want 3 (got %+v)", len(vlans), vlans)
	}
	seen := map[int]bool{}
	for _, v := range vlans {
		seen[v.VID] = true
	}
	for _, vid := range []int{10, 11, 16} {
		if !seen[vid] {
			t.Fatalf("VLAN %d missing from %+v", vid, vlans)
		}
	}
}

// Los bridges DSA escriben `option ports 'lan1 lan2 ...'` como un solo
// valor; sin el split por espacios la VLAN entera se colapsaba en un
// pseudo-puerto con todos los nombres pegados (#485, switch de 52 bocas).
func TestCollectVLANsSplitsSpaceSeparatedPorts(t *testing.T) {
	show := `network.lan_vlan=bridge-vlan
network.lan_vlan.device='switch'
network.lan_vlan.vlan='1'
network.lan_vlan.ports='lan1 lan2 lan3 lan49 lan52'
`
	vlans := collectVLANs(parseUCIShow(show, "network"), "switch")
	if len(vlans) != 1 {
		t.Fatalf("len(vlans) = %d, want 1", len(vlans))
	}
	want := []string{"lan1", "lan2", "lan3", "lan49", "lan52"}
	if len(vlans[0].Ports) != len(want) {
		t.Fatalf("ports = %+v, want %d entries", vlans[0].Ports, len(want))
	}
	for i, p := range vlans[0].Ports {
		if p.Port != want[i] || p.Tagged {
			t.Fatalf("ports[%d] = %+v, want %s sin tag", i, p, want[i])
		}
	}
}

func TestParseVlanPortFlags(t *testing.T) {
	cases := []struct {
		in   string
		want VLANPort
	}{
		{"lan2:u*", VLANPort{Port: "lan2", Tagged: false, PVID: true}},
		{"lan2:t", VLANPort{Port: "lan2", Tagged: true}},
		{"lan2:u", VLANPort{Port: "lan2"}},
		{"lan2", VLANPort{Port: "lan2"}},
	}
	for _, c := range cases {
		if got := parseVlanPort(c.in); got != c.want {
			t.Fatalf("parseVlanPort(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// The PVID marker must survive a parse/write round trip: dropping it turns
// an access port into one that strands every untagged frame it receives.
func TestVlanPortRoundTripKeepsPVID(t *testing.T) {
	for _, raw := range []string{"lan2:u*", "lan2:t", "lan2:u"} {
		if got := formatVlanPort(parseVlanPort(raw)); got != raw {
			t.Fatalf("round trip of %q = %q", raw, got)
		}
	}
	// A bare port is untagged without PVID, and normalizes to the explicit form.
	if got := formatVlanPort(parseVlanPort("lan2")); got != "lan2:u" {
		t.Fatalf("round trip of bare lan2 = %q, want lan2:u", got)
	}
}

func TestProtectedVIDsFollowsLANVlan(t *testing.T) {
	protected := protectedVIDs(sectionsForTest(), "br-lan")
	if !protected[10] {
		t.Fatalf("VLAN 10 (network.lan is on br-lan.10) must be protected, got %+v", protected)
	}
	for _, vid := range []int{11, 16} {
		if protected[vid] {
			t.Fatalf("VLAN %d must stay editable, got %+v", vid, protected)
		}
	}
}

// When the LAN interface sits on the bridge itself rather than a VLAN
// sub-interface, the untagged/PVID VLANs are the ones carrying it.
func TestProtectedVIDsWhenLANIsOnBridgeItself(t *testing.T) {
	show := `network.@device[0]=device
network.@device[0].name='br0'
network.@device[0].type='bridge'
network.lan=interface
network.lan.device='br0'
network.@bridge-vlan[0]=bridge-vlan
network.@bridge-vlan[0].device='br0'
network.@bridge-vlan[0].vlan='1'
network.@bridge-vlan[0].ports='lan1:u*'
network.@bridge-vlan[1]=bridge-vlan
network.@bridge-vlan[1].device='br0'
network.@bridge-vlan[1].vlan='7'
network.@bridge-vlan[1].ports='lan1:t'
`
	protected := protectedVIDs(parseUCIShow(show, "network"), "br0")
	if !protected[1] || protected[7] {
		t.Fatalf("protected = %+v, want {1:true} only", protected)
	}
}

// The UI offers untagged with and without PVID separately, as LuCI does,
// so the requested combination has to survive untouched.
func TestNormalizeVLANPortsHonoursRequestedFlags(t *testing.T) {
	cases := []struct {
		name string
		in   VLANPort
		want string
	}{
		{"untagged and the port default", VLANPort{Port: "lan3", PVID: true}, "lan3:u*"},
		{"untagged only", VLANPort{Port: "lan3"}, "lan3:u"},
		{"tagged", VLANPort{Port: "lan3", Tagged: true}, "lan3:t"},
		{"tagged never carries the PVID", VLANPort{Port: "lan3", Tagged: true, PVID: true}, "lan3:t"},
	}
	for _, c := range cases {
		ports, err := normalizeVLANPorts(VLANEdit{VID: 11, Ports: []VLANPort{c.in}}, sectionsForTest(), "br-lan")
		if err != nil {
			t.Fatalf("%s: normalizeVLANPorts: %v", c.name, err)
		}
		if got := formatVlanPort(ports[0]); got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Switching a port from tagged to untagged and back must not be blocked:
// the cycle-based UI could only leave the tagged state by removing the
// port, which is a dead end on a VLAN whose last port is protected.
func TestNormalizeVLANPortsAllowsTaggedToUntaggedOnAFreePort(t *testing.T) {
	edit := VLANEdit{VID: 11, Ports: []VLANPort{{Port: "lan3", PVID: true}}}
	if _, err := normalizeVLANPorts(edit, sectionsForTest(), "br-lan"); err != nil {
		t.Fatalf("lan3 is untagged nowhere else; the change must be allowed: %v", err)
	}
}

// lan2 is already the untagged/PVID port of VLAN 10; making it untagged in
// VLAN 11 too would silently steal the port's ingress VLAN.
func TestNormalizeVLANPortsRejectsSecondUntaggedVLAN(t *testing.T) {
	edit := VLANEdit{VID: 11, Ports: []VLANPort{{Port: "lan2"}}}
	if _, err := normalizeVLANPorts(edit, sectionsForTest(), "br-lan"); err == nil {
		t.Fatal("expected a conflict error for a second untagged VLAN on lan2")
	}
}

func TestNormalizeVLANPortsAllowsTaggedOnPVIDPort(t *testing.T) {
	edit := VLANEdit{VID: 11, Ports: []VLANPort{{Port: "lan2", Tagged: true}}}
	ports, err := normalizeVLANPorts(edit, sectionsForTest(), "br-lan")
	if err != nil {
		t.Fatalf("tagged membership must be allowed alongside another VLAN's PVID: %v", err)
	}
	if ports[0].PVID {
		t.Fatalf("tagged port must not carry PVID: %+v", ports[0])
	}
}

// Emptying VLAN 16 would leave the "guests" network (br-lan.16) addressed
// but unreachable - that is what took the guests network down in practice.
func TestCheckNotStrandingBlocksEmptyingAVLANWithANetwork(t *testing.T) {
	err := checkNotStranding(16, nil, sectionsForTest(), "br-lan")
	if err == nil {
		t.Fatal("expected emptying VLAN 16 to be refused: network.guests is on br-lan.16")
	}
}

func TestCheckNotStrandingAllowsVLANWithoutNetwork(t *testing.T) {
	if err := checkNotStranding(11, nil, sectionsForTest(), "br-lan"); err != nil {
		t.Fatalf("VLAN 11 has no interface bound; emptying it must be allowed: %v", err)
	}
}

func TestCheckNotStrandingAllowsRemovingOneOfSeveralPorts(t *testing.T) {
	ports := []VLANPort{{Port: "lan2", Tagged: true}}
	if err := checkNotStranding(16, ports, sectionsForTest(), "br-lan"); err != nil {
		t.Fatalf("removing a port while others remain must be allowed: %v", err)
	}
}

// A VLAN created empty has no "ports" option at all, and `uci delete` on a
// missing option fails - so the first edit after creating one must not try
// to clear a list that was never written.
func TestVlanPortOpsSkipsTheClearWhenThereIsNoPortList(t *testing.T) {
	ops := vlanPortOps("@bridge-vlan[7]", []VLANPort{{Port: "lan2", Tagged: true}}, false)
	for _, op := range ops {
		if op.Kind == "uci_delete" {
			t.Fatalf("no ports list exists yet; ops must not delete one: %+v", ops)
		}
	}
	if ops[0].Kind != "uci_add_list" || ops[0].Args[1] != "lan2:t" {
		t.Fatalf("ops[0] = %+v, want the tagged port added first", ops[0])
	}
}

func TestVlanPortOpsClearsAnExistingPortList(t *testing.T) {
	ops := vlanPortOps("@bridge-vlan[1]", []VLANPort{{Port: "lan2", Tagged: true}}, true)
	if ops[0].Kind != "uci_delete" || ops[0].Args[0] != "network.@bridge-vlan[1].ports" {
		t.Fatalf("ops[0] = %+v, want the existing list cleared first", ops[0])
	}
}

// The empty VLAN 18 the user hit this on: parsed from a real config, its
// section carries device and vlan but no ports option.
func TestVLANCreatedWithoutPortsHasNoPortList(t *testing.T) {
	show := uciShowNetworkWithVlans + `network.@bridge-vlan[7]=bridge-vlan
network.@bridge-vlan[7].device='br-lan'
network.@bridge-vlan[7].vlan='18'
`
	sections := parseUCIShow(show, "network")
	if len(sections["@bridge-vlan[7]"].Options["ports"]) != 0 {
		t.Fatalf("VLAN 18 must parse with no ports, got %+v", sections["@bridge-vlan[7]"].Options["ports"])
	}
	if got := findVLANSection(sections, "br-lan", 18); got != "@bridge-vlan[7]" {
		t.Fatalf("findVLANSection(18) = %q, want @bridge-vlan[7]", got)
	}
}

func TestFindVLANSectionMatchesExistingVID(t *testing.T) {
	if got := findVLANSection(sectionsForTest(), "br-lan", 16); got != "@bridge-vlan[2]" {
		t.Fatalf("section for VID 16 = %q, want @bridge-vlan[2]", got)
	}
	if got := findVLANSection(sectionsForTest(), "br-lan", 99); got != "" {
		t.Fatalf("section for absent VID 99 = %q, want empty", got)
	}
}

// A bridge under a different name must work exactly the same.
func TestVLANsOnRenamedBridge(t *testing.T) {
	show := `network.@device[0]=device
network.@device[0].name='br-home'
network.@device[0].type='bridge'
network.lan=interface
network.lan.device='br-home.5'
network.@bridge-vlan[0]=bridge-vlan
network.@bridge-vlan[0].device='br-home'
network.@bridge-vlan[0].vlan='5'
network.@bridge-vlan[0].ports='lan1:u*'
network.@bridge-vlan[1]=bridge-vlan
network.@bridge-vlan[1].device='br-home'
network.@bridge-vlan[1].vlan='6'
network.@bridge-vlan[1].ports='lan1:t'
`
	sections := parseUCIShow(show, "network")
	if got := resolveBridge(sections); got != "br-home" {
		t.Fatalf("resolveBridge = %q, want br-home", got)
	}
	vlans := collectVLANs(sections, "br-home")
	if len(vlans) != 2 {
		t.Fatalf("len(vlans) = %d, want 2", len(vlans))
	}
	if !protectedVIDs(sections, "br-home")[5] {
		t.Fatal("VLAN 5 carries the LAN on br-home and must be protected")
	}
}
