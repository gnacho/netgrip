package modules

import (
	"testing"
	"time"
)

func TestNetifydTableAggregation(t *testing.T) {
	tbl := newNetifydTable(256, 4096)

	// First flow identifies the application.
	tbl.setFlowApp("abc123", "YouTube", "HTTP/S", "")
	// Flow stats add bytes.
	tbl.addStats("abc123", 100, 200, 300, 10)
	tbl.addStats("abc123", 50, 70, 120, 5)

	// Another app.
	tbl.setFlowApp("def456", "QUIC", "QUIC", "")
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
	if apps[0].Flows != 1 {
		t.Fatalf("expected YouTube flows 1, got %d", apps[0].Flows)
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
	// Stats without any flow mapping wait for an application name and land
	// on Unknown when the flow ends.
	tbl.addStats("no-flow", 1, 2, 3, 1)
	tbl.purgeFlow("no-flow", 1, 2, 3, 1)
	apps := tbl.Apps()
	if len(apps) != 1 || apps[0].Name != "Unknown" {
		t.Fatalf("expected Unknown app, got %+v", apps)
	}
	if apps[0].Bytes != 3 {
		t.Fatalf("expected 3 bytes, got %d", apps[0].Bytes)
	}
}

func TestNetifydTableEviction(t *testing.T) {
	tbl := newNetifydTable(2, 2)
	// Fill the apps table past the limit.
	tbl.setFlowApp("a", "AppA", "", "")
	tbl.addStats("a", 100, 0, 100, 1)
	tbl.setFlowApp("b", "AppB", "", "")
	tbl.addStats("b", 50, 0, 50, 1)
	tbl.setFlowApp("c", "AppC", "", "")
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
	if apps[0].Flows != 1 {
		t.Fatalf("expected 1 flow, got %d", apps[0].Flows)
	}
}

func TestNetifydTableTimeline(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("d1", "YouTube", "", "")
	tbl.addStats("d1", 100, 200, 300, 10)
	tbl.setFlowApp("d2", "QUIC", "", "")
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
	tbl.setFlowApp("d1", "YouTube", "", "")

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

// Netifyd repite el total acumulado del flujo en cada mensaje: sumarlos
// inflaba el trafico (#620).
func TestNetifydTableCumulativeCountersCountOnce(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("d1", "YouTube", "", "")
	tbl.addStats("d1", 400, 600, 1000, 10)
	tbl.addStats("d1", 400, 600, 1000, 10)
	tbl.addStats("d1", 400, 600, 1000, 10)

	apps := tbl.Apps()
	if len(apps) != 1 || apps[0].Bytes != 1000 {
		t.Fatalf("expected 1000 bytes, got %+v", apps)
	}
	if apps[0].Packets != 10 {
		t.Fatalf("expected 10 packets, got %d", apps[0].Packets)
	}
}

// nDPI suele dar el protocolo antes que la aplicacion: los bytes vistos en
// ese hueco tienen que acabar en la aplicacion, no quedarse en el protocolo.
func TestNetifydTableMovesPendingBytesToTheApplication(t *testing.T) {
	tbl := newNetifydTable(256, 4096)

	tbl.setFlowApp("d1", "", "HTTP/S", "")
	tbl.addStats("d1", 100, 200, 300, 5)
	if apps := tbl.Apps(); len(apps) != 0 {
		t.Fatalf("sin aplicacion no se contabiliza todavia: %+v", apps)
	}

	tbl.setFlowApp("d1", "youtube", "HTTP/S", "")
	apps := tbl.Apps()
	if len(apps) != 1 || apps[0].Name != "youtube" {
		t.Fatalf("expected youtube, got %+v", apps)
	}
	if apps[0].Bytes != 300 || apps[0].Flows != 1 {
		t.Fatalf("bytes/flows = %d/%d, want 300/1", apps[0].Bytes, apps[0].Flows)
	}

	tbl.addStats("d1", 100, 200, 500, 7)
	apps = tbl.Apps()
	if apps[0].Bytes != 500 {
		t.Fatalf("expected 500 bytes, got %d", apps[0].Bytes)
	}
}

// Un flujo que nunca recibe aplicacion acaba en su protocolo en vez de
// quedarse invisible para siempre.
func TestNetifydTablePendingFallsBackToTheProtocol(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("d1", "", "SSH", "")
	tbl.addStats("d1", 10, 20, 30, 1)

	tbl.mu.Lock()
	tbl.flows["d1"].pendingAt = time.Now().Add(-2 * pendingFlushAfter)
	tbl.mu.Unlock()

	tbl.addStats("d1", 10, 20, 40, 2)
	apps := tbl.Apps()
	if len(apps) != 1 || apps[0].Name != "SSH" {
		t.Fatalf("expected SSH, got %+v", apps)
	}
	if apps[0].Bytes != 40 {
		t.Fatalf("expected 40 bytes, got %d", apps[0].Bytes)
	}
}

func TestAppLabelStripsTheNetifyNamespace(t *testing.T) {
	cases := map[string]string{
		"netify.youtube": "youtube",
		"netify.netflix": "netflix",
		"YouTube":        "YouTube",
		"netify.":        "netify.",
	}
	for in, want := range cases {
		if got := appLabel(in); got != want {
			t.Errorf("appLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNetifydClientStripsTheNamespace(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	c := newNetifydSocketClient("", tbl)
	c.handleMessage(`{"type":"flow","flow":{"digest":"d1","detected_application_name":"netify.youtube","detected_protocol_name":"HTTP/S"}}`)
	c.handleMessage(`{"type":"flow_stats","flow":{"digest":"d1","local_bytes":10,"other_bytes":20,"total_bytes":30,"total_packets":2}}`)

	apps := tbl.Apps()
	if len(apps) != 1 || apps[0].Name != "youtube" {
		t.Fatalf("expected youtube, got %+v", apps)
	}
}

// The category netifyd reports is kept with the application, so the interface
// can fall back to a category icon when it has no brand icon (#687).
func TestNetifydAppKeepsTheCategory(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("cat1", "YouTube", "HTTP/S", "streaming")
	tbl.addStats("cat1", 0, 1024, 1024, 4)

	for _, a := range tbl.Apps() {
		if a.Name == "YouTube" {
			if a.Category != "streaming" {
				t.Fatalf("App category = %q, want streaming", a.Category)
			}
			return
		}
	}
	t.Fatal("YouTube not in the app table")
}

func TestNetifydTimelineCarriesTheCategory(t *testing.T) {
	tbl := newNetifydTable(256, 4096)
	tbl.setFlowApp("cat2", "GitHub", "HTTP/S", "admin")
	tbl.addStats("cat2", 0, 2048, 2048, 8)

	tl := tbl.Timeline()
	if len(tl.Top) != 1 {
		t.Fatalf("top = %d, want 1", len(tl.Top))
	}
	if tl.Top[0].Category != "admin" {
		t.Fatalf("top category = %q, want admin", tl.Top[0].Category)
	}
}
