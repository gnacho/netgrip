package ubus

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// WirelessInterface is one AP interface on a radio, with its clients.
// Sensitive UCI fields (key, wep keys, radius secrets) are NEVER exposed.
type WirelessInterface struct {
	Ifname     string           `json:"ifname"`
	SSID       string           `json:"ssid"`
	Encryption string           `json:"encryption"`
	Disabled   bool             `json:"disabled"`
	Hidden     bool             `json:"hidden"`
	BSSID      string           `json:"bssid"`
	RFaceName  string           `json:"rname"` // UCI section name, e.g. default_radio0
	Clients    []WirelessClient `json:"clients"`
}

// WirelessRadio is one physical radio.
type WirelessRadio struct {
	Name       string              `json:"name"`
	Up         bool                `json:"up"`
	Band       string              `json:"band"`
	Channel    string              `json:"channel"`
	Htmode     string              `json:"htmode"`
	TxPower    int                 `json:"txpower"`
	Interfaces []WirelessInterface `json:"interfaces"`
}

// GetWirelessStatus merges `ubus call network.wireless status` with
// hostapd get_clients. The wireless status payload includes the PSK in
// plaintext ("key"): it is deliberately never unmarshalled into the output.
func GetWirelessStatus() ([]WirelessRadio, error) {
	raw, err := Call("network.wireless", "status")
	if err != nil {
		return nil, err
	}
	var payload map[string]struct {
		Up     bool `json:"up"`
		Config struct {
			Band    string `json:"band"`
			Channel string `json:"channel"`
			Htmode  string `json:"htmode"`
			TxPower int    `json:"txpower"`
		} `json:"config"`
		Interfaces []struct {
			Ifname  string `json:"ifname"`
			Section string `json:"section"`
			Config  struct {
				SSID       string `json:"ssid"`
				Encryption string `json:"encryption"`
				Disabled   bool   `json:"disabled"`
			} `json:"config"`
		} `json:"interfaces"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	clientsByIface, err := WirelessClients()
	if err != nil {
		clientsByIface = map[string][]WirelessClient{}
	}
	hiddenBySection := wifiHiddenBySection()

	radios := make([]WirelessRadio, 0, len(payload))
	for name, r := range payload {
		radio := WirelessRadio{
			Name:    name,
			Up:      r.Up,
			Band:    r.Config.Band,
			Channel: r.Config.Channel,
			Htmode:  r.Config.Htmode,
			TxPower: r.Config.TxPower,
		}
		for _, iface := range r.Interfaces {
			clients := clientsByIface["hostapd."+iface.Ifname]
			if clients == nil {
				clients = []WirelessClient{}
			}
			radio.Interfaces = append(radio.Interfaces, WirelessInterface{
				Ifname:     iface.Ifname,
				SSID:       iface.Config.SSID,
				Encryption: iface.Config.Encryption,
				Disabled:   iface.Config.Disabled,
				Hidden:     hiddenBySection[iface.Section],
				BSSID:      ifaceAddr(iface.Ifname),
				RFaceName:  iface.Section,
				Clients:    clients,
			})
		}
		if radio.Interfaces == nil {
			radio.Interfaces = []WirelessInterface{}
		}
		radios = append(radios, radio)
	}
	return radios, nil
}

// wifiHiddenBySection reads the hidden option for every wifi-iface section.
func wifiHiddenBySection() map[string]bool {
	out, err := exec.Command("sh", "-c", "uci show wireless | grep '=wifi-iface' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return map[string]bool{}
	}
	m := map[string]bool{}
	for _, section := range strings.Fields(string(out)) {
		m[section] = uciOpt("wireless."+section+".hidden") == "1"
	}
	return m
}

func uciOpt(key string) string {
	out, err := exec.Command("uci", "-q", "get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ifaceAddr returns the MAC/BSSID of an interface via `iw dev <iface> info`.
func ifaceAddr(iface string) string {
	out, err := exec.Command("iw", "dev", iface, "info").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "addr ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "addr "))
		}
	}
	return ""
}
