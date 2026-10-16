package modules

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestAssembleL2ClientsExcludesSelf(t *testing.T) {
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", port: "lan1", vlan: 1},
		{mac: "02:00:00:00:00:22", port: "lan1", vlan: 1, flags: unix.NTF_SELF},
	}
	clients := assembleL2Clients(entries)
	if len(clients) != 1 {
		t.Fatalf("clients = %v, want only the non-self entry", clients)
	}
	if clients[0].MAC != "aa:bb:cc:00:00:01" {
		t.Fatalf("kept client = %+v, want the non-self MAC", clients[0])
	}
}

func TestAssembleL2ClientsGroupsVlansByMACPort(t *testing.T) {
	// The same MAC on the same port across several VLANs must collapse to
	// one row with all VLANs, not one row per VLAN (#520).
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", port: "lan1", vlan: 10},
		{mac: "aa:bb:cc:00:00:01", port: "lan1", vlan: 20},
		{mac: "aa:bb:cc:00:00:01", port: "lan1", vlan: 10}, // duplicate
		{mac: "aa:bb:cc:00:00:02", port: "lan1", vlan: 10},
	}
	clients := assembleL2Clients(entries)
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2 rows: %v", len(clients), clients)
	}
	var target *L2Client
	for i := range clients {
		if clients[i].MAC == "aa:bb:cc:00:00:01" {
			target = &clients[i]
		}
	}
	if target == nil {
		t.Fatalf("the VLAN-grouped MAC is missing: %v", clients)
	}
	if len(target.Vlans) != 2 || target.Vlans[0] != 10 || target.Vlans[1] != 20 {
		t.Fatalf("vlans = %v, want [10 20]", target.Vlans)
	}
}

func TestAssembleL2ClientsKeepsMACOnTwoPorts(t *testing.T) {
	// A MAC learned on two ports stays as two rows: loop detection depends
	// on it (same rationale as parseBrctlShowmacs).
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", port: "lan1", vlan: 0},
		{mac: "aa:bb:cc:00:00:01", port: "lan2", vlan: 0},
	}
	clients := assembleL2Clients(entries)
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2 (one per port): %v", len(clients), clients)
	}
}

func TestAssembleL2ClientsStaticAndAge(t *testing.T) {
	// A configured entry is reported as static: the kernel marks it NUD_NOARP
	// or, for the entries hostapd and the WAN port install, NUD_PERMANENT (a
	// real router dump uses the latter). Learned entries are REACHABLE/STALE.
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", port: "lan1", vlan: 0, state: unix.NUD_NOARP, used: 300},
		{mac: "aa:bb:cc:00:00:02", port: "lan1", vlan: 0, state: unix.NUD_PERMANENT, used: 0},
		{mac: "aa:bb:cc:00:00:03", port: "lan1", vlan: 0, state: unix.NUD_REACHABLE, used: 16609},
	}
	clients := assembleL2Clients(entries)
	if len(clients) != 3 {
		t.Fatalf("clients = %d, want 3: %v", len(clients), clients)
	}
	byMAC := map[string]L2Client{}
	for _, c := range clients {
		byMAC[c.MAC] = c
	}
	if c := byMAC["aa:bb:cc:00:00:01"]; !c.Static || c.Age != 3 {
		t.Errorf("NOARP entry = %+v, want Static=true Age=3", c)
	}
	if c := byMAC["aa:bb:cc:00:00:02"]; !c.Static || c.Age != 0 {
		t.Errorf("PERMANENT entry = %+v, want Static=true Age=0", c)
	}
	if c := byMAC["aa:bb:cc:00:00:03"]; c.Static || c.Age != 166 {
		t.Errorf("learned entry = %+v, want Static=false Age=166", c)
	}
}

func TestAssembleL2ClientsStableOrder(t *testing.T) {
	entries := []l2Neigh{
		{mac: "cc:cc:cc:00:00:03", port: "lan1", vlan: 0},
		{mac: "aa:aa:aa:00:00:01", port: "lan2", vlan: 0},
		{mac: "bb:bb:bb:00:00:02", port: "lan1", vlan: 0},
	}
	clients := assembleL2Clients(entries)
	want := []string{
		"bb:bb:bb:00:00:02", // lan1
		"cc:cc:cc:00:00:03", // lan1
		"aa:aa:aa:00:00:01", // lan2
	}
	for i, w := range want {
		if clients[i].MAC != w {
			t.Fatalf("order[%d] = %s, want %s (full: %v)", i, clients[i].MAC, w, clients)
		}
	}
}

