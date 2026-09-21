package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

var reHostname = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?$`)

// DNSConfig is the read-only DNS state.
type DNSConfig struct {
	Applicable    bool `json:"applicable"`
	RebindProtect bool `json:"rebind_protection"`
	OverrideDNS   bool `json:"override_dns"`
	DnsVpn        bool `json:"dns_vpn_local"`
	ForceDNS      bool `json:"force_dns"`
	AdGuardActive bool `json:"adguard_active"`
	// AdGuardInstalled/AdGuardRunning describe the adguardhome package and
	// its init.d service (lowercase "adguardhome", verified on OpenWrt
	// 24.10); AdGuardActive is the dnsmasq side of the integration.
	AdGuardInstalled bool `json:"adguard_installed"`
	AdGuardRunning   bool `json:"adguard_running"`
	// AdGuardProtection is the DNS handoff (#360): dnsmasq forwards
	// everything to the local AdGuard listener. AdGuardHasBackup reports the
	// snapshot taken before the handoff, so the UI can promise a restore.
	AdGuardProtection bool        `json:"adguard_protection"`
	AdGuardHasBackup  bool        `json:"adguard_has_backup"`
	AdGuardDnsPort    int         `json:"adguard_dns_port,omitempty"`
	// DoH (#364): whether AdGuard resolves through DNS-over-HTTPS upstreams,
	// the current upstream list (capped at 8) and the provider presets the UI
	// offers. Providers are a backend constant (single source of truth).
	DohEnabled   bool          `json:"doh_enabled"`
	DohUpstreams []string      `json:"doh_upstreams"`
	DohProviders []DohProvider `json:"doh_providers"`
	Hosts        []HostEntry   `json:"hosts"`
}

// HostEntry is one line of the custom hosts mapping.
type HostEntry struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

func dnsApplicable() bool {
	return executor.ServiceEnabled("dnsmasq")
}

// ProbeDNS reads the DNS state.
func ProbeDNS() *DNSConfig {
	st := dnsmasqStateNow()
	protection := adGuardProtected(st)
	c := &DNSConfig{
		Applicable:        dnsApplicable(),
		RebindProtect:     dnsmasqBool("rebind_protection"),
		OverrideDNS:       !dnsmasqBool("localservice"),
		DnsVpn:            dnsmasqBool("dns_vpn_local"),
		ForceDNS:          dhcpHasForceDNS(dhcpOptionValues(), uciGet("network.lan.ipaddr")),
		AdGuardActive:     dnsmasqBool("adguard_active") || uciGet("dhcp.lan.dhcp_option") != "" || protection,
		AdGuardProtection: protection,
		AdGuardHasBackup:  loadAdGuardBackup() != nil,
		Hosts:             parseHostsFile(hostsPath()),
	}
	c.AdGuardInstalled = pkgInstalled("adguardhome")
	c.DohProviders = adGuardDohPresets
	if c.AdGuardInstalled {
		c.AdGuardRunning = executor.ServiceRunning("adguardhome")
		c.AdGuardDnsPort = adGuardResolvedPort()
		c.DohEnabled, c.DohUpstreams = probeDoHState()
	}
	return c
}

// validAdGuardActions are the service actions the card exposes.
var validAdGuardActions = map[string]bool{"start": true, "stop": true}

// AdGuardAction starts or stops the AdGuardHome service, also toggling its
// rc.d autostart so the choice survives a reboot. Only available when the
// package is installed; the healthcheck polls the running state and rolls
// back by reversing the action.
func AdGuardAction(action string) (*DNSConfig, bool, error) {
	if !validAdGuardActions[action] {
		return ProbeDNS(), false, fmt.Errorf("unsupported action %q", action)
	}
	if !pkgInstalled("adguardhome") {
		return ProbeDNS(), false, fmt.Errorf("adguardhome is not installed")
	}
	rcAction := "enable"
	if action == "stop" {
		rcAction = "disable"
	}
	ops := []executor.Op{
		{Kind: "initd", Args: []string{"adguardhome", action}},
		{Kind: "initd", Args: []string{"adguardhome", rcAction}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		return ProbeDNS(), false, err
	}
	want := action == "start"
	for i := 0; i < 10; i++ {
		if executor.ServiceRunning("adguardhome") == want {
			return ProbeDNS(), false, nil
		}
		time.Sleep(time.Second)
	}
	reverse := "start"
	if action == "start" {
		reverse = "stop"
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"adguardhome", reverse}})
	return ProbeDNS(), true, fmt.Errorf("healthcheck failed after %q, rolled back", action)
}

func dnsmasqBool(opt string) bool {
	return dnsmasqGet(opt) == "1"
}

func dnsmasqGet(opt string) string {
	return uciGet("dhcp.@dnsmasq[0]." + opt)
}

func hostsPath() string {
	if p := uciGet("dhcp.@dnsmasq[0].addnhosts"); p != "" {
		return p
	}
	return "/etc/hosts"
}

func parseHostsFile(path string) []HostEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return []HostEntry{}
	}
	var entries []HostEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			entries = append(entries, HostEntry{IP: fields[0], Hostname: fields[1]})
		}
	}
	if entries == nil {
		return []HostEntry{}
	}
	return entries
}

// dhcpOptionValues returns the dhcp.lan.dhcp_option list values (e.g.
// "3,192.168.1.1", "6,192.168.1.1").
func dhcpOptionValues() []string {
	out, err := exec.Command("uci", "show", "dhcp.lan.dhcp_option").Output()
	if err != nil {
		return nil
	}
	return parseDHCPOptionShow(string(out))
}

// parseDHCPOptionShow parses the raw output of `uci show dhcp.lan.dhcp_option`
// into its list values. Pure for testability.
func parseDHCPOptionShow(line string) []string {
	line = strings.TrimSpace(line)
	_, val, ok := strings.Cut(line, "=")
	if !ok {
		return nil
	}
	return parseUCIValues(val)
}

// dhcpHasForceDNS reports whether the dhcp.lan.dhcp_option list already
// contains the "6,<routerIP>" entry that forces clients to use the router.
func dhcpHasForceDNS(values []string, routerIP string) bool {
	want := "6," + routerIP
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// planForceDNSOps builds the UCI ops that toggle DHCP option 6 on dhcp.lan,
// forcing every client to use the router as its DNS server. Enable removes
// any existing 6, entries first, then announces the router; disable only
// removes them.
func planForceDNSOps(current []string, routerIP string, enable bool) []executor.Op {
	var ops []executor.Op
	for _, v := range current {
		if strings.HasPrefix(v, "6,") {
			ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{"dhcp.lan.dhcp_option", v}})
		}
	}
	if enable {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"dhcp.lan.dhcp_option", "6," + routerIP}})
	}
	return ops
}

// SetDNS toggles DNS options (rebind protection, override client DNS, VPN
// DNS, force router DNS).
func SetDNS(rebindProtect, overrideDNS, dnsVpn, forceDNS *bool) (*DNSConfig, bool, error) {
	if !dnsApplicable() {
		return ProbeDNS(), false, fmt.Errorf("DNS settings only apply on the gateway (dnsmasq)")
	}
	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return ProbeDNS(), false, err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}
	var ops []executor.Op
	if rebindProtect != nil {
		ops = append(ops, dnsmasqSet("rebind_protection", boolVal(*rebindProtect))...)
	}
	if overrideDNS != nil {
		ops = append(ops, dnsmasqSet("localservice", boolVal(!*overrideDNS))...)
	}
	if dnsVpn != nil {
		ops = append(ops, dnsmasqSet("dns_vpn_local", boolVal(*dnsVpn))...)
	}
	if forceDNS != nil {
		ip := uciGet("network.lan.ipaddr")
		if ip == "" || !reIPv4.MatchString(ip) {
			return ProbeDNS(), false, fmt.Errorf("cannot resolve the LAN router IP (network.lan.ipaddr)")
		}
		ops = append(ops, planForceDNSOps(dhcpOptionValues(), ip, *forceDNS)...)
	}
	if len(ops) == 0 {
		return ProbeDNS(), false, fmt.Errorf("nothing to change")
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeDNS(), true, err
	}
	return ProbeDNS(), false, nil
}

func boolVal(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func dnsmasqSet(opt, val string) []executor.Op {
	return []executor.Op{{Kind: "uci_set", Args: []string{"dhcp.@dnsmasq[0]." + opt, val}}}
}

// SetHosts adds, edits or removes a hosts entry (gateway only).
func SetHosts(ip, hostname string, remove bool) (*DNSConfig, bool, error) {
	if !dnsApplicable() {
		return ProbeDNS(), false, fmt.Errorf("hosts only apply on the gateway (dnsmasq)")
	}
	if !reIPv4.MatchString(ip) || !reHostname.MatchString(hostname) {
		return ProbeDNS(), false, fmt.Errorf("invalid ip/hostname")
	}
	path := hostsPath()
	snap := readHostsRaw(path)
	rollback := func() { writeHostsRaw(path, snap) }

	entries := parseHostsFile(path)
	var newEntries []HostEntry
	found := false
	for _, e := range entries {
		if strings.EqualFold(e.Hostname, hostname) {
			if remove {
				found = true
				continue
			}
			newEntries = append(newEntries, HostEntry{IP: ip, Hostname: hostname})
			found = true
			continue
		}
		newEntries = append(newEntries, e)
	}
	if !found && !remove {
		newEntries = append(newEntries, HostEntry{IP: ip, Hostname: hostname})
	}
	if err := writeHostsRaw(path, hostEntriesRaw(newEntries)); err != nil {
		rollback()
		return ProbeDNS(), true, err
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	return ProbeDNS(), false, nil
}

func readHostsRaw(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func writeHostsRaw(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func hostEntriesRaw(entries []HostEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s %s\n", e.IP, e.Hostname)
	}
	return b.String()
}
