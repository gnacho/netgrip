package modules

import "testing"

func TestAggregateClients(t *testing.T) {
	tel := aggregateClients([]Client{
		{Type: "wifi24", Signal: -50, RxBytes: 100, TxBytes: 200},
		{Type: "wifi5", Signal: -80, RxBytes: 300, TxBytes: 400}, // weak
		{Type: "wifi5", Signal: 0, RxBytes: 1, TxBytes: 2},       // no measurement
		{Type: "cable", RxBytes: 5, TxBytes: 6},
	})
	if tel.Total != 4 {
		t.Fatalf("total = %d, want 4", tel.Total)
	}
	if tel.Wifi24 != 1 || tel.Wifi5 != 2 || tel.Cable != 1 {
		t.Fatalf("band split = %d/%d/%d, want 1/2/1", tel.Wifi24, tel.Wifi5, tel.Cable)
	}
	if tel.Weak != 1 {
		t.Fatalf("weak = %d, want 1 (signal 0 must not count)", tel.Weak)
	}
	if tel.RxBytes != 406 || tel.TxBytes != 608 {
		t.Fatalf("bytes = %d/%d, want 406/608", tel.RxBytes, tel.TxBytes)
	}
}

func TestAggregateClientsEmpty(t *testing.T) {
	tel := aggregateClients(nil)
	if tel == nil || tel.Total != 0 {
		t.Fatalf("expected a zero telemetry, got %+v", tel)
	}
}

func TestProbeTelemetryNeverNil(t *testing.T) {
	if tel := ProbeTelemetry(); tel == nil {
		t.Fatal("ProbeTelemetry returned nil")
	}
}
