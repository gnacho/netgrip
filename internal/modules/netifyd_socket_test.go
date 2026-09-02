package modules

import (
	"testing"
	"time"
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

func TestNetifydTableTimeline(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("d1", "YouTube")
	tbl.addStats("d1", 100, 200, 300, 10)
	tbl.setFlowApp("d2", "QUIC")
	tbl.addStats("d2", 10, 20, 30, 1)

	timeline := tbl.Timeline()
	if timeline.Totals.Total != 330 {
		t.Fatalf("expected totals total 330, got %d", timeline.Totals.Total)
	}
	if len(timeline.Top) != 2 {
		t.Fatalf("expected 2 top apps, got %d", len(timeline.Top))
	}
	if timeline.Top[0].Name != "YouTube" {
		t.Fatalf("expected top app YouTube, got %s", timeline.Top[0].Name)
	}
	if len(timeline.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(timeline.Buckets))
	}
	bucket := timeline.Buckets[0]
	if len(bucket.Apps) != 2 {
		t.Fatalf("expected 2 apps in bucket, got %d", len(bucket.Apps))
	}
	if bucket.Apps["YouTube"].Total != 300 {
		t.Fatalf("expected YouTube bucket total 300, got %d", bucket.Apps["YouTube"].Total)
	}
}

func TestNetifydTableTimelineEviction(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.maxBuckets = 2
	tbl.setFlowApp("d1", "YouTube")

	base := time.Now().Unix() / int64(timelineBucketDuration.Seconds()) * int64(timelineBucketDuration.Seconds())
	// Inject buckets directly to avoid time dependency.
	tbl.buckets[base] = map[string]*NetifydBucket{"YouTube": {Total: 1}}
	tbl.buckets[base+300] = map[string]*NetifydBucket{"YouTube": {Total: 2}}
	tbl.buckets[base+600] = map[string]*NetifydBucket{"YouTube": {Total: 3}}

	timeline := tbl.Timeline()
	if len(timeline.Buckets) != 2 {
		t.Fatalf("expected 2 buckets after eviction, got %d", len(timeline.Buckets))
	}
	if timeline.Totals.Total != 5 {
		t.Fatalf("expected total 5 after eviction, got %d", timeline.Totals.Total)
	}
}
