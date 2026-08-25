package modules

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gnacho/owpanel/internal/executor"
)

// LANConfig is the read-only LAN + DHCP + reservation state.
type LANConfig struct {
	Applicable   bool          `json:"applicable"`
	IpAddr       string        `json:"ipaddr"`
	Netmask      string        `json:"netmask"`
	ApIsolation  bool          `json:"ap_isolation"`
	Dhcp         DHCPConfig    `json:"dhcp"`
	Reservations []Reservation `json:"reservations"`
}

// DHCPConfig is the per-LAN DHCP server settings.
type DHCPConfig struct {
	Enabled   bool   `json:"enabled"`
	Start     int    `json:"start"`
	Limit     int    `json:"limit"`
	LeaseTime int    `json:"lease_time"` // minutes
	Gateway   string `json:"gateway,omitempty"`
	DNS1      string `json:"dns1,omitempty"`
	DNS2      string `json:"dns2,omitempty"`
}

// Reservation is one static DHCP lease (MAC -> IP).
type Reservation struct {
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
	Name string `json:"name,omitempty"`
}

func lanApplicable() bool {
	if !executor.ServiceEnabled("dnsmasq") {
		return false
	}
	return uciGet("network.lan.proto") == "static"
}

// ProbeLAN reads the LAN, DHCP and reservation state.
func ProbeLAN() *LANConfig {
	c := &LANConfig{
		Applicable:   lanApplicable(),
		IpAddr:       uciGet("network.lan.ipaddr"),
		Netmask:      uciGet("network.lan.netmask"),
		Dhcp:         probeDHCP(),
		Reservations: probeReservations(),
	}
	c.ApIsolation = probeApIsolation()
	return c
}

func probeDHCP() DHCPConfig {
	d := DHCPConfig{
		Enabled:   uciGet("dhcp.lan.dhcpv4") == "server",
		LeaseTime: dhcpLeaseMinutes(),
	}
	if v, err := strconv.Atoi(uciGet("dhcp.lan.start")); err == nil {
		d.Start = v
	}
	if v, err := strconv.Atoi(uciGet("dhcp.lan.limit")); err == nil {
		d.Limit = v
	}
	d.DNS1, d.DNS2, d.Gateway = dhcpOptionServers()
	return d
}

func probeApIsolation() bool {
	for _, section := range wifiIfaceSections() {
		if uciGet("wireless."+section+".isolate") == "1" {
			return true
		}
	}
	return false
}

func probeReservations() []Reservation {
	var list []Reservation
	for _, section := range hostSections() {
		mac := strings.ToLower(uciGet("dhcp." + section + ".mac"))
		ip := uciGet("dhcp." + section + ".ip")
		if mac == "" {
			continue
		}
		list = append(list, Reservation{MAC: mac, IP: ip, Name: uciGet("dhcp." + section + ".name")})
	}
	if list == nil {
		return []Reservation{}
	}
	return list
}

func dhcpLeaseMinutes() int {
	lt := uciGet("dhcp.lan.leasetime")
	for _, unit := range []struct {
		pfx  string
		mult int
	}{
		{"m", 1}, {"h", 60}, {"d", 1440}, {"w", 10080},
	} {
		if strings.HasSuffix(lt, unit.pfx) {
			if n, err := strconv.Atoi(strings.TrimSuffix(lt, unit.pfx)); err == nil {
				return n * unit.mult
			}
		}
	}
	if n, err := strconv.Atoi(lt); err == nil {
		return n
	}
	return 720
}

// dhcpOptionServers extracts DHCP options 3 (gateway) and 6 (DNS) from
// dhcp.lan.dhcp_option, encoded as the GL portal does (e.g. "3,192.168.1.1").
func dhcpOptionServers() (dns1, dns2, gw string) {
	out, err := exec.Command("sh", "-c", "uci -q get dhcp.lan.dhcp_option").Output()
	if err != nil {
		return "", "", ""
	}
	for _, opt := range strings.Fields(string(out)) {
		parts := strings.Split(opt, ",")
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "3":
			gw = parts[1]
		case "6":
			if dns1 == "" {
				dns1 = parts[1]
			} else {
				dns2 = parts[1]
			}
		}
	}
	return dns1, dns2, gw
}

