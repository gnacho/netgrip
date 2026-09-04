package modules

import (
	"fmt"
	"regexp"
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

// RadioEdit is a user change to a radio device (not an AP interface). Empty
// fields are left unchanged. TxPower of 0 leaves the current value untouched.
type RadioEdit struct {
	Radio   string `json:"radio"`             // UCI device, e.g. radio0
	Channel string `json:"channel,omitempty"` // e.g. "1", "36", or "auto"
	Htmode  string `json:"htmode,omitempty"`  // e.g. HE20, HE80, VHT40
	TxPower int    `json:"txpower,omitempty"` // dBm; 0 = keep current
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

// SetWifiRadio applies a RadioEdit (channel/txpower/htmode) to a wireless
// device with a snapshot of wireless, a reload of that radio, a healthcheck
// that the radio stays up on the requested channel, and rollback on failure.
func SetWifiRadio(edit RadioEdit) (*ubus.WirelessRadio, bool, error) {
	if edit.Radio == "" {
		return nil, false, fmt.Errorf("radio is required")
	}
	if err := validateRadioEdit(edit); err != nil {
		return nil, false, err
	}
	snap, err := executor.Snapshot("wireless")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot wireless: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("wireless", snap)
		_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{edit.Radio}})
	}

	var ops []executor.Op
	if edit.Channel != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"wireless." + edit.Radio + ".channel", edit.Channel}})
	}
	if edit.Htmode != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"wireless." + edit.Radio + ".htmode", edit.Htmode}})
	}
	if edit.TxPower > 0 {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"wireless." + edit.Radio + ".txpower", strconv.Itoa(edit.TxPower)}})
	}
	if len(ops) == 0 {
		return nil, false, fmt.Errorf("nothing to change")
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return nil, true, err
	}
	_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{edit.Radio}})

	radio, perr := radioFromStatus(edit.Radio)
	if perr != nil || !radioHealthy(edit, radio) {
		rollback()
		return &ubus.WirelessRadio{}, true, fmt.Errorf("radio healthcheck failed, rolled back")
	}
	return radio, false, nil
}

// validateRadioEdit checks the radio is set, something is changed, the
// channel numeric/auto and the htmode token (HE/VHT/HT + 20/40/80/160),
// keeping it permissive across bands.
func validateRadioEdit(edit RadioEdit) error {
	if edit.Radio == "" {
		return fmt.Errorf("radio is required")
	}
	if edit.Channel == "" && edit.Htmode == "" && edit.TxPower == 0 {
		return fmt.Errorf("nothing to change")
	}
	if edit.Channel != "" && edit.Channel != "auto" {
		if _, err := strconv.Atoi(edit.Channel); err != nil {
			return fmt.Errorf("invalid channel: %q", edit.Channel)
		}
	}
	if edit.Htmode != "" {
		re := regexp.MustCompile(`^(HE|VHT|HT)(20|40|80|160)$`)
		if !re.MatchString(edit.Htmode) {
			return fmt.Errorf("unknown htmode: %q", edit.Htmode)
		}
	}
	return nil
}

// radioHealthy verifies the radio is still up and, when a channel was
// requested, that it actually landed on it.
func radioHealthy(edit RadioEdit, radio *ubus.WirelessRadio) bool {
	if radio == nil || !radio.Up {
		return false
	}
	if edit.Channel != "" && edit.Channel != "auto" && radio.Channel != edit.Channel {
		return false
	}
	return true
}

func radioFromStatus(name string) (*ubus.WirelessRadio, error) {
	radios, err := ubus.GetWirelessStatus()
	if err != nil {
		return nil, err
	}
	for i := range radios {
		if radios[i].Name == name {
			return &radios[i], nil
		}
	}
	return nil, fmt.Errorf("radio %q not found", name)
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
