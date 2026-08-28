package modules

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

const (
	guestSection  = "netgrip_guest"
	guestZoneName = "netgrip_guest"
	guestFwdName  = "netgrip_guest_wan"
	guestSubnetIP = "192.168.9.1"
	guestNetmask  = "255.255.255.0"
)

// GuestConfig is the user-provided guest WiFi configuration.
type GuestConfig struct {
	Enabled bool   `json:"enabled"`
	SSID    string `json:"ssid"`
	Key     string `json:"key"`
	Band    string `json:"band"` // 2g | 5g | both
}

// GuestProbe is the read-only guest network state.
type GuestProbe struct {
	Gateway    bool     `json:"gateway"`
	Active     bool     `json:"active"`
	SSID       string   `json:"ssid"`
	Subnet     string   `json:"subnet"`
	Ifaces     []string `json:"ifaces"`
	Clients    int      `json:"clients"`
	GLConflict bool     `json:"gl_conflict"`
}

// ProbeGuest reads the guest network state.
func ProbeGuest() *GuestProbe {
	p := &GuestProbe{
		Gateway: uciSectionExists("network.wan") && executor.ServiceEnabled("firewall"),
		Ifaces:  []string{},
	}
	if !p.Gateway {
		return p
	}
	// GL firmware guest wifi uses guest2g/guest5g sections: warn about
	// coexistence instead of silently sharing the band.
	if uciSectionExists("wireless.guest2g") || uciSectionExists("wireless.guest5g") {
		p.GLConflict = true
	}
	if !uciSectionExists("wireless." + guestSection) {
		return p
	}
	p.SSID = uciGet("wireless." + guestSection + ".ssid")
	p.Active = uciGet("wireless."+guestSection+".disabled") != "1"
	p.Subnet = uciGet("network.guest.ipaddr")
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

// SetGuest applies the guest WiFi configuration with snapshots and rollback.
func SetGuest(cfg GuestConfig) (*GuestProbe, bool, error) {
	probe := ProbeGuest()
	if !probe.Gateway {
		return probe, false, fmt.Errorf("guest WiFi needs a gateway (WAN + firewall); not possible on a dumb AP")
	}
	snaps := map[string]string{}
	for _, cfgName := range []string{"network", "dhcp", "firewall", "wireless"} {
		s, err := executor.Snapshot(cfgName)
		if err != nil {
			return probe, false, fmt.Errorf("snapshot %s: %w", cfgName, err)
		}
		snaps[cfgName] = s
	}
	rollback := func() {
		for cfgName, snap := range snaps {
			_ = executor.Restore(cfgName, snap)
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
		_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{}})
	}

	ops, err := guestOps(cfg)
	if err != nil {
		return probe, false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeGuest(), true, err
	}

	ok := func() bool {
		for range 75 {
			p := ProbeGuest()
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
		return ProbeGuest(), true, fmt.Errorf("healthcheck failed after apply (enabled=%v), rolled back", cfg.Enabled)
	}
	return ProbeGuest(), false, nil
}

func guestOps(cfg GuestConfig) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}

	// Infra (subnet, DHCP, zone) is created on first enable and kept.
	if cfg.Enabled {
		if strings.TrimSpace(cfg.SSID) == "" {
			return nil, fmt.Errorf("ssid is required")
		}
		if len(cfg.Key) < 8 {
			return nil, fmt.Errorf("key must be at least 8 characters")
		}
		if !uciSectionExists("network.guest") {
			set("network.guest", "interface")
			set("network.guest.proto", "static")
			set("network.guest.ipaddr", guestSubnetIP)
			set("network.guest.netmask", guestNetmask)
			set("dhcp.guest", "dhcp")
			set("dhcp.guest.interface", "guest")
			set("dhcp.guest.start", "100")
			set("dhcp.guest.limit", "150")
			set("dhcp.guest.leasetime", "12h")
			// Zone: clients may reach the router (DHCP/DNS) and the
			// internet, but no forwarding to lan exists, so no LAN access.
			set("firewall."+guestZoneName, "zone")
			set("firewall."+guestZoneName+".name", "netgrip-guest")
			set("firewall."+guestZoneName+".input", "ACCEPT")
			set("firewall."+guestZoneName+".output", "ACCEPT")
			set("firewall."+guestZoneName+".forward", "REJECT")
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"firewall." + guestZoneName + ".network", "guest"}})
			set("firewall."+guestFwdName, "forwarding")
			set("firewall."+guestFwdName+".src", "guest")
			set("firewall."+guestFwdName+".dest", "wan")
		}

		devices := guestRadios(cfg.Band)
		if len(devices) == 0 {
			return nil, fmt.Errorf("no wireless radio found for band %q", cfg.Band)
		}
		for i, device := range devices {
			section := guestSection
			if len(devices) > 1 {
				section = fmt.Sprintf("%s_%d", guestSection, i)
			}
			base := "wireless." + section
			set(base, "wifi-iface")
			set(base+".device", device)
			set(base+".mode", "ap")
			set(base+".ssid", cfg.SSID)
			set(base+".encryption", "psk2")
			set(base+".key", cfg.Key)
			set(base+".network", "guest")
			set(base+".isolate", "1")
			set(base+".disabled", "0")
		}
		ops = append(ops,
			executor.Op{Kind: "uci_commit", Args: []string{"network"}},
			executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
			executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
			executor.Op{Kind: "uci_commit", Args: []string{"wireless"}},
			executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
			executor.Op{Kind: "initd", Args: []string{"dnsmasq", "restart"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
		)
		for _, device := range devices {
			ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{device}})
		}
		return ops, nil
	}

	// Disable: SSID off, infra kept for the next enable.
	for _, section := range guestSections() {
		set("wireless."+section+".disabled", "1")
	}
	if len(guestSections()) > 0 {
		ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
		for _, device := range guestRadios(ProbeGuestBand()) {
			ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{device}})
		}
	}
	return ops, nil
}

func guestSections() []string {
	out, err := exec.Command("sh", "-c", "uci show wireless | grep '=wifi-iface' | cut -d. -f2 | cut -d= -f1 | grep '^"+guestSection+"'").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(out))
}

func guestRadios(band string) []string {
	if band == "2g" || band == "5g" {
		if r := radioForBand(band); r != "" {
			return []string{r}
		}
		return []string{}
	}
	radios, _ := ubusRadios()
	var devices []string
	for _, r := range radios {
		devices = append(devices, r.Name)
	}
	return devices
}

// ProbeGuestBand resolves the band of the configured guest SSID.
func ProbeGuestBand() string {
	device := uciGet("wireless." + guestSection + ".device")
	radios, _ := ubusRadios()
	for _, r := range radios {
		if r.Name == device {
			return r.Band
		}
	}
	return ""
}
