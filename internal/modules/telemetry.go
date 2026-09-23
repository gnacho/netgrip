// telemetry.go: aggregated client and traffic telemetry (#401, #403) used by
// the MQTT integration to expose router activity to Home Assistant without a
// NetPulse server.
package modules

import (
	"math"
	"sync"
	"time"
)

// TelemetryProbe is the aggregated view of the connected clients plus the LAN
// traffic counters.
type TelemetryProbe struct {
	Total         int `json:"total"`
	Wifi24        int `json:"wifi24"`
	Wifi5         int `json:"wifi5"`
	Cable         int `json:"cable"`
	Weak          int `json:"weak"`
	WifiMinSignal int `json:"wifi_min_signal"`
	WifiAvgSignal int `json:"wifi_avg_signal"`
	// LAN bridge counters: cumulative bytes and the rate derived from the
	// previous sample. "Rx" is traffic entering the bridge (from the LAN
	// clients), "tx" is traffic leaving it towards them.
	RxBytes int64   `json:"rx_bytes"`
	TxBytes int64   `json:"tx_bytes"`
	RxMbps  float64 `json:"rx_mbps"`
	TxMbps  float64 `json:"tx_mbps"`
}

// weakSignalDbm: clients at or below this signal count as weak. Chosen from
// the usual Wi-Fi thresholds; a value of 0 means "no measurement" and never
// counts.
const weakSignalDbm = -75

// telemetryTTL dedupes the probe between the panel and the MQTT publisher:
// ListClients forks several commands, so it is cached briefly.
const telemetryTTL = 15 * time.Second

// trafficSample keeps the last LAN bridge counters so the next sample can
// turn them into a rate. Package-level on purpose: the probe is cached, so
// samples are spaced by telemetryTTL at the very least.
type trafficSample struct {
	at     time.Time
	rx, tx int64
}

var (
	trafficMu   sync.Mutex
	lastTraffic trafficSample
)

// ProbeTelemetry returns the aggregated client telemetry, cached for
// telemetryTTL.
func ProbeTelemetry() *TelemetryProbe {
	p, _ := CachedRead("telemetry", telemetryTTL, func() (*TelemetryProbe, error) {
		return sampleTelemetry(), nil
	})
	if p == nil {
		return &TelemetryProbe{}
	}
	return p
}

// sampleTelemetry folds the client list and the LAN bridge counters into one
// probe, deriving the traffic rate from the previous sample.
func sampleTelemetry() *TelemetryProbe {
	t := aggregateClients(ListClients(""))
	t.RxBytes, t.TxBytes = bridgeCounters()

	now := time.Now()
	trafficMu.Lock()
	if !lastTraffic.at.IsZero() {
		secs := now.Sub(lastTraffic.at).Seconds()
		t.RxMbps = rateMbps(t.RxBytes-lastTraffic.rx, secs)
		t.TxMbps = rateMbps(t.TxBytes-lastTraffic.tx, secs)
	}
	lastTraffic = trafficSample{at: now, rx: t.RxBytes, tx: t.TxBytes}
	trafficMu.Unlock()

	return t
}

// bridgeCounters returns the cumulative LAN bridge counters, the same source
// the panel traffic graph uses. Per-client counters are not used: most devices
// do not account per station and report zero, and APs often have no WAN at all.
func bridgeCounters() (int64, int64) {
	bridge := LANBridge()
	for _, c := range NetDevCounters() {
		if c.Name == bridge {
			return c.RxBytes, c.TxBytes
		}
	}
	return 0, 0
}

// rateMbps converts a byte delta over an interval into Mbit/s, rounded to two
// decimals. A negative delta (counter reset) reads as zero.
func rateMbps(deltaBytes int64, seconds float64) float64 {
	if seconds <= 0 || deltaBytes <= 0 {
		return 0
	}
	return math.Round(float64(deltaBytes)*8/seconds/1e6*100) / 100
}

// aggregateClients folds a client list into the telemetry counters. Pure so it
// can be tested without touching the router.
func aggregateClients(clients []Client) *TelemetryProbe {
	t := &TelemetryProbe{}
	var signalSum, signalCount int
	for _, c := range clients {
		t.Total++
		switch c.Type {
		case "wifi5":
			t.Wifi5++
		case "wifi24":
			t.Wifi24++
		default:
			t.Cable++
		}
		// Only Wi-Fi clients with a real measurement feed the signal figures.
		if (c.Type != "wifi24" && c.Type != "wifi5") || c.Signal == 0 {
			continue
		}
		signalSum += c.Signal
		signalCount++
		if t.WifiMinSignal == 0 || c.Signal < t.WifiMinSignal {
			t.WifiMinSignal = c.Signal
		}
		if c.Signal < weakSignalDbm {
			t.Weak++
		}
	}
	if signalCount > 0 {
		t.WifiAvgSignal = int(math.Round(float64(signalSum) / float64(signalCount)))
	}
	return t
}
