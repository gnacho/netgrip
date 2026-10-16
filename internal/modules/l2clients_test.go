package modules

import (
	"testing"

	"golang.org/x/sys/unix"
)

// portIndex maps the fixture link indexes used in the tests to names.
func fixedPorts(idx int) string {
	switch idx {
	case 1:
		return "lan1"
	case 2:
		return "lan2"
	default:
		return ""
	}
}

func TestAssembleL2ClientsExcludesSelf(t *testing.T) {
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", vlan: 1, linkIdx: 1},
		{mac: "02:00:00:00:00:22", vlan: 1, linkIdx: 1, flags: unix.NTF_SELF},
	}
	clients := assembleL2Clients(entries, fixedPorts)
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
		{mac: "aa:bb:cc:00:00:01", vlan: 10, linkIdx: 1},
		{mac: "aa:bb:cc:00:00:01", vlan: 20, linkIdx: 1},
		{mac: "aa:bb:cc:00:00:01", vlan: 10, linkIdx: 1}, // duplicate
		{mac: "aa:bb:cc:00:00:02", vlan: 10, linkIdx: 1},
	}
	clients := assembleL2Clients(entries, fixedPorts)
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
		{mac: "aa:bb:cc:00:00:01", vlan: 0, linkIdx: 1},
		{mac: "aa:bb:cc:00:00:01", vlan: 0, linkIdx: 2},
	}
	clients := assembleL2Clients(entries, fixedPorts)
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2 (one per port): %v", len(clients), clients)
	}
}

func TestAssembleL2ClientsStaticAndAge(t *testing.T) {
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", vlan: 0, linkIdx: 1, state: unix.NUD_NOARP, used: 300},
		{mac: "aa:bb:cc:00:00:02", vlan: 0, linkIdx: 1, used: 0},
	}
	clients := assembleL2Clients(entries, fixedPorts)
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2: %v", len(clients), clients)
	}
	byMAC := map[string]L2Client{}
	for _, c := range clients {
		byMAC[c.MAC] = c
	}
	if c := byMAC["aa:bb:cc:00:00:01"]; !c.Static || c.Age != 3 {
		t.Errorf("static entry = %+v, want Static=true Age=3", c)
	}
	if c := byMAC["aa:bb:cc:00:00:02"]; c.Static || c.Age != 0 {
		t.Errorf("unknown-age entry = %+v, want Static=false Age=0", c)
	}
}

func TestAssembleL2ClientsStableOrder(t *testing.T) {
	entries := []l2Neigh{
		{mac: "cc:cc:cc:00:00:03", vlan: 0, linkIdx: 1},
		{mac: "aa:aa:aa:00:00:01", vlan: 0, linkIdx: 2},
		{mac: "bb:bb:bb:00:00:02", vlan: 0, linkIdx: 1},
	}
	clients := assembleL2Clients(entries, fixedPorts)
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

func TestAssembleL2ClientsSkipsUnresolvedPort(t *testing.T) {
	entries := []l2Neigh{
		{mac: "aa:bb:cc:00:00:01", vlan: 0, linkIdx: 9},
	}
	clients := assembleL2Clients(entries, fixedPorts)
	if len(clients) != 0 {
		t.Fatalf("clients = %v, want none for an unresolvable port", clients)
	}
}
