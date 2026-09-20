package modules

import (
	"strings"
	"testing"
)

// `uci show network` shape from a board with 802.1Q VLAN filtering:
// bridge-vlan sections render their type as the VALUE of the bare section
// line (network.@bridge-vlan[0]=bridge-vlan), never as a ".type" attribute
// - the old parser looked for a ".type" key that uci show never emits, so
// it found zero VLANs on every router. The LAN sits on VLAN 10, not the
// hardcoded VID 1 the old code protected. Secrets are stripped; the VLAN
// shape is verbatim.
const uciShowNetworkWithVlans = `network.loopback=interface
network.loopback.device='lo'
network.@device[0]=device
network.@device[0].name='br-lan'
network.@device[0].type='bridge'
network.@device[0].ports='lan2'
network.lan=interface
network.lan.device='br-lan.10'
network.lan.proto='static'
network.wan=interface
network.wan.device='/dev/cdc-wdm0'
network.wan.proto='qmi'
network.@bridge-vlan[0]=bridge-vlan
network.@bridge-vlan[0].device='br-lan'
network.@bridge-vlan[0].vlan='10'
network.@bridge-vlan[0].ports='lan2:u*'
network.@bridge-vlan[1]=bridge-vlan
network.@bridge-vlan[1].device='br-lan'
network.@bridge-vlan[1].vlan='11'
network.@bridge-vlan[1].ports='lan2:t'
network.@bridge-vlan[2]=bridge-vlan
network.@bridge-vlan[2].device='br-lan'
network.@bridge-vlan[2].vlan='16'
network.@bridge-vlan[2].ports='lan2:t'
network.isp=interface
network.isp.proto='pppoe'
network.isp.device='lan1'
network.guests=interface
network.guests.proto='static'
network.guests.device='br-lan.16'
`

func sectionsForTest() map[string]uciSection {
	return parseUCIShow(uciShowNetworkWithVlans, "network")
}

func TestParseUCIShowSectionTypesAndLists(t *testing.T) {
	sections := parseUCIShow(uciShowNetworkWithVlans, "network")

	lan, ok := sections["lan"]
	if !ok || lan.Type != "interface" {
		t.Fatalf("lan = %+v, want an interface section", lan)
	}
	if got := lan.Option("device"); got != "br-lan.10" {
		t.Fatalf("lan.device = %q, want br-lan.10", got)
	}
	if sections["@bridge-vlan[0]"].Type != "bridge-vlan" {
		t.Fatalf("bridge-vlan type not read from the bare section line: %+v", sections["@bridge-vlan[0]"])
	}
	if got := sections["@device[0]"].Option("type"); got != "bridge" {
		t.Fatalf("@device[0].type = %q, want bridge", got)
	}
}

// UCI list options repeat the same key; every value has to be kept.
func TestParseUCIShowKeepsAllListValues(t *testing.T) {
	show := `network.@bridge-vlan[0]=bridge-vlan
network.@bridge-vlan[0].ports='lan1:t'
network.@bridge-vlan[0].ports='lan2:u*'
network.@bridge-vlan[0].ports='lan3:t'
`
	ports := parseUCIShow(show, "network")["@bridge-vlan[0]"].Options["ports"]
	if len(ports) != 3 {
		t.Fatalf("ports = %v, want 3 values", ports)
	}
}

// uci show renders a whole list on ONE line as space-separated quoted
// tokens ('a' 'b' 'c'); each token is its own element.
func TestParseUCIShowSplitsSingleLineLists(t *testing.T) {
	show := `banip.global=banip
banip.global.ban_feed='cinsscore' 'debl' 'turris' 'doh'
banip.global.ban_feedin='cinsscore' 'debl'
banip.global.ban_descr=a plain scalar
`
	opts := parseUCIShow(show, "banip")["global"].Options
	want := map[string][]string{
		"ban_feed":   {"cinsscore", "debl", "turris", "doh"},
		"ban_feedin": {"cinsscore", "debl"},
		"ban_descr":  {"a plain scalar"},
	}
	for k, w := range want {
		if got := opts[k]; len(got) != len(w) || strings.Join(got, ",") != strings.Join(w, ",") {
			t.Fatalf("%s = %v, want %v", k, got, w)
		}
	}
}

func TestResolveBridgePrefersTheLANsBridge(t *testing.T) {
	cases := []struct {
		name string
		show string
		want string
	}{
		{
			name: "lan on a vlan sub-interface",
			show: "network.@device[0]=device\nnetwork.@device[0].name='br-lan'\nnetwork.@device[0].type='bridge'\nnetwork.lan=interface\nnetwork.lan.device='br-lan.10'\n",
			want: "br-lan",
		},
		{
			name: "renamed bridge",
			show: "network.@device[0]=device\nnetwork.@device[0].name='br0'\nnetwork.@device[0].type='bridge'\nnetwork.lan=interface\nnetwork.lan.device='br0'\n",
			want: "br0",
		},
		{
			name: "no lan section, falls back to the bridge-vlan device",
			show: "network.@bridge-vlan[0]=bridge-vlan\nnetwork.@bridge-vlan[0].device='br-guest'\nnetwork.@bridge-vlan[0].vlan='3'\n",
			want: "br-guest",
		},
		{
			name: "only a declared bridge device",
			show: "network.@device[0]=device\nnetwork.@device[0].name='br-home'\nnetwork.@device[0].type='bridge'\n",
			want: "br-home",
		},
		{
			name: "nothing bridge-shaped",
			show: "network.lan=interface\nnetwork.lan.device='eth0'\n",
			want: "",
		},
	}
	for _, c := range cases {
		if got := resolveBridge(parseUCIShow(c.show, "network")); got != c.want {
			t.Fatalf("%s: resolveBridge = %q, want %q", c.name, got, c.want)
		}
	}
}
