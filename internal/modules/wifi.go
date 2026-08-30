package modules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// WifiEdit is the user-provided change to one AP interface (a "radio"'s
// principal network). Empty fields are left unchanged. Key is write-only:
// it is never read back.
type WifiEdit struct {
	Section    string `json:"section"` // UCI section, e.g. default_radio0
	SSID       string `json:"ssid,omitempty"`
	Key        string `json:"key,omitempty"`
	Encryption string `json:"encryption,omitempty"`
	Hidden     *bool  `json:"hidden,omitempty"`
	Disabled   *bool  `json:"disabled,omitempty"`
	// MAC sets a fixed BSSID (e.g. "00:11:22:33:44:55"); empty means keep.
	MAC string `json:"mac,omitempty"`
}

// WifiUI is the state an interface editor needs: current values safe to
// display (never the PSK) plus the BSSID and hidden state.
type WifiUI struct {
	Section    string                `json:"section"`
	Radio      string                `json:"radio"`
	Ifname     string                `json:"ifname"`
	Band       string                `json:"band"`
	SSID       string                `json:"ssid"`
	Encryption string                `json:"encryption"`
	HasKey     bool                  `json:"has_key"`
	Hidden     bool                  `json:"hidden"`
	MAC        string                `json:"mac"`
	BSSID      string                `json:"bssid"`
	Disabled   bool                  `json:"disabled"`
	Clients    []ubus.WirelessClient `json:"clients"`
}

// ProbeWifiUI returns the editable state of all radio interfaces.
func ProbeWifiUI() ([]WifiUI, error) {
	radios, err := ubus.GetWirelessStatus()
	if err != nil {
		return nil, err
	}
	var out []WifiUI
	for _, r := range radios {
		for _, iface := range r.Interfaces {
			u := WifiUI{
				Section:    iface.RFaceName,
				Radio:      r.Name,
				Ifname:     iface.Ifname,
				Band:       r.Band,
				SSID:       iface.SSID,
				Encryption: iface.Encryption,
				Hidden:     iface.Hidden,
				BSSID:      iface.BSSID,
				Disabled:   iface.Disabled,
				Clients:    iface.Clients,
			}
			u.HasKey = wifiHasKey(iface.RFaceName)
			u.MAC = wifiGetMAC(iface.RFaceName)
			out = append(out, u)
		}
	}
	return out, nil
}

// SetWifi applies a WifiEdit to a single AP interface with snapshot of
// wireless, wifi reload of the radio, a healthcheck that hostapd is still up
// and the SSID reflects the change, and rollback on failure.
func SetWifi(edit WifiEdit) (*WifiUI, bool, error) {
	if edit.Section == "" {
		return nil, false, fmt.Errorf("section is required")
	}
	snap, err := executor.Snapshot("wireless")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot wireless: %w", err)
	}
	radio := wirelessSectionDevice(edit.Section)
	rollback := func() {
		_ = executor.Restore("wireless", snap)
		if radio != "" {
			_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{radio}})
		}
	}

	ops, err := wifiEditOps(edit)
	if err != nil {
		return nil, false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return nil, true, err
	}
	if radio != "" {
		_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{radio}})
	}

	probe, perr := ProbeWifiUI()
	if perr != nil || !wifiHealthy(edit, probe) {
		rollback()
		return &WifiUI{}, true, fmt.Errorf("wifi healthcheck failed, rolled back")
	}
	if uciGet("wireless."+edit.Section+".disabled") == "1" {
		return wifiUIFIState(edit.Section), false, nil
	}
	return probeWifiSection(edit.Section), false, nil
}

// wifiUIFIState synthesises a WifiUI for a disabled (not enumerated) radio.
func wifiUIFIState(section string) *WifiUI {
	return &WifiUI{
		Section:    section,
		Radio:      wirelessSectionDevice(section),
		SSID:       uciGet("wireless." + section + ".ssid"),
		Encryption: uciGet("wireless." + section + ".encryption"),
		Hidden:     uciGet("wireless."+section+".hidden") == "1",
		Disabled:   true,
		HasKey:     uciGet("wireless."+section+".key") != "",
	}
}

func wifiHasKey(section string) bool {
	return uciGet("wireless."+section+".key") != ""
}

func wifiGetMAC(section string) string {
	return uciGet("wireless." + section + ".macaddr")
}

func wirelessSectionDevice(section string) string {
	return uciGet("wireless." + section + ".device")
}

func wifiEditOps(edit WifiEdit) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	del := func(key string) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{key}})
	}
	base := "wireless." + edit.Section

	if edit.SSID != "" {
		set(base+".ssid", edit.SSID)
	}
	if edit.Encryption != "" {
		if !wifiEncryptionValid(edit.Encryption) {
			return nil, fmt.Errorf("unknown Wi-Fi encryption: %q", edit.Encryption)
		}
		set(base+".encryption", edit.Encryption)
	}
	if edit.Key != "" {
		if len(edit.Key) < 8 || len(edit.Key) > 63 {
			return nil, fmt.Errorf("WPA key must be between 8 and 63 characters")
		}
		set(base+".key", edit.Key)
	}
	if edit.Hidden != nil {
		if *edit.Hidden {
			set(base+".hidden", "1")
		} else {
			del(base + ".hidden")
		}
	}
	if edit.Disabled != nil {
		if *edit.Disabled {
			set(base+".disabled", "1")
		} else {
			set(base+".disabled", "0")
		}
	}
	if edit.MAC != "" {
		if !wifiMACValid(edit.MAC) {
			return nil, fmt.Errorf("invalid MAC address: %q", edit.MAC)
		}
		set(base+".macaddr", edit.MAC)
	}

	if len(ops) == 0 {
		return nil, fmt.Errorf("nothing to change")
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	return ops, nil
}

func wifiEncryptionValid(e string) bool {
	switch e {
	case "psk", "psk2", "psk-mixed", "sae", "sae-mixed", "none", "open":
		return true
	}
	return false
}

func wifiMACValid(m string) bool {
	parts := strings.Split(m, ":")
	if len(parts) != 6 {
		return false
	}
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 16, 8)
		if err != nil || n > 0xff {
			return false
		}
	}
	return true
}

func wifiHealthy(edit WifiEdit, probe []WifiUI) bool {
	// A disabled radio drops out of the active interface list, so a
	// disable change must be validated against UCI, not the probe.
	if edit.Disabled != nil && *edit.Disabled {
		return uciGet("wireless."+edit.Section+".disabled") == "1"
	}
	for _, u := range probe {
		if u.Section != edit.Section {
			continue
		}
		if edit.SSID != "" && u.SSID != edit.SSID {
			return false
		}
		if edit.Hidden != nil && u.Hidden != *edit.Hidden {
			return false
		}
		if edit.Disabled != nil && u.Disabled != *edit.Disabled {
			return false
		}
		return true
	}
	return false
}

func probeWifiSection(section string) *WifiUI {
	ui, err := ProbeWifiUI()
	if err != nil {
		return &WifiUI{Section: section}
	}
	for _, u := range ui {
		if u.Section == section {
			return &u
		}
	}
	return &WifiUI{Section: section}
}

// WifiKey returns the PSK of a wifi-iface section. Exposed only via an
// authenticated endpoint (never included in ProbeWifiUI) so a cached
// /api/wifi response cannot leak it. Empty string means no key set.
func WifiKey(section string) string {
	if section == "" {
		return ""
	}
	return uciGet("wireless." + section + ".key")
}
