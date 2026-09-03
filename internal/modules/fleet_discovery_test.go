package modules

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func validTestBeacon(id, name, version, address string, port int) []byte {
	b := fleetBeacon{
		V:       fleetDiscoveryVersion,
		Type:    "netgrip-fleet-beacon",
		ID:      id,
		Name:    name,
		Version: version,
		Address: address,
		Port:    port,
		TS:      time.Now().Unix(),
	}
	data, _ := json.Marshal(b)
	return data
}

func resetFleetDiscovered(t *testing.T) {
	t.Helper()
	fleetDiscovered = make(map[string]*DiscoveredFleetPeer)
	t.Cleanup(func() { fleetDiscovered = make(map[string]*DiscoveredFleetPeer) })
}

func TestFleetBeaconBuild(t *testing.T) {
	fleetBeaconID = "rt3"
	fleetBeaconName = "rt3"
	fleetBeaconVersion = "0.26.1"
	fleetBeaconPort = 9090
	defer func() {
		fleetBeaconID = ""
		fleetBeaconName = ""
		fleetBeaconVersion = "dev"
		fleetBeaconPort = 8080
	}()

	data := buildBeaconForPeer(nil)
	var b fleetBeacon
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("unmarshal beacon: %v", err)
	}
	if b.V != fleetDiscoveryVersion || b.Type != "netgrip-fleet-beacon" {
		t.Fatalf("unexpected beacon header: %+v", b)
	}
	if b.ID != "rt3" || b.Name != "rt3" || b.Version != "0.26.1" || b.Port != 9090 || b.TS == 0 {
		t.Fatalf("unexpected beacon body: %+v", b)
	}
	if !validFleetBeaconShape(b) {
		t.Fatalf("generated beacon does not pass shape validation")
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

func TestRecordFleetBeaconUnsignedForeignAccepted(t *testing.T) {
	resetFleetDiscovered(t)
	fleetBeaconID = "local-router"
	defer func() { fleetBeaconID = "" }()

	// Beacons are public announcements (no shared secret, #220): a beacon
	// from a different fresh install must be discovered as-is.
	recordFleetBeacon(net.ParseIP("192.168.1.2"), validTestBeacon("rt2", "rt2", "0.60.0", "", 8080))

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

func TestRecordFleetBeaconBadShape(t *testing.T) {
	resetFleetDiscovered(t)

	b, _ := json.Marshal(fleetBeacon{
		V:       1,
		Type:    "netgrip-fleet-beacon",
		ID:      "RT 2!", // invalid id charset
		Name:    "rt2",
		Version: "0.26.0",
		Port:    8080,
		TS:      time.Now().Unix(),
	})
	recordFleetBeacon(net.ParseIP("192.168.1.2"), b)

	peers, _ := ListDiscoveredFleetPeers()
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers (bad id), got %d", len(peers))
	}
}

func TestRecordFleetBeaconStale(t *testing.T) {
	resetFleetDiscovered(t)

	b := validTestBeacon("rt2", "rt2", "0.26.0", "", 8080)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["ts"] = time.Now().Unix() - fleetDiscoveryMaxAgeSec - 10
	b, _ = json.Marshal(raw)
	recordFleetBeacon(net.ParseIP("192.168.1.2"), b)

	peers, _ := ListDiscoveredFleetPeers()
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers (stale beacon), got %d", len(peers))
	}
}

func TestRecordFleetBeaconFromFuture(t *testing.T) {
	resetFleetDiscovered(t)

	b := validTestBeacon("rt7", "rt7", "0.26.0", "", 8080)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["ts"] = time.Now().Unix() + fleetDiscoveryMaxAgeSec + 10
	b, _ = json.Marshal(raw)
	recordFleetBeacon(net.ParseIP("192.168.1.7"), b)

	peers, _ := ListDiscoveredFleetPeers()
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers (future beacon), got %d", len(peers))
	}
}

func TestListDiscoveredSkipsAdopted(t *testing.T) {
	resetFleetDiscovered(t)

	recordFleetBeacon(net.ParseIP("192.168.1.3"), validTestBeacon("rt4", "rt4", "0.26.0", "", 8080))

	tmp := t.TempDir()
	fleetConfigPath = tmp + "/fleet.json"
	defer func() { fleetConfigPath = "/etc/netgrip/fleet.json" }()
	data, _ := json.Marshal(FleetConfig{Nodes: []FleetNode{{ID: "rt4", Name: "rt4", Address: "192.168.1.3:8080"}}})
	if err := SaveFleetConfigFromBytes(data); err != nil {
		t.Fatalf("save fleet config: %v", err)
	}

	peers, err := ListDiscoveredFleetPeers()
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers (rt4 adopted), got %d", len(peers))
	}
}

func TestFleetDiscoveryCleanup(t *testing.T) {
	resetFleetDiscovered(t)

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

func TestAdoptFleetPeerRequiresDiscovery(t *testing.T) {
	resetFleetDiscovered(t)

	if err := AdoptFleetPeer("rt6", "rt6", "192.168.1.6:8080", "secret"); err == nil || err.Error() != "peer not discovered" {
		t.Fatalf("expected peer not discovered error, got %v", err)
	}
}
