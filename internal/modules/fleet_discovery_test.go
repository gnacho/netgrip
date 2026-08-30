package modules

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestFleetBeaconBuild(t *testing.T) {
	fleetBeaconID = "rt3"
	fleetBeaconName = "rt3"
	fleetBeaconVersion = "0.26.1"
	fleetBeaconPort = 9090
	defer func() { fleetBeaconID = ""; fleetBeaconName = ""; fleetBeaconVersion = "dev"; fleetBeaconPort = 8080 }()

	data := buildBeaconForPeer(nil)
	var b fleetBeacon
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("unmarshal beacon: %v", err)
	}
	if b.V != fleetDiscoveryVersion || b.Type != "netgrip-fleet-beacon" {
		t.Fatalf("unexpected beacon header: %+v", b)
	}
	if b.ID != "rt3" || b.Name != "rt3" || b.Version != "0.26.1" || b.Port != 9090 {
		t.Fatalf("unexpected beacon body: %+v", b)
	}
}

func TestIsFleetProbe(t *testing.T) {
	if !isFleetProbe([]byte(`{"v":1,"type":"netgrip-fleet-probe"}`)) {
		t.Fatal("expected valid probe")
	}
	if isFleetProbe([]byte(`{"v":1,"type":"netgrip-fleet-beacon"}`)) {
		t.Fatal("beacon is not a probe")
	}
	if isFleetProbe([]byte(`{"v":2,"type":"netgrip-fleet-probe"}`)) {
		t.Fatal("v2 should not match")
	}
}

func TestRecordFleetBeacon(t *testing.T) {
	fleetDiscovered = make(map[string]*DiscoveredFleetPeer)
	defer func() { fleetDiscovered = make(map[string]*DiscoveredFleetPeer) }()

	b, _ := json.Marshal(fleetBeacon{
		V:       1,
		Type:    "netgrip-fleet-beacon",
		ID:      "rt2",
		Name:    "rt2",
		Version: "0.26.0",
		Port:    8080,
	})
	recordFleetBeacon(net.ParseIP("192.168.1.2"), b)

	peers, err := ListDiscoveredFleetPeers()
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	p := peers[0]
	if p.ID != "rt2" || p.Address != "192.168.1.2" || p.Port != 8080 {
		t.Fatalf("unexpected peer: %+v", p)
	}
}

func TestListDiscoveredSkipsAdopted(t *testing.T) {
	fleetDiscovered = make(map[string]*DiscoveredFleetPeer)
	defer func() { fleetDiscovered = make(map[string]*DiscoveredFleetPeer) }()

	recordFleetBeacon(net.ParseIP("192.168.1.3"), mustMarshal(fleetBeacon{ID: "rt4", Name: "rt4", Version: "0.26.0", Port: 8080}))

	tmp := t.TempDir()
	fleetConfigPath = tmp + "/fleet.json"
	defer func() { fleetConfigPath = "/etc/netgrip/fleet.json" }()
	_ = SaveFleetConfig(FleetConfig{Nodes: []FleetNode{{ID: "rt4", Name: "rt4", Address: "192.168.1.3:8080"}}})

	peers, err := ListDiscoveredFleetPeers()
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers (rt4 adopted), got %d", len(peers))
	}
}

func TestFleetDiscoveryCleanup(t *testing.T) {
	fleetDiscovered = make(map[string]*DiscoveredFleetPeer)
	defer func() { fleetDiscovered = make(map[string]*DiscoveredFleetPeer) }()

	fleetDiscovered["old"] = &DiscoveredFleetPeer{ID: "old", SeenAt: time.Now().Add(-10 * time.Minute)}
	fleetDiscovered["fresh"] = &DiscoveredFleetPeer{ID: "fresh", SeenAt: time.Now()}

	// simular un ciclo de cleanup
	fleetDiscMu.Lock()
	for id, p := range fleetDiscovered {
		if time.Since(p.SeenAt) > fleetDiscoveryTTL {
			delete(fleetDiscovered, id)
		}
	}
	fleetDiscMu.Unlock()

	if _, ok := fleetDiscovered["old"]; ok {
		t.Fatal("old peer should be cleaned")
	}
	if _, ok := fleetDiscovered["fresh"]; !ok {
		t.Fatal("fresh peer should remain")
	}
}

func TestRecordFleetBeaconFromUDP(t *testing.T) {
	fleetDiscovered = make(map[string]*DiscoveredFleetPeer)
	defer func() { fleetDiscovered = make(map[string]*DiscoveredFleetPeer) }()

	// Simulamos un beacon UDP recibido de otro router.
	b, _ := json.Marshal(fleetBeacon{
		V:       1,
		Type:    "netgrip-fleet-beacon",
		ID:      "rt5",
		Name:    "rt5",
		Version: "0.26.0",
		Port:    8080,
	})
	recordFleetBeacon(net.ParseIP("192.168.1.5"), b)

	peers, err := ListDiscoveredFleetPeers()
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if len(peers) != 1 || peers[0].ID != "rt5" || peers[0].Address != "192.168.1.5" {
		t.Fatalf("unexpected peer: %+v", peers)
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
