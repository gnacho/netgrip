package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// ddnsEntryName maps a domain to a safe UCI section name.
func ddnsEntryName(domain string) string {
	// OpenWrt UCI section names only tolerate alphanumeric and underscore in
	// practice. Replace everything else with an underscore and collapse.
	safe := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(domain, "_")
	safe = strings.Trim(safe, "_")
	if safe == "" || (safe[0] >= '0' && safe[0] <= '9') {
		safe = "d" + safe
	}
	return safe
}

// DDNSConfig is the user-provided entry configuration. Password is write-only.
type DDNSConfig struct {
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"service_name"`
	Domain      string `json:"domain"`
	LookupHost  string `json:"lookup_host"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
}

// DDNSEntry is a single DDNS service state.
type DDNSEntry struct {
	Section      string `json:"section"`
	Enabled      bool   `json:"enabled"`
	Running      bool   `json:"running"`
	ServiceName  string `json:"service_name"`
	Domain       string `json:"domain"`
	LookupHost   string `json:"lookup_host"`
	Username     string `json:"username"`
	RegisteredIP string `json:"registered_ip"`
	LastUpdate   string `json:"last_update"`
}

// DDNSProbe is the read-only DDNS state.
type DDNSProbe struct {
	Installed bool        `json:"installed"`
	Entries   []DDNSEntry `json:"entries"`
}

func ddnsInstalled() bool {
	_, err := os.Stat("/etc/init.d/ddns")
	return err == nil
}

func ddnsServiceRunning(section string) bool {
	out, err := exec.Command("pgrep", "-f", "dynamic_dns_updater.sh.*"+section).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// ddnsSections returns the named ddns service sections currently configured.
func ddnsSections() []string {
	out, err := exec.Command("uci", "show", "ddns").Output()
	if err != nil {
		return nil
	}
	var sections []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ddns.") {
			continue
		}
		// Format: ddns.<section>.option=...
		rest := strings.TrimPrefix(line, "ddns.")
		idx := strings.IndexAny(rest, ".=")
		if idx == -1 {
			continue
		}
		section := rest[:idx]
		if section == "" || seen[section] {
			continue
		}
		seen[section] = true
		sections = append(sections, section)
	}
	return sections
}

func ddnsReadEntry(section string) *DDNSEntry {
	base := "ddns." + section
	if !uciSectionExists(base) {
		return nil
	}
	e := &DDNSEntry{Section: section}
	e.ServiceName = uciGet(base + ".service_name")
	e.Domain = uciGet(base + ".domain")
	e.LookupHost = uciGet(base + ".lookup_host")
	e.Username = uciGet(base + ".username")
	e.Enabled = uciGet(base+".enabled") == "1"
	e.Running = ddnsServiceRunning(section)
	if data, err := os.ReadFile("/tmp/run/ddns/" + section + ".dat"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "registered_ip=") {
				e.RegisteredIP = strings.Trim(strings.TrimPrefix(line, "registered_ip="), "'\"")
			}
		}
		if st, err := os.Stat("/tmp/run/ddns/" + section + ".dat"); err == nil {
			e.LastUpdate = st.ModTime().Format(time.RFC3339)
		}
	}
	return e
}

// ProbeDDNS reads all configured DDNS entries. Never includes passwords.
// Only service sections with a domain are listed: the stock ddns-scripts
// example rows ship as service sections, but the global section (no domain)
// is noise (#204).
func ProbeDDNS() *DDNSProbe {
	p := &DDNSProbe{Installed: ddnsInstalled()}
	if !p.Installed {
		return p
	}
	for _, section := range ddnsSections() {
		if uciGet("ddns."+section) != "service" {
			continue
		}
		e := ddnsReadEntry(section)
		if e == nil || e.Domain == "" {
			continue
		}
		p.Entries = append(p.Entries, *e)
	}
	return p
}

func ddnsEntryByDomain(domain string) *DDNSEntry {
	section := ddnsEntryName(domain)
	if uciSectionExists("ddns." + section) {
		return ddnsReadEntry(section)
	}
	// Fallback: scan by domain value in case the naming convention changed.
	for _, s := range ddnsSections() {
		if e := ddnsReadEntry(s); e != nil && e.Domain == domain {
			return e
		}
	}
	return nil
}

// ddnsSnapshot captures /etc/config/ddns for rollback.
func ddnsSnapshot() string {
	if _, err := os.Stat("/etc/config/ddns"); err != nil {
		return ""
	}
	if s, err := executor.Snapshot("ddns"); err == nil {
		return s
	}
	return ""
}

func ddnsRollback(snap string) {
	if snap != "" {
		_ = executor.Restore("ddns", snap)
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"ddns", "reload"}})
}

