package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// WifiEdit is the user-provided change to one AP interface (a "radio"'s
// principal network). Empty fields are left unchanged. Key is write-only:
// it is never read back. Sections optionally lists extra wifi-iface sections
// to apply the same change to in one transaction (band steering: one network
// across radios); Section stays the primary section.
type WifiEdit struct {
	Section    string   `json:"section"` // UCI section, e.g. default_radio0
	Sections   []string `json:"sections,omitempty"`
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
	seen := make(map[string]bool)
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
			seen[iface.RFaceName] = true
			out = append(out, u)
		}
	}
	// Disabled interfaces drop out of ubus; report the configured ones anyway
	// so the panel can turn them back on (turning every radio off used to
	// leave the Wi-Fi page with nothing to edit).
	for _, sec := range missingWifiSections(seen, wifiIfaceSections()) {
		out = append(out, *wifiUIFIState(sec))
	}
	return out, nil
}

// missingWifiSections returns the configured wifi-iface sections that the
// probe does not list yet, preserving the configured order.
func missingWifiSections(seen map[string]bool, configured []string) []string {
	var out []string
	for _, sec := range configured {
		if sec == "" || seen[sec] {
			continue
		}
		out = append(out, sec)
	}
	return out
}

// SetWifi applies a WifiEdit to one AP interface, or to several when
// edit.Sections is set (band steering), with a single snapshot of wireless,
// a reload of every affected radio, a healthcheck per section, and rollback
// on failure.
func SetWifi(edit WifiEdit) (*WifiUI, bool, error) {
	InvalidateKey("wireless")
	InvalidateKey("ucishow|wireless")
	if edit.Section == "" {
		return nil, false, fmt.Errorf("section is required")
	}
	sections := wifiTargetSections(edit)
	for _, sec := range sections {
		if !uciSectionExists("wireless." + sec) {
			return nil, false, fmt.Errorf("unknown wireless section: %q", sec)
		}
	}
	snap, err := executor.Snapshot("wireless")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot wireless: %w", err)
	}
	reloadAll := func() {
		for _, sec := range sections {
			if r := wirelessSectionDevice(sec); r != "" {
				_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{r}})
			}
		}
	}
	rollback := func() {
		_ = executor.Restore("wireless", snap)
		reloadAll()
	}

	var ops []executor.Op
	for _, sec := range sections {
		o, err := wifiEditOpsFor(sec, edit)
		if err != nil {
			return nil, false, err
		}
		ops = append(ops, o...)
	}
	if len(ops) == 0 {
		return nil, false, fmt.Errorf("nothing to change")
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return nil, true, err
	}
	reloadAll()

	probe, perr := ProbeWifiUI()
	if perr != nil {
		rollback()
		return &WifiUI{}, true, fmt.Errorf("wifi healthcheck failed, rolled back")
	}
	for _, sec := range sections {
		e := edit
		e.Section = sec
		if !wifiHealthy(e, probe) {
			rollback()
			return &WifiUI{}, true, fmt.Errorf("wifi healthcheck failed, rolled back")
		}
	}
	if uciGet("wireless."+edit.Section+".disabled") == "1" {
		return wifiUIFIState(edit.Section), false, nil
	}
	return probeWifiSection(edit.Section), false, nil
}

