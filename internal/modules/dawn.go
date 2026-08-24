package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// DawnClient is one client heard by a DAWN AP.
type DawnClient struct {
	MAC    string `json:"mac"`
	Signal int    `json:"signal"`
}

// DawnAP is one access point in the DAWN mesh.
type DawnAP struct {
	BSSID    string       `json:"bssid"`
	SSID     string       `json:"ssid"`
	Hostname string       `json:"hostname"`
	Iface    string       `json:"iface"`
	Channel  int          `json:"channel"`
	Freq     int          `json:"freq"`
	Util     int          `json:"util"`
	NumSta   int          `json:"num_sta"`
	Local    bool         `json:"local"`
	Clients  []DawnClient `json:"clients"`
}

var reDawnMAC = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)

// DawnNetwork parses `ubus call dawn get_network` into the mesh view.
// Returns an error when dawn is not running on this router.
func DawnNetwork() ([]DawnAP, error) {
	out, err := exec.Command("ubus", "call", "dawn", "get_network").Output()
	if err != nil {
		return nil, fmt.Errorf("dawn is not running on this router")
	}
	var payload map[string]map[string]json.RawMessage
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse dawn network: %w", err)
	}
	var aps []DawnAP
	for ssid, bssids := range payload {
		for bssid, raw := range bssids {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				continue
			}
			ap := DawnAP{BSSID: bssid, SSID: ssid, Clients: []DawnClient{}}
			for key, value := range fields {
				switch key {
				case "hostname":
					ap.Hostname = jsonString(value)
				case "iface":
					ap.Iface = jsonString(value)
				case "channel":
					ap.Channel = jsonInt(value)
				case "freq":
					ap.Freq = jsonInt(value)
				case "channel_utilization":
					ap.Util = jsonInt(value)
				case "num_sta":
					ap.NumSta = jsonInt(value)
				case "local":
					ap.Local = jsonBool(value)
				default:
					if reDawnMAC.MatchString(key) {
						var client struct {
							Signal int `json:"signal"`
						}
						if err := json.Unmarshal(value, &client); err == nil && client.Signal != 0 {
							ap.Clients = append(ap.Clients, DawnClient{MAC: strings.ToLower(key), Signal: client.Signal})
						}
					}
				}
			}
			aps = append(aps, ap)
		}
	}
	if aps == nil {
		return []DawnAP{}, nil
	}
	return aps, nil
}

func jsonString(v json.RawMessage) string {
	var s string
	_ = json.Unmarshal(v, &s)
	return s
}

func jsonInt(v json.RawMessage) int {
	var n int
	_ = json.Unmarshal(v, &n)
	return n
}

func jsonBool(v json.RawMessage) bool {
	var b bool
	_ = json.Unmarshal(v, &b)
	return b
}
