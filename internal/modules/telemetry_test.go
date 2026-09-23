package modules

import "testing"

func TestAggregateClients(t *testing.T) {
	tel := aggregateClients([]Client{
		{Type: "wifi24", Signal: -50},
		{Type: "wifi5", Signal: -80}, // weak
		{Type: "wifi5", Signal: 0},   // no measurement
		{Type: "cable"},
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
	if tel.WifiMinSignal != -80 {
		t.Fatalf("min signal = %d, want -80", tel.WifiMinSignal)
	}
	if tel.WifiAvgSignal != -65 {
		t.Fatalf("avg signal = %d, want -65 (only measured clients)", tel.WifiAvgSignal)
	}
}

func TestAggregateClientsNoMeasurement(t *testing.T) {
	tel := aggregateClients([]Client{
		{Type: "cable"},
		{Type: "wifi5", Signal: 0},
	})
	if tel.WifiMinSignal != 0 || tel.WifiAvgSignal != 0 {
		t.Fatalf("signal figures = %d/%d, want 0/0", tel.WifiMinSignal, tel.WifiAvgSignal)
	}
	if tel.Weak != 0 {
		t.Fatalf("weak = %d, want 0", tel.Weak)
	}
}

func TestAggregateClientsEmpty(t *testing.T) {
	tel := aggregateClients(nil)
	if tel == nil || tel.Total != 0 {
		t.Fatalf("expected a zero telemetry, got %+v", tel)
	}
}

func TestRateMbps(t *testing.T) {
	cases := []struct {
		name    string
		delta   int64
		seconds float64
		want    float64
	}{
		{"no time", 1000, 0, 0},
		{"negative delta (counter reset)", -1000, 60, 0},
		{"no traffic", 0, 60, 0},
		{"one MB over a minute", 7_500_000, 60, 1},
		{"one MB in one second", 1_000_000, 1, 8},
		{"rounded to two decimals", 1_234_567, 60, 0.16},
	}
	for _, tc := range cases {
		if got := rateMbps(tc.delta, tc.seconds); got != tc.want {
			t.Errorf("%s: rateMbps(%d, %v) = %v, want %v", tc.name, tc.delta, tc.seconds, got, tc.want)
		}
	}
}

func TestProbeTelemetryNeverNil(t *testing.T) {
	if tel := ProbeTelemetry(); tel == nil {
		t.Fatal("ProbeTelemetry returned nil")
	}
}
