// mqtt_ha.go: state payload and Home Assistant MQTT Discovery for the MQTT
// integration (#398). State reuses the probes NetGrip already exposes; the
// discovery payloads are what makes Home Assistant create the entities on its
// own.
package modules

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
	"github.com/gonzalop/mq"
)

// mqttState is the JSON published (retained) on netgrip/<node>/state.
type mqttState struct {
	Node         string    `json:"node"`
	Version      string    `json:"version"`
	Hostname     string    `json:"hostname"`
	Model        string    `json:"model"`
	Firmware     string    `json:"firmware"`
	Uptime       int64     `json:"uptime"`
	CPU          float64   `json:"cpu"`
	Busiest      float64   `json:"cpu_busiest"`
	TempC        *float64  `json:"temp_c"`
	Load         []float64 `json:"load"`
	MemUsed      int64     `json:"mem_used"`
	MemTotal     int64     `json:"mem_total"`
	GuestActive  bool      `json:"guest_active"`
	BanipEnabled bool      `json:"banip_enabled"`
	IPv6Enabled  bool      `json:"ipv6_enabled"`
	SQMEnabled   bool      `json:"sqm_enabled"`
	Mode         string    `json:"mode"`
	// Aggregated client telemetry (#401) and Wi-Fi/LAN traffic telemetry (#403).
	ClientsTotal  int     `json:"clients_total"`
	ClientsWifi   int     `json:"clients_wifi"`
	Clients24     int     `json:"clients_24"`
	Clients5      int     `json:"clients_5"`
	ClientsCable  int     `json:"clients_cable"`
	ClientsWeak   int     `json:"clients_weak"`
	WifiMinSignal int     `json:"wifi_min_signal"`
	WifiAvgSignal int     `json:"wifi_avg_signal"`
	RxBytes       int64   `json:"rx_bytes"`
	TxBytes       int64   `json:"tx_bytes"`
	RxMbps        float64 `json:"rx_mbps"`
	TxMbps        float64 `json:"tx_mbps"`
	Ts            int64   `json:"ts"`
}

type mqttBoard struct {
	Model    string `json:"model"`
	Hostname string `json:"hostname"`
	Kernel   string `json:"kernel"`
	Release  struct {
		Distribution string `json:"distribution"`
		Version      string `json:"version"`
		Revision     string `json:"revision"`
	} `json:"release"`
}

// buildMQTTState assembles the state from the existing probes. Every probe is
// best-effort: a failure leaves the zero value rather than aborting.
func buildMQTTState(node, version string) mqttState {
	st := mqttState{Node: node, Version: version, Ts: time.Now().Unix()}

	st.Hostname, _ = os.Hostname()
	if raw, err := ubus.Call("system", "board"); err == nil {
		var b mqttBoard
		if json.Unmarshal(raw, &b) == nil {
			st.Model = b.Model
			if b.Hostname != "" {
				st.Hostname = b.Hostname
			}
			if b.Release.Version != "" {
				st.Firmware = b.Release.Distribution + " " + b.Release.Version
			}
		}
	}
	if info, err := ubus.GetSystemInfo(); err == nil {
		st.Uptime = info.Uptime
		st.Load = info.Load
		st.MemTotal = info.Memory.Total
		st.MemUsed = info.Memory.Total - info.Memory.Available
	}

	cpu := ProbeCPU()
	st.CPU = cpu.UsagePct
	st.Busiest = cpu.Busiest
	st.TempC = cpu.TempC

	st.GuestActive = ProbeGuest().Active
	st.BanipEnabled = ProbeBanipStatusCached().Enabled
	st.IPv6Enabled = ProbeIPv6().State == "enabled"
	st.SQMEnabled = ProbeSQM().Active
	st.Mode = ProbeMode().Mode

	tel := ProbeTelemetry()
	st.ClientsTotal = tel.Total
	st.ClientsWifi = tel.Wifi24 + tel.Wifi5
	st.Clients24 = tel.Wifi24
	st.Clients5 = tel.Wifi5
	st.ClientsCable = tel.Cable
	st.ClientsWeak = tel.Weak
	st.WifiMinSignal = tel.WifiMinSignal
	st.WifiAvgSignal = tel.WifiAvgSignal
	st.RxBytes = tel.RxBytes
	st.TxBytes = tel.TxBytes
	st.RxMbps = tel.RxMbps
	st.TxMbps = tel.TxMbps

	return st
}

// publishState publishes the retained state payload.
func publishState(client *mq.Client, node, version string) {
	payload, err := json.Marshal(buildMQTTState(node, version))
	if err != nil {
		log.Printf("mqtt: marshal state: %v", err)
		return
	}
	mqttPublish(client, stateTopic(node), payload, true)
}

