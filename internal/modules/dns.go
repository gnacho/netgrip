package modules

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

var reHostname = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?$`)

// DNSConfig is the read-only DNS state.
type DNSConfig struct {
	Applicable    bool        `json:"applicable"`
	RebindProtect bool        `json:"rebind_protection"`
	OverrideDNS   bool        `json:"override_dns"`
	DnsVpn        bool        `json:"dns_vpn"`
	AdGuardActive bool        `json:"adguard_active"`
	Hosts         []HostEntry `json:"hosts"`
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
	c := &DNSConfig{
		Applicable:    dnsApplicable(),
		RebindProtect: dnsmasqBool("rebind_protection"),
		OverrideDNS:   !dnsmasqBool("localservice"),
		DnsVpn:        dnsmasqBool("dns_vpn_local"),
		AdGuardActive: dnsmasqBool("adguard_active") || uciGet("dhcp.lan.dhcp_option") != "",
		Hosts:         parseHostsFile(hostsPath()),
	}
	return c
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

// SetDNS toggles DNS options (rebind protection, override client DNS).
func SetDNS(rebindProtect, overrideDNS, dnsVpn *bool) (*DNSConfig, bool, error) {
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