// SetDDNS creates or updates a single DDNS entry identified by its domain.
func SetDDNS(cfg DDNSConfig) (*DDNSProbe, bool, error) {
	if cfg.Domain == "" {
		return ProbeDDNS(), false, fmt.Errorf("domain is required")
	}
	if cfg.Enabled && cfg.ServiceName == "" {
		return ProbeDDNS(), false, fmt.Errorf("service_name and domain are required")
	}

	snap := ddnsSnapshot()
	section := ddnsEntryName(cfg.Domain)
	hadConfig := uciSectionExists("ddns." + section)

	ops, err := ddnsEntryOps(cfg, section, hadConfig)
	if err != nil {
		return ProbeDDNS(), false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		ddnsRollback(snap)
		return ProbeDDNS(), true, err
	}

	if !cfg.Enabled {
		ddnsStopUpdater(section)
	}

	ok := func() bool {
		for range 5 {
			probe := ProbeDDNS()
			entry := findDDNSEntry(probe.Entries, cfg.Domain)
			if cfg.Enabled {
				if entry != nil && entry.Enabled && entry.Running {
					return true
				}
			} else if entry == nil || (!entry.Enabled && !entry.Running) {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	if !ok() {
		ddnsRollback(snap)
		return ProbeDDNS(), true, fmt.Errorf("healthcheck failed after apply (enabled=%v), rolled back", cfg.Enabled)
	}
	return ProbeDDNS(), false, nil
}

func findDDNSEntry(entries []DDNSEntry, domain string) *DDNSEntry {
	for i, e := range entries {
		if e.Domain == domain {
			return &entries[i]
		}
	}
	return nil
}

// DeleteDDNSSection removes one DDNS entry by its unique section id (#204).
func DeleteDDNSSection(section string) (*DDNSProbe, bool, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9_@:-]{1,64}$`).MatchString(section) {
		return ProbeDDNS(), false, fmt.Errorf("invalid section name")
	}
	if !uciSectionExists("ddns." + section) {
		return ProbeDDNS(), false, fmt.Errorf("ddns section %q not found", section)
	}
	snap := ddnsSnapshot()
	ops := []executor.Op{
		{Kind: "uci_delete", Args: []string{"ddns." + section}},
		{Kind: "uci_commit", Args: []string{"ddns"}},
		{Kind: "initd", Args: []string{"ddns", "reload"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		ddnsRollback(snap)
		return ProbeDDNS(), true, err
	}
	return ProbeDDNS(), false, nil
}

// DeleteDDNS removes every DDNS entry matching a domain (compat path).
func DeleteDDNS(domain string) (*DDNSProbe, bool, error) {
	if domain == "" {
		return ProbeDDNS(), false, fmt.Errorf("domain is required")
	}
	var sections []string
	for _, s := range ddnsSections() {
		if e := ddnsReadEntry(s); e != nil && e.Domain == domain {
			sections = append(sections, s)
		}
	}
	if len(sections) == 0 {
		return ProbeDDNS(), false, fmt.Errorf("ddns entry not found for domain %q", domain)
	}
	snap := ddnsSnapshot()
	var ops []executor.Op
	for _, section := range sections {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"ddns." + section}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"ddns"}},
		executor.Op{Kind: "initd", Args: []string{"ddns", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		ddnsRollback(snap)
		return ProbeDDNS(), true, err
	}
	return ProbeDDNS(), false, nil
}

// ddnsStopUpdater kills a per-service updater process (best effort).
func ddnsStopUpdater(section string) {
	data, err := os.ReadFile("/tmp/run/ddns/" + section + ".pid")
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
}

func ddnsEntryOps(cfg DDNSConfig, section string, hadConfig bool) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	base := "ddns." + section

	if !cfg.Enabled {
		if hadConfig {
			set(base+".enabled", "0")
			ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"ddns"}})
		}
		return ops, nil
	}

	if !ddnsInstalled() {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"ddns-scripts"}})
	}
	set(base, "service")
	set(base+".service_name", cfg.ServiceName)
	set(base+".domain", cfg.Domain)
	set(base+".username", cfg.Username)
	if cfg.Password != "" {
		set(base+".password", cfg.Password)
	}
	if cfg.LookupHost != "" {
		set(base+".lookup_host", cfg.LookupHost)
	}
	if uciSectionExists("network.wan") {
		set(base+".ip_source", "network")
		set(base+".ip_network", "wan")
	} else {
		set(base+".ip_source", "web")
		set(base+".ip_url", "http://checkip.dyndns.org")
	}
	set(base+".enabled", "1")
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"ddns"}},
		executor.Op{Kind: "initd", Args: []string{"ddns", "enable"}},
		executor.Op{Kind: "initd", Args: []string{"ddns", "start"}},
	)
	return ops, nil
}
