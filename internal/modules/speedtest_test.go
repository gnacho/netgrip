package modules

import (
	"context"
	"testing"

	"github.com/m-lab/ndt7-client-go/spec"
)

// La tasa sale de la última muestra acumulada del servidor (bytes / tiempo),
// y el RTT de TCP_INFO en microsegundos (#637).
func TestNDTPhaseComputesMbps(t *testing.T) {
	start := func(context.Context) (<-chan spec.Measurement, error) {
		ch := make(chan spec.Measurement, 1)
		ch <- spec.Measurement{
			AppInfo: &spec.AppInfo{ElapsedTime: 1_000_000, NumBytes: 12_500_000}, // 100 Mbps en 1 s
			TCPInfo: func() *spec.TCPInfo { // MinRTT viene de LinuxTCPInfo (campo promovido)
				ti := &spec.TCPInfo{}
				ti.MinRTT = 6000 // 6 ms
				return ti
			}(),
		}
		close(ch)
		return ch, nil
	}

	mbps, rtt, err := ndtPhase(start)
	if err != nil {
		t.Fatalf("ndtPhase: %v", err)
	}
	if mbps < 99.9 || mbps > 100.1 {
		t.Fatalf("mbps = %.2f, want 100", mbps)
	}
	if rtt != 6000 {
		t.Fatalf("rtt = %.0f us, want 6000", rtt)
	}
}

func TestNDTPhaseWithoutSamplesFails(t *testing.T) {
	start := func(context.Context) (<-chan spec.Measurement, error) {
		ch := make(chan spec.Measurement)
		close(ch)
		return ch, nil
	}
	if _, _, err := ndtPhase(start); err == nil {
		t.Fatal("sin mediciones debe devolver error")
	}
}

func TestStartSpeedTestIsSingleFlight(t *testing.T) {
	speedTest.Lock()
	speedTest.result = SpeedTestResult{State: "running", Phase: "download"}
	speedTest.Unlock()
	t.Cleanup(func() {
		speedTest.Lock()
		speedTest.result = SpeedTestResult{State: "idle"}
		speedTest.Unlock()
	})

	if got := StartSpeedTest(); got.State != "running" {
		t.Fatalf("state = %q, want running", got.State)
	}
}