func TestAssembleL2ClientsSkipsEntryWithoutPort(t *testing.T) {
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", vlan: 0},
	}
	clients := assembleL2Clients(entries)
	if len(clients) != 0 {
		t.Fatalf("clients = %v, want none for an entry with no port", clients)
	}
}

func TestNormalizeVlansDropsUnfilteredMarker(t *testing.T) {
	cases := []struct {
		name  string
		in    []int
		want  []int
		empty bool
	}{
		{name: "solo 0 se conserva", in: []int{0}, want: []int{0}},
		{name: "sin vlan no cambia", in: []int{10, 20}, want: []int{10, 20}},
		{name: "0 junto a vlans reales se cae", in: []int{0, 10, 20}, want: []int{10, 20}},
		{name: "0 y 1 (caso real del router)", in: []int{0, 1}, want: []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeVlans(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("normalizeVlans(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("normalizeVlans(%v) = %v, want %v", tc.in, got, tc.want)
				}
			}
		})
	}
}

// TestAssembleL2ClientsLiveRouterFixture usa una muestra real capturada con
// netlink en el router de casa (rt3, br-lan): entradas aprendidas en el
// puerto de bajada, un cliente wifi y una entrada estatica con marcador sin
// filtrar. Fija el aspecto que vera el usuario y protege el agrupado.
func TestAssembleL2ClientsLiveRouterFixture(t *testing.T) {
	entries := []l2Neigh{
		// lan1: la bajada al resto de la casa, MACs aprendidas con edad.
		{mac: "34:5f:45:5f:ab:7d", port: "lan1", vlan: 0, state: unix.NUD_REACHABLE, used: 16609},
		{mac: "5c:cf:7f:4e:f9:d5", port: "lan1", vlan: 0, state: unix.NUD_REACHABLE, used: 16615},
		{mac: "02:78:f4:02:8a:94", port: "lan1", vlan: 0, state: unix.NUD_REACHABLE, used: 127},
		// phy1-ap0: un cliente asociado a la radio de 2,4 GHz.
		{mac: "d8:d6:68:32:7c:46", port: "phy1-ap0", vlan: 0, state: unix.NUD_REACHABLE, used: 275},
		// Entrada estatica del propio puerto WAN, vista en vlan 0 y vlan 1.
		{mac: "28:d1:27:f5:99:f2", port: "wan", vlan: 0, state: unix.NUD_PERMANENT, used: 53713774},
		{mac: "28:d1:27:f5:99:f2", port: "wan", vlan: 1, state: unix.NUD_PERMANENT, used: 53713774},
	}
	clients := assembleL2Clients(entries)
	if len(clients) != 5 {
		t.Fatalf("clients = %d, want 5 rows: %v", len(clients), clients)
	}
	byKey := map[string]L2Client{}
	for _, c := range clients {
		byKey[c.Port+"|"+c.MAC] = c
	}
	if c := byKey["lan1|34:5f:45:5f:ab:7d"]; c.Age != 166 || c.Static || len(c.Vlans) != 1 || c.Vlans[0] != 0 {
		t.Errorf("cliente cableado = %+v, want Age=166 sin static y vlans [0]", c)
	}
	if c, ok := byKey["phy1-ap0|d8:d6:68:32:7c:46"]; !ok || c.Age != 2 {
		t.Errorf("cliente wifi = %+v (presente=%v), want Age=2", c, ok)
	}
	// Esta MAC real lleva el bit locally administered: es un movil con MAC
	// privada, y la vista debe marcarlo en vez de inventar un fabricante.
	if c := byKey["lan1|02:78:f4:02:8a:94"]; !c.Random || c.Vendor != "" {
		t.Errorf("MAC privada = %+v, want Random=true y sin fabricante", c)
	}
	if c := byKey["wan|28:d1:27:f5:99:f2"]; !c.Static || len(c.Vlans) != 1 || c.Vlans[0] != 1 {
		t.Errorf("entrada estatica del WAN = %+v, want static y vlans [1] (sin el marcador 0)", c)
	}
}
