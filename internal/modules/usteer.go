package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// UsteerClient is one client connected to a usteer-managed AP.
type UsteerClient struct {
	MAC    string `json:"mac"`
	Signal int    `json:"signal"`
}

// UsteerAP is one access point seen by usteer.
type UsteerAP struct {
	BSSID    string         `json:"bssid"`
	SSID     string         `json:"ssid"`
	Hostname string         `json:"hostname"`
	Iface    string         `json:"iface"`
	Channel  int            `json:"channel"`
	Freq     int            `json:"freq"`
	Util     int            `json:"util"`
	NumSta   int            `json:"num_sta"`
	Local    bool           `json:"local"`
	Clients  []UsteerClient `json:"clients"`
}

// usteerAPRaw matches the objects returned by local_info/remote_info.
type usteerAPRaw struct {
	BSSID  string `json:"bssid"`
	SSID   string `json:"ssid"`
	Freq   int    `json:"freq"`
	NAssoc int    `json:"n_assoc"`
	Load   int    `json:"load"`
}

// UsteerNetwork parses usteer local_info + remote_info into the mesh view.
// Returns an error when usteer is not running on this router.
func UsteerNetwork() ([]UsteerAP, error) {
	localOut, err := exec.Command("ubus", "call", "usteer", "local_info").Output()
	if err != nil {
		return nil, fmt.Errorf("usteer is not running on this router")
	}
	remoteOut, _ := exec.Command("ubus", "call", "usteer", "remote_info").Output()
	clientsOut, _ := exec.Command("ubus", "call", "usteer", "connected_clients").Output()

	clientsByIface := map[string]map[string]usteerClientRaw{}
	_ = json.Unmarshal(clientsOut, &clientsByIface)

	var aps []UsteerAP
	parse := func(local bool, key string, raw usteerAPRaw) {
		if raw.SSID == "" || raw.BSSID == "" {
			return
		}
		hostname := ""
		if i := strings.IndexByte(key, '#'); i >= 0 {
			hostname = key[:i]
		}
		ap := UsteerAP{
			BSSID:    strings.ToUpper(raw.BSSID),
			SSID:     raw.SSID,
			Hostname: hostname,
			Iface:    key,
			Channel:  channelFromFreq(raw.Freq),
			Freq:     raw.Freq,
			Util:     raw.Load,
			NumSta:   raw.NAssoc,
			Local:    local,
			Clients:  []UsteerClient{},
		}
		if macs, ok := clientsByIface[key]; ok {
			for mac, c := range macs {
				if c.Signal >= 0 {
					continue
				}
				ap.Clients = append(ap.Clients, UsteerClient{
					MAC:    strings.ToUpper(mac),
					Signal: c.Signal,
				})
			}
		}
		aps = append(aps, ap)
	}

	var local map[string]usteerAPRaw
	if err := json.Unmarshal(localOut, &local); err == nil {
		for iface, raw := range local {
			parse(true, iface, raw)
		}
	}
	var remote map[string]usteerAPRaw
	if err := json.Unmarshal(remoteOut, &remote); err == nil {
		for key, raw := range remote {
			parse(false, key, raw)
		}
	}
	if aps == nil {
		return []UsteerAP{}, nil
	}
	return aps, nil
}

type usteerClientRaw struct {
	Signal int `json:"signal"`
}

// channelFromFreq returns the IEEE 802.11 channel number for common
// frequencies. Returns 0 for unknown frequencies.
func channelFromFreq(freq int) int {
	// 2.4 GHz: 2412 = ch1, step 5 MHz.
	if freq >= 2412 && freq <= 2472 && (freq-2412)%5 == 0 {
		return 1 + (freq-2412)/5
	}
	// 5 GHz common channels.
	channels5 := map[int]int{
		5180: 36, 5200: 40, 5220: 44, 5240: 48,
		5260: 52, 5280: 56, 5300: 60, 5320: 64,
		5500: 100, 5520: 104, 5540: 108, 5560: 112,
		5580: 116, 5600: 120, 5620: 124, 5640: 128,
		5660: 132, 5680: 136, 5700: 140, 5720: 144,
		5745: 149, 5765: 153, 5785: 157, 5805: 161, 5825: 165,
	}
	if ch, ok := channels5[freq]; ok {
		return ch
	}
	return 0
}
