// telemetry.go: aggregated client and traffic telemetry (#401) used by the
// MQTT integration to expose router activity to Home Assistant without a
// NetPulse server.
package modules

import "time"

// TelemetryProbe is the aggregated view of the connected clients.
type TelemetryProbe struct {
	Total   int   `json:"total"`
	Wifi24  int   `json:"wifi24"`
	Wifi5   int   `json:"wifi5"`
	Cable   int   `json:"cable"`
	Weak    int   `json:"weak"`
	RxBytes int64 `json:"rx_bytes"`
	TxBytes int64 `json:"tx_bytes"`
}

// weakSignalDbm: clients at or below this signal count as weak. Chosen from
// the usual Wi-Fi thresholds; a value of 0 means "no measurement" and never
// counts.
const weakSignalDbm = -75

// telemetryTTL dedupes the probe between the panel and the MQTT publisher:
// ListClients forks several commands, so it is cached briefly.
const telemetryTTL = 15 * time.Second

// ProbeTelemetry returns the aggregated client telemetry, cached for
// telemetryTTL.
func ProbeTelemetry() *TelemetryProbe {
	p, _ := CachedRead("telemetry", telemetryTTL, func() (*TelemetryProbe, error) {
		return aggregateClients(ListClients("")), nil
	})
	if p == nil {
		return &TelemetryProbe{}
	}
	return p
}

// aggregateClients folds a client list into the telemetry counters. Pure so it
// can be tested without touching the router.
func aggregateClients(clients []Client) *TelemetryProbe {
	t := &TelemetryProbe{}
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
		if c.Type != "cable" && c.Signal != 0 && c.Signal < weakSignalDbm {
			t.Weak++
		}
		t.RxBytes += c.RxBytes
		t.TxBytes += c.TxBytes
	}
	return t
}
