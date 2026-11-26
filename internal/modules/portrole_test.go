package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

// Configuración DSA de un router con LAN de 5 puertos y WAN con VLAN de ISP
// (el caso del GL-MT6000: eth1.20 para PPPoE).
const portRoleRouterFixture = `network.@device[0]=device
network.@device[0].name='br-lan'
network.@device[0].type='bridge'
network.@device[0].ports='lan1' 'lan2' 'lan3' 'lan4' 'lan5'
network.lan=interface
network.lan.device='br-lan'
network.lan.proto='static'
network.wan=interface
network.wan.device='eth1.20'
network.wan.proto='pppoe'
`

// Configuración de un AP: todos los puertos (incluido el llamado "wan") en el
// bridge y la LAN como cliente DHCP.
const portRoleAPFixture = `network.@device[0]=device
network.@device[0].name='br-lan'
network.@device[0].type='bridge'
network.@device[0].ports='lan2' 'lan3' 'lan4' 'wan'
network.lan=interface
network.lan.device='br-lan'
network.lan.proto='dhcp'
`

func roleInput(mode, bridge, wanIface, wanDevice string, candidates ...string) portRoleInput {
	return portRoleInput{
		Bridge: bridge, WanIface: wanIface, WanDevice: wanDevice,
		Mode: mode, Candidates: candidates,
	}
}

