package modules

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gnacho/owpanel/internal/executor"
	"github.com/gnacho/owpanel/internal/ubus"
)

const iotSection = "owpanel_iot"

// IoTConfig is the user-provided IoT SSID configuration.
type IoTConfig struct {
	Enabled bool   `json:"enabled"`
	SSID    string `json:"ssid"`
	Key     string `json:"key"`
	Band    string `json:"band"` // 2g | 5g | both
}

// IoTProbe is the read-only IoT SSID state.
type IoTProbe struct {
	Active   bool     `json:"active"`
	SSID     string   `json:"ssid"`
	Band     string   `json:"band"`
	Isolated bool     `json:"isolated"`
	Ifaces   []string `json:"ifaces"`
	Clients  int      `json:"clients"`
}

// radioForBand resolves the wireless device name for a band ("radio0" etc.)
// by scanning the wireless status (radio names and band assignments differ
// across models: on the AX6 radio0=5g/radio1=2g, on the Flint2 the reverse).
func radioForBand(band string) string {
	radios, err := ubusRadios()
	if err != nil {
		return ""
	}
	for _, r := range radios {
		if r.Band == band {
			return r.Name
		}
	}
	return ""
}

func ubusRadios() ([]ubus.WirelessRadio, error) {
	return ubus.GetWirelessStatus()
}

func iotSections() []string {
	out, err := exec.Command("sh", "-c", "uci show wireless | grep '=wifi-iface' | cut -d. -f2 | cut -d= -f1 | grep '^"+iotSection+"'").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(out))
}

// ProbeIoT reads the IoT SSID state.
func ProbeIoT() *IoTProbe {
	p := &IoTProbe{Ifaces: []string{}}
	sections := iotSections()
	if len(sections) == 0 {
		return p
	}
	first := sections[0]
	p.SSID = uciGet("wireless." + first + ".ssid")
	p.Isolated = uciGet("wireless."+first+".isolate") == "1"
	p.Active = uciGet("wireless."+first+".disabled") != "1"
	device := uciGet("wireless." + first + ".device")
	switch {
	case len(sections) > 1:
		p.Band = "both"
	default:
		radios, _ := ubusRadios()
		for _, r := range radios {
			if r.Name == device {
				p.Band = r.Band
			}
		}
	}
	// Live interfaces and client count from the wireless status.
	// Freshly reloaded ifaces can appear with an empty ifname for a
	// moment: skip those (they are not broadcasting yet).
	radios, _ := ubusRadios()
	for _, r := range radios {
		for _, iface := range r.Interfaces {
			if iface.Ifname == "" {
				continue
			}
			if iface.SSID == p.SSID && p.SSID != "" {
				p.Ifaces = append(p.Ifaces, iface.Ifname)
				p.Clients += len(iface.Clients)
			}
		}
	}
	return p
}

// SetIoT applies the IoT SSID configuration with snapshot, per-radio reload
// and rollback.
func SetIoT(cfg IoTConfig) (*IoTProbe, bool, error) {
	snapWireless, err := executor.Snapshot("wireless")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot wireless: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("wireless", snapWireless)
		for _, radio := range radiosToReload(cfg.Band) {
			_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{radio}})
		}
	}

	ops, err := iotOps(cfg)
	if err != nil {
		return ProbeIoT(), false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeIoT(), true, err
	}

	ok := func() bool {
		for range 75 {
			p := ProbeIoT()
			if cfg.Enabled {
				if p.Active && len(p.Ifaces) > 0 {
					return true
				}
			} else if !p.Active {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	if !ok() {
		rollback()
		return ProbeIoT(), true, fmt.Errorf("healthcheck failed after apply (enabled=%v), rolled back", cfg.Enabled)
	}
	return ProbeIoT(), false, nil
}

func radiosToReload(band string) []string {
	switch band {
	case "2g", "5g":
		if r := radioForBand(band); r != "" {
			return []string{r}
		}
		return []string{}
	default:
		return nil // all radios
	}
}

func iotOps(cfg IoTConfig) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}

	if !cfg.Enabled {
		for _, section := range iotSections() {
			set("wireless."+section+".disabled", "1")
		}
		if len(iotSections()) > 0 {
			ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
			for _, radio := range radiosToReload(ProbeIoT().Band) {
				ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{radio}})
			}
			if len(radiosToReload(ProbeIoT().Band)) == 0 {
				ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{}})
			}
		}
		return ops, nil
	}

	if strings.TrimSpace(cfg.SSID) == "" {
		return nil, fmt.Errorf("ssid is required")
	}
	if len(cfg.Key) < 8 {
		return nil, fmt.Errorf("key must be at least 8 characters")
	}

	devices := []string{}
	for _, radio := range radiosToReload(cfg.Band) {
		devices = append(devices, radio)
	}
	if len(devices) == 0 {
		if cfg.Band == "both" || cfg.Band == "" {
			radios, _ := ubusRadios()
			for _, r := range radios {
				devices = append(devices, r.Name)
			}
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no wireless radio found for band %q", cfg.Band)
	}

	for i, device := range devices {
		section := iotSection
		if len(devices) > 1 {
			section = fmt.Sprintf("%s_%d", iotSection, i)
		}
		base := "wireless." + section
		set(base, "wifi-iface")
		set(base+".device", device)
		set(base+".mode", "ap")
		set(base+".ssid", cfg.SSID)
		set(base+".encryption", "psk2")
		set(base+".key", cfg.Key)
		set(base+".network", "lan")
		// Client isolation: stations cannot talk to each other, exactly
		// what an IoT SSID wants while staying reachable from the LAN.
		set(base+".isolate", "1")
		set(base+".disabled", "0")
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	for _, device := range devices {
		ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{device}})
	}
	return ops, nil
}
