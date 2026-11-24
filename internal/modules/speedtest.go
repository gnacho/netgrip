package modules

import (
	"context"
	"fmt"
	"sync"
	"time"

	ndt7 "github.com/m-lab/ndt7-client-go"
	"github.com/m-lab/ndt7-client-go/spec"
)

// Test de velocidad M-Lab NDT7 (#637), igual que el de NetPulse.
//
// Se ejecuta EN EL ROUTER: mide la línea del operador, no el WiFi del móvil
// que abre el panel. Un solo test a la vez (dos saturarían el enlace y
// falsearían el resultado) y el estado se guarda en memoria: no se persiste
// histórico, la tarjeta muestra la última medición de esta sesión.

const (
	speedTestClient = "netgrip-router-panel"
	// Cada fase dura ~10 s por especificación; 30 s de margen cubre el
	// descubrimiento del servidor y un enlace lento.
	speedPhaseTimeout = 30 * time.Second
)

// SpeedTestResult is the public view of the last measurement.
type SpeedTestResult struct {
	State      string  `json:"state"`           // idle | running | done | error
	Phase      string  `json:"phase,omitempty"` // download | upload
	DownMbps   float64 `json:"down_mbps"`
	UpMbps     float64 `json:"up_mbps"`
	PingMs     float64 `json:"ping_ms,omitempty"`
	Server     string  `json:"server,omitempty"`
	Error      string  `json:"error,omitempty"`
	StartedAt  int64   `json:"started_at,omitempty"`
	FinishedAt int64   `json:"finished_at,omitempty"`
}

var speedTest = struct {
	sync.Mutex
	result SpeedTestResult
}{result: SpeedTestResult{State: "idle"}}

// SpeedTestStatus returns the current state of the measurement.
func SpeedTestStatus() SpeedTestResult {
	speedTest.Lock()
	defer speedTest.Unlock()
	return speedTest.result
}

// StartSpeedTest starts a measurement in the background and returns the new
// state. Calling it while one is running is a no-op that returns the running
// state, so a double click does not start two tests.
func StartSpeedTest() SpeedTestResult {
	speedTest.Lock()
	defer speedTest.Unlock()
	if speedTest.result.State == "running" {
		return speedTest.result
	}
	speedTest.result = SpeedTestResult{State: "running", Phase: "download", StartedAt: time.Now().Unix()}
	go runSpeedTest()
	return speedTest.result
}

func setSpeedTest(patch func(r *SpeedTestResult)) {
	speedTest.Lock()
	defer speedTest.Unlock()
	patch(&speedTest.result)
}

func runSpeedTest() {
	defer func() {
		// Un pánico en el cliente NDT no debe dejar la tarjeta "en marcha"
		// para siempre.
		if r := recover(); r != nil {
			setSpeedTest(func(r *SpeedTestResult) {
				r.State = "error"
				r.Phase = ""
				r.Error = fmt.Sprintf("panic: %v", r)
				r.FinishedAt = time.Now().Unix()
			})
		}
	}()

	client := ndt7.NewClient(speedTestClient, speedTestClient)

	down, rttUs, err := ndtPhase(func(ctx context.Context) (<-chan spec.Measurement, error) {
		return client.StartDownload(ctx)
	})
	if err != nil {
		setSpeedTest(func(r *SpeedTestResult) {
			r.State, r.Phase, r.Error = "error", "", fmt.Sprintf("bajada: %v", err)
			r.FinishedAt = time.Now().Unix()
		})
		return
	}
	server := client.FQDN
	setSpeedTest(func(r *SpeedTestResult) {
		r.DownMbps = down
		r.Phase = "upload"
		if rttUs > 0 {
			r.PingMs = rttUs / 1000
		}
		r.Server = server
	})

	up, _, err := ndtPhase(func(ctx context.Context) (<-chan spec.Measurement, error) {
		return client.StartUpload(ctx)
	})
	if err != nil {
		setSpeedTest(func(r *SpeedTestResult) {
			r.State, r.Phase, r.Error = "error", "", fmt.Sprintf("subida: %v", err)
			r.FinishedAt = time.Now().Unix()
		})
		return
	}
	setSpeedTest(func(r *SpeedTestResult) {
		r.State, r.Phase = "done", ""
		r.UpMbps = up
		if r.Server == "" {
			r.Server = client.FQDN
		}
		r.FinishedAt = time.Now().Unix()
	})
}

// ndtPhase runs one phase and returns the rate in Mbps from the last
// application-level sample (cumulative bytes over elapsed microseconds, both
// measured by the server) plus the smallest RTT seen, in microseconds.
func ndtPhase(start func(context.Context) (<-chan spec.Measurement, error)) (mbps, minRTTus float64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), speedPhaseTimeout)
	defer cancel()

	ch, err := start(ctx)
	if err != nil {
		return 0, 0, err
	}
	var last *spec.AppInfo
	for m := range ch {
		if m.AppInfo != nil {
			cp := *m.AppInfo
			last = &cp
		}
		if m.TCPInfo != nil && m.TCPInfo.MinRTT > 0 {
			minRTTus = float64(m.TCPInfo.MinRTT)
		}
	}
	if last == nil || last.ElapsedTime <= 0 {
		return 0, 0, fmt.Errorf("el servidor no envió mediciones")
	}
	bps := float64(last.NumBytes) * 8 / (float64(last.ElapsedTime) / 1e6)
	return bps / 1e6, minRTTus, nil
}
