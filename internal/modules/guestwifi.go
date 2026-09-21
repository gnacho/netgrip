package modules

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

const (
	guestSection        = "netgrip_guest"
	guestZoneName       = "netgrip_guest"
	guestFwdName        = "netgrip_guest_wan"
	guestNetmask        = "255.255.255.0"
	guestMaxSubnetTries = 20
)

// GuestConfig is the user-provided guest WiFi configuration.
type GuestConfig struct {
	Enabled bool   `json:"enabled"`
	SSID    string `json:"ssid"`
	Key     string `json:"key"`
	Band    string `json:"band"`    // 2g | 5g | both
	Subnet  string `json:"subnet"`  // optional router IP of the guest network ("" = default)
	Isolate *bool  `json:"isolate"` // nil = leave unchanged, true/false = set
}

// GuestProbe is the read-only guest network state.
type GuestProbe struct {
	Gateway    bool     `json:"gateway"`
	Active     bool     `json:"active"`
	SSID       string   `json:"ssid"`
	Subnet     string   `json:"subnet"`
	Isolate    bool     `json:"isolate"`
	Ifaces     []string `json:"ifaces"`
	Clients    int      `json:"clients"`
	GLConflict bool     `json:"gl_conflict"`
}

// ProbeGuest reads the guest network state.
func ProbeGuest() *GuestProbe {
	p := &GuestProbe{
		Gateway: hasUplink() && executor.ServiceEnabled("firewall"),
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
	p.Isolate = uciGet("wireless."+guestSection+".isolate") == "1"
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
	InvalidateKey("guestwifi")
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

	// Disable: SSID off, infra kept for the next enable.
	if !cfg.Enabled {
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

	// Infra (subnet, DHCP, zone) is created on first enable and kept. The
	// subnet is only chosen at creation; changing it afterwards is out of scope.
	creating := !uciSectionExists("network.guest")
	if creating {
		if strings.TrimSpace(cfg.SSID) == "" {
			return nil, fmt.Errorf("ssid is required")
		}
		if len(cfg.Key) < 8 {
			return nil, fmt.Errorf("key must be at least 8 characters")
		}
		subnet, err := pickGuestSubnet(cfg.Subnet, existingGuestSubnets()...)
		if err != nil {
			return nil, err
		}
		set("network.guest", "interface")
		set("network.guest.proto", "static")
		set("network.guest.ipaddr", guestRouterIP(cfg.Subnet, subnet))
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
		set("firewall."+guestFwdName+".dest", internetZone())
	}

	if creating {
		devices := guestRadios(cfg.Band)
		if len(devices) == 0 {
			return nil, fmt.Errorf("no wireless radio found for band %q", cfg.Band)
		}
		isolate := true // secure default at creation
		if cfg.Isolate != nil {
			isolate = *cfg.Isolate
		}
		for i, device := range devices {
			section := guestSectionName(i)
			base := "wireless." + section
			set(base, "wifi-iface")
			set(base+".device", device)
			set(base+".mode", "ap")
			set(base+".ssid", cfg.SSID)
			set(base+".encryption", "psk2")
			set(base+".key", cfg.Key)
			set(base+".network", "guest")
			set(base+".isolate", isolateValue(isolate))
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

	// Update: network.guest already exists. Only apply what the caller set
	// (SSID/key change, re-enable, or an isolation toggle).
	sections := guestSections()
	if len(sections) == 0 {
		return nil, fmt.Errorf("guest wifi-iface not found")
	}
	changed := false
	if cfg.SSID != "" || cfg.Key != "" {
		if cfg.Key != "" && len(cfg.Key) < 8 {
			return nil, fmt.Errorf("key must be at least 8 characters")
		}
		for _, section := range sections {
			base := "wireless." + section
			if cfg.SSID != "" {
				set(base+".ssid", cfg.SSID)
			}
			if cfg.Key != "" {
				set(base+".encryption", "psk2")
				set(base+".key", cfg.Key)
			}
			set(base+".disabled", "0")
		}
		changed = true
	}
	if cfg.Isolate != nil {
		for _, section := range sections {
			set("wireless."+section+".isolate", isolateValue(*cfg.Isolate))
		}
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("nothing to change")
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	for _, device := range radiosForSections(sections) {
		ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{device}})
	}
	return ops, nil
}

// isolateValue encodes a bool as the UCI option value ("1"/"0").
func isolateValue(isolate bool) string {
	if isolate {
		return "1"
	}
	return "0"
}

// pickGuestSubnet chooses the /24 subnet for the guest network, returned as a
// CIDR string ("192.168.10.0/24"). requested is the router IP the user asked
// for ("" = default). existing holds the already-taken subnets as CIDR strings
// (LAN and any statically configured WAN). A requested subnet that collides is
// an error; the default silently advances to the next free /24.
func pickGuestSubnet(requested string, existing ...string) (string, error) {
	taken := map[string]bool{}
	for _, cidr := range existing {
		if n := normalizeCIDR(cidr); n != "" {
			taken[n] = true
		}
	}

	if requested == "" {
		for i := 0; i < guestMaxSubnetTries; i++ {
			cidr := fmt.Sprintf("192.168.%d.0/24", 10+i)
			if !taken[cidr] {
				return cidr, nil
			}
		}
		return "", fmt.Errorf("no free /24 subnet found for the guest network")
	}

	ip := net.ParseIP(strings.TrimSpace(requested)).To4()
	if ip == nil {
		return "", fmt.Errorf("guest subnet %q is not a valid IPv4 address", requested)
	}
	if !ip.IsPrivate() {
		return "", fmt.Errorf("guest subnet %q is not a private IPv4 address", requested)
	}
	if ip[3] == 0 || ip[3] == 255 {
		return "", fmt.Errorf("guest subnet %q is a network or broadcast address", requested)
	}
	cidr := fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2])
	if taken[cidr] {
		return "", fmt.Errorf("guest subnet %s overlaps the LAN or WAN", cidr)
	}
	return cidr, nil
}

// normalizeCIDR canonicalizes a CIDR string into "a.b.c.d/nn", or "" when it
// cannot be parsed. It accepts the forms produced by subnetCIDR.
func normalizeCIDR(cidr string) string {
	_, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return ""
	}
	ip := ipnet.IP.To4()
	if ip == nil {
		return ""
	}
	ones, _ := ipnet.Mask.Size()
	return fmt.Sprintf("%s/%d", ip.String(), ones)
}

