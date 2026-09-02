package modules

import (
	"testing"
)

func TestNetifydTableAggregation(t *testing.T) {
	tbl := newNetifydTable(256, 4096)

	// First flow identifies the application.
	tbl.setFlowApp("abc123", "YouTube")
	// Flow stats add bytes.
	tbl.addStats("abc123", 100, 200, 300, 10)
	tbl.addStats("abc123", 50, 70, 120, 5)

	// Another app.
	tbl.setFlowApp("def456", "QUIC")
	tbl.addStats("def456", 10, 20, 30, 1)

	apps := tbl.Apps()
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Sorted by bytes descending: YouTube first.
	if apps[0].Name != "YouTube" {
		t.Fatalf("expected first app YouTube, got %s", apps[0].Name)
	}
	if apps[0].Bytes != 420 {
		t.Fatalf("expected YouTube bytes 420, got %d", apps[0].Bytes)
	}
	if apps[0].Flows != 2 {
		t.Fatalf("expected YouTube flows 2, got %d", apps[0].Flows)
	}
	if apps[0].Packets != 15 {
		t.Fatalf("expected YouTube packets 15, got %d", apps[0].Packets)
	}

	if apps[1].Name != "QUIC" {
		t.Fatalf("expected second app QUIC, got %s", apps[1].Name)
	}
}

func TestNetifydTableUnknownDigest(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	// Stats without a previous flow mapping end up under Unknown.
	tbl.addStats("no-flow", 1, 2, 3, 1)
	apps := tbl.Apps()
	if len(apps) != 1 || apps[0].Name != "Unknown" {
		t.Fatalf("expected Unknown app, got %+v", apps)
	}
}

func TestNetifydTableEviction(t *testing.T) {
	tbl := newNetifydTable(2, 2)
	// Fill the apps table past the limit.
	tbl.setFlowApp("a", "AppA")
	tbl.addStats("a", 100, 0, 100, 1)
	tbl.setFlowApp("b", "AppB")
	tbl.addStats("b", 50, 0, 50, 1)
	tbl.setFlowApp("c", "AppC")
	tbl.addStats("c", 10, 0, 10, 1)

	apps := tbl.Apps()
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps after eviction, got %d", len(apps))
	}
	for _, a := range apps {
		if a.Name == "AppC" {
			t.Fatal("smallest app AppC should have been evicted")
		}
	}
}

func TestNetifydClientHandleMessage(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	c := newNetifydSocketClient("", tbl)

	c.handleMessage(`{"type":"flow","flow":{"digest":"d1","detected_application_name":"YouTube","detected_protocol_name":"TLS"}}`)
	c.handleMessage(`{"type":"flow_stats","flow":{"digest":"d1","local_bytes":100,"other_bytes":200,"total_bytes":300,"total_packets":10}}`)
	c.handleMessage(`{"type":"flow_purge","flow":{"digest":"d1","local_bytes":50,"other_bytes":70,"total_bytes":120,"total_packets":5}}`)

	apps := tbl.Apps()
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "YouTube" {
		t.Fatalf("expected YouTube, got %s", apps[0].Name)
	}
	if apps[0].Bytes != 420 {
		t.Fatalf("expected 420 bytes, got %d", apps[0].Bytes)
	}
	if apps[0].Packets != 15 {
		t.Fatalf("expected 15 packets, got %d", apps[0].Packets)
	}
	if apps[0].Flows != 2 {
		t.Fatalf("expected 2 flows, got %d", apps[0].Flows)
	}
}