func opsString(ops []executor.Op) string {
	var b strings.Builder
	for _, op := range ops {
		b.WriteString(op.Kind)
		for _, a := range op.Args {
			b.WriteString(" " + a)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestBuildPortRoleOpsMoveLanPortToWANKeepsTheVlanSuffix(t *testing.T) {
	sections := parseUCIShow(portRoleRouterFixture, "network")
	in := roleInput("router", "br-lan", "wan", "eth1.20", "lan1", "lan2", "lan3", "lan4", "lan5")

	ops, err := buildPortRoleOps(sections, in, "lan3", "wan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := opsString(ops)
	for _, want := range []string{
		"uci_del_list network.@device[0].ports lan3\n",
		"uci_set network.wan.device lan3.20\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ops missing %q in:\n%s", want, got)
		}
	}
	// eth1 no es un puerto del chasis (0.9 de un Flint2): no se mete en el
	// bridge a lo loco.
	if strings.Contains(got, "uci_add_list") {
		t.Fatalf("dedicated WAN netdev must not be bridged:\n%s", got)
	}
	for _, op := range ops {
		if err := executor.Validate(op); err != nil {
			t.Fatalf("generated op fails the executor allowlist: %v", err)
		}
	}
}

// Router con la WAN en el puerto llamado "wan", que es un puerto del switch:
// al mover la WAN a lan4, "wan" vuelve a LAN.
const portRoleSwapFixture = `network.@device[0]=device
network.@device[0].name='br-lan'
network.@device[0].type='bridge'
network.@device[0].ports='lan1' 'lan2' 'lan3'
network.lan=interface
network.lan.device='br-lan'
network.lan.proto='static'
network.wan=interface
network.wan.device='wan'
network.wan.proto='dhcp'
`

func TestBuildPortRoleOpsSwapPutsTheOldSwitchWanPortBackToLan(t *testing.T) {
	sections := parseUCIShow(portRoleSwapFixture, "network")
	in := roleInput("router", "br-lan", "wan", "wan", "lan1", "lan2", "lan3", "lan4", "wan")

	ops, err := buildPortRoleOps(sections, in, "lan4", "wan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := opsString(ops)
	for _, want := range []string{
		"uci_set network.wan.device lan4\n",
		"uci_add_list network.@device[0].ports wan\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ops missing %q in:\n%s", want, got)
		}
	}
}

func TestBuildPortRoleOpsWriteIfnameOnLegacyConfig(t *testing.T) {
	legacy := strings.Replace(portRoleRouterFixture,
		"network.wan.device='eth1.20'", "network.wan.ifname='eth1.20'", 1)
	sections := parseUCIShow(legacy, "network")
	in := roleInput("router", "br-lan", "wan", "eth1.20", "lan1", "lan2", "lan3", "lan4", "lan5")

	ops, err := buildPortRoleOps(sections, in, "lan2", "wan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := opsString(ops)
	if !strings.Contains(got, "uci_set network.wan.ifname lan2.20\n") {
		t.Fatalf("legacy ifname config must be written as ifname:\n%s", got)
	}
	if strings.Contains(got, "network.wan.device") {
		t.Fatalf("must not add device to a legacy ifname config:\n%s", got)
	}
}

func TestBuildPortRoleOpsMoveWanPortToLanIsRefused(t *testing.T) {
	sections := parseUCIShow(portRoleRouterFixture, "network")
	in := roleInput("router", "br-lan", "wan", "eth1.20", "lan1", "lan2", "lan3", "lan4", "lan5")

	_, err := buildPortRoleOps(sections, in, "lan3", "lan3")
	if err == nil || !strings.Contains(err.Error(), "role must be") {
		t.Fatalf("unknown role must be refused, got %v", err)
	}

	in = roleInput("router", "br-lan", "wan", "lan3", "lan1", "lan2", "lan3", "lan4", "lan5")
	_, err = buildPortRoleOps(sections, in, "lan3", "lan")
	if err == nil || !strings.Contains(err.Error(), "left without an uplink") {
		t.Fatalf("removing the only WAN must be refused with the Modo pointer, got %v", err)
	}
}

func TestBuildPortRoleOpsAlreadyThere(t *testing.T) {
	sections := parseUCIShow(portRoleRouterFixture, "network")
	in := roleInput("router", "br-lan", "wan", "eth1.20", "lan1", "lan2", "lan3", "lan4", "lan5")
	if _, err := buildPortRoleOps(sections, in, "lan2", "lan"); err == nil || !strings.Contains(err.Error(), "already a LAN port") {
		t.Fatalf("already-LAN must be refused, got %v", err)
	}
	inOld := roleInput("router", "br-lan", "wan", "lan2", "lan1", "lan2", "lan3", "lan4", "lan5")
	if _, err := buildPortRoleOps(sections, inOld, "lan2", "wan"); err == nil || !strings.Contains(err.Error(), "already the WAN port") {
		t.Fatalf("already-WAN must be refused, got %v", err)
	}
}

func TestBuildPortRoleOpsAPCannotAssignWan(t *testing.T) {
	sections := parseUCIShow(portRoleAPFixture, "network")
	in := roleInput("ap", "br-lan", "", "", "lan2", "lan3", "lan4", "wan")
	if _, err := buildPortRoleOps(sections, in, "lan2", "wan"); err == nil || !strings.Contains(err.Error(), "not a router") {
		t.Fatalf("AP must refuse the WAN role, got %v", err)
	}
}

func TestBuildPortRoleOpsRefusesPortOwnedByAnotherInterface(t *testing.T) {
	fixture := portRoleRouterFixture + "network.secondwan=interface\nnetwork.secondwan.device='lan4'\nnetwork.secondwan.proto='dhcp'\n"
	sections := parseUCIShow(fixture, "network")
	in := roleInput("router", "br-lan", "wan", "eth1.20", "lan1", "lan2", "lan3", "lan4", "lan5")
	_, err := buildPortRoleOps(sections, in, "lan4", "wan")
	if err == nil || !strings.Contains(err.Error(), "already used by interface secondwan") {
		t.Fatalf("secondwan-owned port must be refused, got %v", err)
	}
}

func TestBuildPortRoleOpsRefusesWhenVlanFilteringOwnsThePort(t *testing.T) {
	fixture := portRoleRouterFixture +
		"network.@bridge-vlan[0]=bridge-vlan\nnetwork.@bridge-vlan[0].device='br-lan'\nnetwork.@bridge-vlan[0].vlan='10'\nnetwork.@bridge-vlan[0].ports='lan3:u*' 'lan1:t'\n"
	sections := parseUCIShow(fixture, "network")
	in := roleInput("router", "br-lan", "wan", "eth1.20", "lan1", "lan2", "lan3", "lan4", "lan5")
	_, err := buildPortRoleOps(sections, in, "lan3", "wan")
	if err == nil || !strings.Contains(err.Error(), "VLAN filtering") {
		t.Fatalf("bridge-vlan membership must send the user to the VLANs page, got %v", err)
	}
}

func TestBuildPortRoleOpsLanRoleAddsThePort(t *testing.T) {
	sections := parseUCIShow(portRoleSwapFixture, "network")
	in := roleInput("router", "br-lan", "wan", "wan", "lan1", "lan2", "lan3", "lan4")
	// lan4 existe en el chasis y no está en el bridge: es un puerto sin rol,
	// y pasarlo a LAN es la operación normal.
	ops, err := buildPortRoleOps(sections, in, "lan4", "lan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := opsString(ops); !strings.Contains(got, "uci_add_list network.@device[0].ports lan4\n") {
		t.Fatalf("expected the port added to the bridge, got:\n%s", got)
	}
}

func TestNormalizePortMAC(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr string
	}{
		{in: "", want: ""},
		{in: "AA:BB:CC:DD:EE:F0", want: "aa:bb:cc:dd:ee:f0"},
		{in: " aa:bb:cc:dd:ee:f0\n", want: "aa:bb:cc:dd:ee:f0"},
		{in: "aa-bb-cc-dd-ee-f0", want: "aa:bb:cc:dd:ee:f0"},
		{in: "aa:bb:cc:dd:ee", wantErr: "invalid MAC address"},
		{in: "zz:bb:cc:dd:ee:f0", wantErr: "invalid MAC address"},
		{in: "01:00:5e:00:00:01", wantErr: "unicast"},
		{in: "00:00:00:00:00:00", wantErr: "invalid MAC address"},
	}
	for _, c := range cases {
		got, err := normalizePortMAC(c.in)
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("normalizePortMAC(%q) err = %v, want containing %q", c.in, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizePortMAC(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("normalizePortMAC(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBasePortNameAndVlanSuffix(t *testing.T) {
	cases := []struct{ in, base, suffix string }{
		{"eth1.20", "eth1", ".20"},
		{"lan3", "lan3", ""},
		{"wan.100", "wan", ".100"},
		{"br-lan.10", "br-lan", ".10"},
		{"eth1.abc", "eth1.abc", ""},
	}
	for _, c := range cases {
		if got := basePortName(c.in); got != c.base {
			t.Fatalf("basePortName(%q) = %q, want %q", c.in, got, c.base)
		}
		if got := vlanSuffix(c.in); got != c.suffix {
			t.Fatalf("vlanSuffix(%q) = %q, want %q", c.in, got, c.suffix)
		}
	}
}

func TestNamedDeviceSectionIsAValidUCISection(t *testing.T) {
	sec := namedDeviceSection("LAN-3.2")
	if sec != "port_lan_3_2" {
		t.Fatalf("namedDeviceSection = %q", sec)
	}
	// El nombre generado tiene que pasar el allowlist del ejecutor.
	if err := executor.Validate(executor.Op{Kind: "uci_set", Args: []string{"network." + sec, "device"}}); err != nil {
		t.Fatalf("generated section name is not a valid UCI key: %v", err)
	}
}