// subnetCIDR converts an ipaddr + netmask pair into a canonical CIDR string,
// or "" when the pair is empty or invalid.
func subnetCIDR(ipaddr, netmask string) string {
	ip := net.ParseIP(ipaddr).To4()
	mask := net.IPMask(net.ParseIP(netmask).To4())
	if ip == nil || len(mask) != 4 {
		return ""
	}
	ones, bits := mask.Size()
	if bits == 0 || ones == 0 {
		return ""
	}
	return fmt.Sprintf("%s/%d", ip.Mask(mask).String(), ones)
}

// existingGuestSubnets returns the CIDR strings of the subnets the guest
// network must not collide with: the LAN and a statically configured WAN.
func existingGuestSubnets() []string {
	var subs []string
	if c := subnetCIDR(uciGet("network.lan.ipaddr"), uciGet("network.lan.netmask")); c != "" {
		subs = append(subs, c)
	}
	if wan := uplinkNetwork(); wan != "" {
		if c := subnetCIDR(uciGet("network."+wan+".ipaddr"), uciGet("network."+wan+".netmask")); c != "" {
			subs = append(subs, c)
		}
	}
	return subs
}

// guestRouterIP returns the router IP to assign to the guest interface: the
// requested host when given, otherwise the first host of the chosen subnet.
func guestRouterIP(requested, subnet string) string {
	if requested != "" {
		return requested
	}
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return ""
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return ""
	}
	ip4[3] = 1
	return ip4.String()
}

// guestSectionName returns the wifi-iface section name for radio index i.
// The first radio always uses the base name because ProbeGuest and
// ProbeGuestBand read wireless.<guestSection> directly; suffixing every
// section when band=both left the base section missing and the post-apply
// healthcheck rolled the whole change back (#315).
func guestSectionName(i int) string {
	if i == 0 {
		return guestSection
	}
	return fmt.Sprintf("%s_%d", guestSection, i)
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