// mqttEntity is one Home Assistant discovery entry.
type mqttEntity struct {
	component string
	objectID  string
	config    map[string]any
}

// publishDiscovery publishes the retained discovery configs so Home Assistant
// creates (or refreshes) the entities. Called on every (re)connect.
func publishDiscovery(client *mq.Client, node, version string) {
	for _, e := range mqttDiscoveryEntities(node, version, boardModel()) {
		payload, err := json.Marshal(e.config)
		if err != nil {
			log.Printf("mqtt: marshal discovery %s: %v", e.objectID, err)
			continue
		}
		topic := "homeassistant/" + e.component + "/netgrip_" + node + "/" + e.objectID + "/config"
		mqttPublish(client, topic, payload, true)
	}
}

// boardModel returns the router model from ubus, or "" when unavailable.
func boardModel() string {
	raw, err := ubus.Call("system", "board")
	if err != nil {
		return ""
	}
	var b mqttBoard
	if json.Unmarshal(raw, &b) != nil {
		return ""
	}
	return b.Model
}

// mqttDiscoveryEntities builds the Home Assistant discovery entries. Pure
// (no I/O) so it can be tested directly.
func mqttDiscoveryEntities(node, version, model string) []mqttEntity {
	device := map[string]any{
		"identifiers":  []string{"netgrip_" + node},
		"name":         "NetGrip " + node,
		"manufacturer": "NetGrip",
		"model":        model,
		"sw_version":   version,
	}
	avail := map[string]any{
		"availability_topic":    availabilityTopic(node),
		"payload_available":     "online",
		"payload_not_available": "offline",
	}

	state := stateTopic(node)
	newEntity := func(component, objectID string, extra map[string]any) mqttEntity {
		cfg := map[string]any{
			"unique_id":   "netgrip_" + node + "_" + objectID,
			"object_id":   "netgrip_" + node + "_" + objectID,
			"device":      device,
			"state_topic": state,
		}
		for k, v := range avail {
			cfg[k] = v
		}
		for k, v := range extra {
			cfg[k] = v
		}
		return mqttEntity{component: component, objectID: objectID, config: cfg}
	}

	return []mqttEntity{
		newEntity("switch", "guest_wifi", map[string]any{
			"name":           "Guest WiFi",
			"value_template": "{{ 'ON' if value_json.guest_active else 'OFF' }}",
			"command_topic":  "netgrip/" + node + "/command/guest_wifi",
			"payload_on":     "ON",
			"payload_off":    "OFF",
			"icon":           "mdi:wifi-plus",
		}),
		newEntity("switch", "banip", map[string]any{
			"name":           "banIP",
			"value_template": "{{ 'ON' if value_json.banip_enabled else 'OFF' }}",
			"command_topic":  "netgrip/" + node + "/command/banip",
			"payload_on":     "ON",
			"payload_off":    "OFF",
			"icon":           "mdi:shield-lock",
		}),
		newEntity("switch", "ipv6", map[string]any{
			"name":           "IPv6",
			"value_template": "{{ 'ON' if value_json.ipv6_enabled else 'OFF' }}",
			"command_topic":  "netgrip/" + node + "/command/ipv6",
			"payload_on":     "ON",
			"payload_off":    "OFF",
			"icon":           "mdi:ip-network",
		}),
		newEntity("switch", "sqm", map[string]any{
			"name":           "SQM",
			"value_template": "{{ 'ON' if value_json.sqm_enabled else 'OFF' }}",
			"command_topic":  "netgrip/" + node + "/command/sqm",
			"payload_on":     "ON",
			"payload_off":    "OFF",
			"icon":           "mdi:speedometer",
		}),
		newEntity("button", "banip_reload", map[string]any{
			"name":          "Reload banIP feeds",
			"command_topic": "netgrip/" + node + "/command/banip",
			"payload_press": "RELOAD",
			"icon":          "mdi:shield-refresh",
		}),
		newEntity("button", "reboot", map[string]any{
			"name":            "Reboot",
			"command_topic":   "netgrip/" + node + "/command/reboot",
			"payload_press":   "PRESS",
			"icon":            "mdi:restart",
			"entity_category": "config",
		}),
		newEntity("sensor", "cpu_usage", map[string]any{
			"name":                "CPU usage",
			"value_template":      "{{ value_json.cpu }}",
			"unit_of_measurement": "%",
			"state_class":         "measurement",
			"icon":                "mdi:speedometer",
		}),
		newEntity("sensor", "cpu_busiest", map[string]any{
			"name":                "CPU busiest core",
			"value_template":      "{{ value_json.cpu_busiest }}",
			"unit_of_measurement": "%",
			"state_class":         "measurement",
			"icon":                "mdi:speedometer",
			"entity_category":     "diagnostic",
		}),
		newEntity("sensor", "temperature", map[string]any{
			"name":                "Temperature",
			"value_template":      "{{ value_json.temp_c }}",
			"unit_of_measurement": "°C",
			"device_class":        "temperature",
			"state_class":         "measurement",
		}),
		newEntity("sensor", "memory_used", map[string]any{
			"name":                "Memory used",
			"value_template":      "{{ (value_json.mem_used / 1048576) | round(0) }}",
			"unit_of_measurement": "MB",
			"state_class":         "measurement",
			"icon":                "mdi:memory",
			"entity_category":     "diagnostic",
		}),
		newEntity("sensor", "uptime", map[string]any{
			"name":                "Uptime",
			"value_template":      "{{ value_json.uptime }}",
			"unit_of_measurement": "s",
			"device_class":        "duration",
			"entity_category":     "diagnostic",
			"icon":                "mdi:timer-outline",
		}),
		newEntity("sensor", "mode", map[string]any{
			"name":            "Mode",
			"value_template":  "{{ value_json.mode }}",
			"icon":            "mdi:router-wireless",
			"entity_category": "diagnostic",
		}),
		newEntity("sensor", "version", map[string]any{
			"name":            "NetGrip version",
			"value_template":  "{{ value_json.version }}",
			"icon":            "mdi:information-outline",
			"entity_category": "diagnostic",
		}),
		newEntity("sensor", "clients_total", map[string]any{
			"name":           "Clients",
			"value_template": "{{ value_json.clients_total }}",
			"state_class":    "measurement",
			"icon":           "mdi:account-network",
		}),
		newEntity("sensor", "clients_wifi", map[string]any{
			"name":            "Wi-Fi clients",
			"value_template":  "{{ value_json.clients_wifi }}",
			"state_class":     "measurement",
			"icon":            "mdi:wifi",
			"entity_category": "diagnostic",
		}),
		newEntity("sensor", "clients_cable", map[string]any{
			"name":            "Wired clients",
			"value_template":  "{{ value_json.clients_cable }}",
			"state_class":     "measurement",
			"icon":            "mdi:ethernet",
			"entity_category": "diagnostic",
		}),
		newEntity("sensor", "clients_weak", map[string]any{
			"name":            "Weak-signal clients",
			"value_template":  "{{ value_json.clients_weak }}",
			"state_class":     "measurement",
			"icon":            "mdi:wifi-alert",
			"entity_category": "diagnostic",
		}),
		newEntity("sensor", "wifi_signal_min", map[string]any{
			"name":                "Weakest client signal",
			"value_template":      "{{ value_json.wifi_min_signal }}",
			"unit_of_measurement": "dBm",
			"device_class":        "signal_strength",
			"state_class":         "measurement",
			"icon":                "mdi:wifi-strength-1",
			"entity_category":     "diagnostic",
		}),
		newEntity("sensor", "wifi_signal_avg", map[string]any{
			"name":                "Average client signal",
			"value_template":      "{{ value_json.wifi_avg_signal }}",
			"unit_of_measurement": "dBm",
			"device_class":        "signal_strength",
			"state_class":         "measurement",
			"icon":                "mdi:wifi-strength-3",
			"entity_category":     "diagnostic",
		}),
		newEntity("sensor", "rx_mbps", map[string]any{
			"name":                "LAN traffic in",
			"value_template":      "{{ value_json.rx_mbps }}",
			"unit_of_measurement": "Mbit/s",
			"state_class":         "measurement",
			"icon":                "mdi:download-network",
		}),
		newEntity("sensor", "tx_mbps", map[string]any{
			"name":                "LAN traffic out",
			"value_template":      "{{ value_json.tx_mbps }}",
			"unit_of_measurement": "Mbit/s",
			"state_class":         "measurement",
			"icon":                "mdi:upload-network",
		}),
		newEntity("sensor", "rx_bytes", map[string]any{
			"name":                "LAN received total",
			"value_template":      "{{ value_json.rx_bytes }}",
			"unit_of_measurement": "B",
			"device_class":        "data_size",
			"state_class":         "total_increasing",
			"icon":                "mdi:download-network",
			"entity_category":     "diagnostic",
		}),
		newEntity("sensor", "tx_bytes", map[string]any{
			"name":                "LAN sent total",
			"value_template":      "{{ value_json.tx_bytes }}",
			"unit_of_measurement": "B",
			"device_class":        "data_size",
			"state_class":         "total_increasing",
			"icon":                "mdi:upload-network",
			"entity_category":     "diagnostic",
		}),
	}
}