// wifiTargetSections returns the wifi-iface sections an edit applies to: the
// explicit Sections list when given (deduplicated, primary first), else just
// the primary Section.
func wifiTargetSections(edit WifiEdit) []string {
	out := []string{edit.Section}
	seen := map[string]bool{edit.Section: true}
	for _, s := range edit.Sections {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// SetWifiRadio applies a RadioEdit (channel/txpower/htmode) to a wireless
// device with a snapshot of wireless, a reload of that radio, a healthcheck
// that the radio stays up on the requested channel, and rollback on failure.
func SetWifiRadio(edit RadioEdit) (*ubus.WirelessRadio, bool, error) {
	InvalidateKey("wireless")
	InvalidateKey("ucishow|wireless")
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
		// A full bounce, not a reload: reload asks the running hostapd to
		// reconfigure, and a hostapd that died (unsupported channel) has
		// nobody to answer, leaving the radio dark (#452).
		_ = executor.Run(executor.Op{Kind: "wifi_bounce", Args: []string{edit.Radio}})
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

	radio, perr := waitRadioHealthy(edit)
	if perr != nil {
		rollback()
		return &ubus.WirelessRadio{}, true, fmt.Errorf("radio healthcheck failed, rolled back")
	}
	return radio, false, nil
}

// hostapdServing verifies every interface of the radio reports a real
// channel through iwinfo. It returns verified=false when there is nothing to
// check (no interfaces), so unusual setups never trigger a false rollback.
func hostapdServing(radio *ubus.WirelessRadio) (serving bool, verified bool) {
	if radio == nil {
		return true, false
	}
	if len(radio.Interfaces) == 0 {
		// The radio reports up but netifd created no interfaces: hostapd
		// is either still starting or dead (#452). Count it as verified-
		// not-serving so the polling window keeps waiting; a genuinely
		// dead hostapd exhausts it and rolls back.
		return false, true
	}
	for _, iface := range radio.Interfaces {
		if iface.Ifname == "" {
			continue
		}
		out, err := exec.Command("iwinfo", iface.Ifname, "info").Output()
		if err != nil {
			// The interface exists per ubus but iwinfo cannot see it:
			// hostapd never created it.
			return false, true
		}
		if ch, ok := iwinfoChannel(string(out)); !ok || ch <= 0 {
			return false, true
		}
	}
	return true, true
}

// iwinfoChannel extracts the channel number from `iwinfo <if> info` output.
// The channel appears as a field anywhere ("Channel: 36 (5.180 GHz)" on its
// own line or after "Mode: Master", depending on the iwinfo version). ok is
// false when no channel field is present.
func iwinfoChannel(info string) (int, bool) {
	fields := strings.Fields(info)
	for i, f := range fields {
		if f != "Channel:" || i+1 >= len(fields) {
			continue
		}
		ch, err := strconv.Atoi(fields[i+1])
		if err != nil {
			return 0, false
		}
		return ch, true
	}
	return 0, false
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

// waitRadioHealthy polls the radio up to ~16s after a reload: the interface
// needs a moment to come back up, so an immediate read could wrongly report a
// change as a failure (false rollback). Only after the window is it fatal.
//
// #452: ubus can report the radio up with the requested channel while
// hostapd actually failed to start (a channel the board lists but cannot
// serve); iwinfo then shows "Channel: 0 unknown" and the SSID never comes
// up. So each iteration also waits for hostapd to serve: a transient
// mid-restart read is given more window, a genuinely dead hostapd exhausts
// it and fails.
func waitRadioHealthy(edit RadioEdit) (*ubus.WirelessRadio, error) {
	var last *ubus.WirelessRadio
	for i := 0; i < 8; i++ {
		if r, err := radioFromStatus(edit.Radio); err == nil {
			last = r
			if radioHealthy(edit, r) {
				if serving, verified := hostapdServing(r); !verified || serving {
					return r, nil
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	return last, fmt.Errorf("radio %q did not stabilise", edit.Radio)
}

// wifiUIFIState synthesises a WifiUI for a configured wifi-iface that ubus
// does not enumerate (typically a disabled one), so the panel can still show
// it and turn it back on.
func wifiUIFIState(section string) *WifiUI {
	device := wirelessSectionDevice(section)
	return &WifiUI{
		Section:    section,
		Radio:      device,
		Band:       uciGet("wireless." + device + ".band"),
		SSID:       uciGet("wireless." + section + ".ssid"),
		Encryption: uciGet("wireless." + section + ".encryption"),
		Hidden:     uciGet("wireless."+section+".hidden") == "1",
		Disabled:   uciGet("wireless."+section+".disabled") == "1",
		HasKey:     uciGet("wireless."+section+".key") != "",
		Clients:    []ubus.WirelessClient{},
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

func wifiEditOpsFor(section string, edit WifiEdit) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	del := func(key string) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{key}})
	}
	base := "wireless." + section

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
