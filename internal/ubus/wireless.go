package ubus

import "encoding/json"

// WirelessInterface is one AP interface on a radio, with its clients.
// Sensitive UCI fields (key, wep keys, radius secrets) are NEVER exposed.
type WirelessInterface struct {
	Ifname     string           `json:"ifname"`
	SSID       string           `json:"ssid"`
	Encryption string           `json:"encryption"`
	Disabled   bool             `json:"disabled"`
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
			Ifname string `json:"ifname"`
			Config struct {
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