// SetLAN applies ipaddr/netmask and/or AP isolation (gateway only).
func SetLAN(cfg struct {
	IpAddr      *string `json:"ipaddr,omitempty"`
	Netmask     *string `json:"netmask,omitempty"`
	ApIsolation *bool   `json:"ap_isolation,omitempty"`
}) (*LANConfig, bool, error) {
	if !lanApplicable() {
		return ProbeLAN(), false, fmt.Errorf("LAN settings only apply on the gateway (static IP + dnsmasq)")
	}
	snapNet, err := executor.Snapshot("network")
	if err != nil {
		return ProbeLAN(), false, fmt.Errorf("snapshot network: %w", err)
	}
	snapWifi, _ := executor.Snapshot("wireless")
	rollback := func() {
		_ = executor.Restore("network", snapNet)
		if snapWifi != "" {
			_ = executor.Restore("wireless", snapWifi)
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	}

	var ops []executor.Op
	changed := false
	if cfg.IpAddr != nil && *cfg.IpAddr != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network.lan.ipaddr", *cfg.IpAddr}})
		changed = true
	}
	if cfg.Netmask != nil && *cfg.Netmask != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network.lan.netmask", *cfg.Netmask}})
		changed = true
	}
	if changed {
		ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})
	}
	if cfg.ApIsolation != nil {
		ops = append(ops, apIsolationOps(*cfg.ApIsolation)...)
		changed = true
	}
	if !changed {
		return ProbeLAN(), false, fmt.Errorf("nothing to change")
	}

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeLAN(), true, err
	}
	if changed {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	}
	return ProbeLAN(), false, nil
}

func apIsolationOps(enabled bool) []executor.Op {
	var ops []executor.Op
	val := "0"
	if enabled {
		val = "1"
	}
	for _, section := range wifiIfaceSections() {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"wireless." + section + ".isolate", val}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	for _, radio := range radiosForSections(wifiIfaceSections()) {
		ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{radio}})
	}
	return ops
}

// SetDHCP applies the DHCP server settings (gateway only).
func SetDHCP(cfg DHCPConfig) (*LANConfig, bool, error) {
	if !lanApplicable() {
		return ProbeLAN(), false, fmt.Errorf("DHCP settings only apply on the gateway (static IP + dnsmasq)")
	}
	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return ProbeLAN(), false, err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}

	if cfg.Start < 0 || cfg.Start > 254 || cfg.Limit < 0 || cfg.Limit > 255 || cfg.Start+cfg.Limit > 255 {
		return ProbeLAN(), false, fmt.Errorf("invalid DHCP range (start 0-254, limit, start+limit <= 255)")
	}
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"dhcp.lan.dhcpv4", dhcpv4State(cfg.Enabled)}},
		{Kind: "uci_set", Args: []string{"dhcp.lan.start", strconv.Itoa(cfg.Start)}},
		{Kind: "uci_set", Args: []string{"dhcp.lan.limit", strconv.Itoa(cfg.Limit)}},
		{Kind: "uci_set", Args: []string{"dhcp.lan.leasetime", dhcpLeaseString(cfg.LeaseTime)}},
	}
	ops = append(ops, dhcpServerOptions(cfg)...)
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeLAN(), true, err
	}
	return ProbeLAN(), false, nil
}

func dhcpv4State(enabled bool) string {
	if enabled {
		return "server"
	}
	return "disabled"
}

func dhcpLeaseString(minutes int) string {
	if minutes > 0 && minutes%1440 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}

func dhcpServerOptions(cfg DHCPConfig) []executor.Op {
	var otps []string
	if cfg.Gateway != "" {
		otps = append(otps, "3,"+cfg.Gateway)
	}
	if cfg.DNS1 != "" {
		otps = append(otps, "6,"+cfg.DNS1)
	}
	if cfg.DNS2 != "" {
		otps = append(otps, "6,"+cfg.DNS2)
	}
	var ops []executor.Op
	if len(otps) > 0 {
		for _, o := range otps {
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"dhcp.lan.dhcp_option", o}})
		}
	} else {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"dhcp.lan.dhcp_option"}})
	}
	return ops
}

// SetReservation adds, edits or removes a static DHCP reservation.
func SetReservation(mac, ip, name string, reserved bool) (*LANConfig, bool, error) {
	if !lanApplicable() {
		return ProbeLAN(), false, fmt.Errorf("reservations only apply on the gateway (static IP + dnsmasq)")
	}
	mac = strings.ToLower(mac)
	if !reMac.MatchString(mac) || (reserved && !reIPv4.MatchString(ip)) {
		return ProbeLAN(), false, fmt.Errorf("invalid mac/ip")
	}
	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return ProbeLAN(), false, err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}
	section := "owpanel_host_" + strings.ReplaceAll(mac, ":", "")
	base := "dhcp." + section
	var ops []executor.Op
	if reserved {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{base, "host"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".mac", mac}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".ip", ip}},
		)
		if name != "" {
			ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".name", name}})
		}
	} else if uciSectionExists(base) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{base}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeLAN(), true, err
	}
	return ProbeLAN(), false, nil
}

// ClearReservations removes every owpanel_* reservation (gateway only).
func ClearReservations() (*LANConfig, bool, error) {
	if !lanApplicable() {
		return ProbeLAN(), false, fmt.Errorf("reservations only apply on the gateway (static IP + dnsmasq)")
	}
	sections := hostSections()
	if len(sections) == 0 {
		return ProbeLAN(), false, nil
	}
	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return ProbeLAN(), false, err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}
	var ops []executor.Op
	for _, s := range sections {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"dhcp." + s}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeLAN(), true, err
	}
	return ProbeLAN(), false, nil
}
