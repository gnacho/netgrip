package modules

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"gopkg.in/yaml.v3"
)

// DNS-over-HTTPS upstreams for AdGuard Home (#364). The #360 handoff points
// dnsmasq at AdGuard, but AdGuard itself still resolves through the upstreams
// in /etc/adguardhome.yaml (system resolvers by default). This feature lets
// the user swap those upstreams for DoH endpoints so the whole path from the
// client to the resolver is encrypted. Because enabling rewrites the config
// file (yaml.v3 drops comments and reorders keys), the previous upstream_dns
// and bootstrap_dns are backed up to /etc/netgrip/adguard_doh_backup.json and
// restored if the restart healthcheck fails.

const adGuardDohBackupPath = "/etc/netgrip/adguard_doh_backup.json"

// adGuardBootstrapDNSServers are the plain-IP resolvers AdGuard uses to
// resolve the DoH hostnames themselves. They MUST stay plain IPs: during
// protection the system resolver points back at dnsmasq -> AdGuard, so a
// hostname bootstrap would loop resolution forever.
var adGuardBootstrapDNSServers = []string{"9.9.9.9", "1.1.1.1"}

// DohProvider is one DoH preset offered in the UI. The URL is what gets
// written into upstream_dns; the ID keys the localized label on the frontend.
type DohProvider struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// adGuardDohPresets is the single source of truth for the DoH providers the
// UI offers. It is exposed to the frontend through the DNS probe.
var adGuardDohPresets = []DohProvider{
	{ID: "cloudflare", URL: "https://dns.cloudflare.com/dns-query"},
	{ID: "quad9", URL: "https://dns.quad9.net/dns-query"},
	{ID: "google", URL: "https://dns.google/dns-query"},
	{ID: "adguard", URL: "https://dns.adguard-dns.com/dns-query"},
}

// adGuardDohBackup is the snapshot of the upstream_dns/bootstrap_dns state
// taken before enabling DoH, so a disable or a failed enable can restore it.
// The pointer fields keep "absent" distinct from "empty list".
type adGuardDohBackup struct {
	Version      int       `json:"version"`
	UpstreamDNS  *[]string `json:"upstream_dns"`
	BootstrapDNS *[]string `json:"bootstrap_dns"`
	Created      int64     `json:"created"`
}

// parseAdGuardYAML decodes an AdGuardHome yaml config into a map. An empty
// input yields an empty map rather than an error.
func parseAdGuardYAML(data []byte) (map[string]any, error) {
	doc := map[string]any{}
	if len(data) == 0 {
		return doc, nil
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// yamlStringListPresent walks a path of map keys to a string list, returning
// it plus a presence flag (false when the key path does not exist at all, so
// a restore can delete the key instead of writing an empty list).
func yamlStringListPresent(doc map[string]any, path ...string) ([]string, bool) {
	var cur any = doc
	for _, k := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[k]
		if !ok {
			return nil, false
		}
	}
	list, ok := cur.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out, true
}

// yamlStringList is yamlStringListPresent without the presence flag; missing
// or malformed values collapse to an empty slice (fail-soft, nil-safe).
func yamlStringList(doc map[string]any, path ...string) []string {
	list, _ := yamlStringListPresent(doc, path...)
	if list == nil {
		return []string{}
	}
	return list
}

func getUpstreamDNS(doc map[string]any) []string {
	return yamlStringList(doc, "dns", "upstream_dns")
}

func getBootstrapDNS(doc map[string]any) []string {
	return yamlStringList(doc, "dns", "bootstrap_dns")
}

// anyStrings converts a []string into the []any shape yaml.v3 produces when
// it unmarshals a sequence, so in-memory lists stay indistinguishable from
// freshly parsed ones (and the getters work before any round-trip).
func anyStrings(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

// setDoHUpstream writes upstream_dns and bootstrap_dns under dns:, creating
// the dns: map when absent.
func setDoHUpstream(doc map[string]any, upstreams, bootstrap []string) {
	dnsMap, ok := doc["dns"].(map[string]any)
	if !ok {
		dnsMap = map[string]any{}
		doc["dns"] = dnsMap
	}
	dnsMap["upstream_dns"] = anyStrings(upstreams)
	dnsMap["bootstrap_dns"] = anyStrings(bootstrap)
}

// dohConfig is the upstream_dns/bootstrap_dns slice of an AdGuard config,
// with presence flags so a restore can delete keys that did not exist.
type dohConfig struct {
	upstreams   []string
	upPresent   bool
	bootstrap   []string
	bootPresent bool
}

func captureDoHConfig(doc map[string]any) dohConfig {
	u, up := yamlStringListPresent(doc, "dns", "upstream_dns")
	b, bp := yamlStringListPresent(doc, "dns", "bootstrap_dns")
	return dohConfig{upstreams: u, upPresent: up, bootstrap: b, bootPresent: bp}
}

// applyDoHConfig writes (or deletes) upstream_dns/bootstrap_dns according to
// the presence flags, and removes an emptied dns: map again so a restore
// returns the file to exactly its pre-DoH shape.
func applyDoHConfig(doc map[string]any, c dohConfig) {
	dnsMap, ok := doc["dns"].(map[string]any)
	if !ok {
		dnsMap = map[string]any{}
	}
	if c.upPresent {
		dnsMap["upstream_dns"] = anyStrings(c.upstreams)
	} else {
		delete(dnsMap, "upstream_dns")
	}
	if c.bootPresent {
		dnsMap["bootstrap_dns"] = anyStrings(c.bootstrap)
	} else {
		delete(dnsMap, "bootstrap_dns")
	}
	if len(dnsMap) == 0 {
		delete(doc, "dns")
	} else {
		doc["dns"] = dnsMap
	}
}

// renderAdGuardYAML marshals the config back to yaml. yaml.v3 drops comments
// and sorts keys; callers back up the previous values and the UI warns that
// the file is rewritten.
func renderAdGuardYAML(doc map[string]any) ([]byte, error) {
	return yaml.Marshal(doc)
}

// validateDoHUpstreams rejects an empty list or any upstream that is not an
// https:// URL (plain-IP, tls:// and quic:// upstreams are not DoH).
func validateDoHUpstreams(upstreams []string) error {
	if len(upstreams) == 0 {
		return fmt.Errorf("at least one DNS-over-HTTPS upstream is required")
	}
	for _, u := range upstreams {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("upstream %q is not an https:// URL", u)
		}
	}
	return nil
}

func dohEnabled(upstreams []string) bool {
	for _, u := range upstreams {
		if strings.HasPrefix(u, "https://") {
			return true
		}
	}
	return false
}

// probeDoHState reads the AdGuard yaml and returns whether DoH is active and
// the (capped) upstream list. Fail-soft: an unreadable or unparseable file
// reports inactive with no upstreams.
func probeDoHState() (enabled bool, upstreams []string) {
	data, err := os.ReadFile(adGuardConfigPath)
	if err != nil {
		return false, []string{}
	}
	doc, err := parseAdGuardYAML(data)
	if err != nil {
		return false, []string{}
	}
	ups := getUpstreamDNS(doc)
	if len(ups) > 8 {
		ups = ups[:8]
	}
	return dohEnabled(ups), ups
}

func saveAdGuardDohBackup(b *adGuardDohBackup) error {
	if err := os.MkdirAll(filepath.Dir(adGuardDohBackupPath), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(adGuardDohBackupPath, data, 0o600)
}

func loadAdGuardDohBackup() *adGuardDohBackup {
	data, err := os.ReadFile(adGuardDohBackupPath)
	if err != nil {
		return nil
	}
	var b adGuardDohBackup
	if json.Unmarshal(data, &b) != nil || b.Version != 1 {
		return nil
	}
	return &b
}

func presentPtr(s []string, present bool) *[]string {
	if !present {
		return nil
	}
	return &s
}

func derefPtr(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

// adGuardDoHApply writes the config, restarts AdGuard and waits for a healthy
// DNS answer through dnsmasq (the same readiness poll as the #360 handoff).
func adGuardDoHApply(doc map[string]any) error {
	out, err := renderAdGuardYAML(doc)
	if err != nil {
		return fmt.Errorf("rendering %s: %w", adGuardConfigPath, err)
	}
	if err := os.WriteFile(adGuardConfigPath, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", adGuardConfigPath, err)
	}
	if err := executor.Apply([]executor.Op{{Kind: "initd", Args: []string{"adguardhome", "restart"}}}, nil); err != nil {
		return err
	}
	if !adGuardReady() {
		return fmt.Errorf("dns healthcheck failed")
	}
	return nil
}

// adGuardDoHRestore rewrites the config back to a previous upstream state and
// restarts AdGuard. It leaves the backup file alone; the caller decides when
// the backup has served its purpose.
func adGuardDoHRestore(c dohConfig) {
	data, err := os.ReadFile(adGuardConfigPath)
	if err != nil {
		return
	}
	doc, err := parseAdGuardYAML(data)
	if err != nil {
		return
	}
	applyDoHConfig(doc, c)
	if out, err := renderAdGuardYAML(doc); err == nil {
		_ = os.WriteFile(adGuardConfigPath, out, 0o600)
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"adguardhome", "restart"}})
}

// AdGuardDoH enables or disables DNS-over-HTTPS upstreams for AdGuard Home.
// Gateway-only and gated on the adguardhome package, like the #360 handoff.
func AdGuardDoH(action string, upstreams []string) (*DNSConfig, bool, error) {
	if !dnsApplicable() {
		return ProbeDNS(), false, fmt.Errorf("DNS-over-HTTPS only applies on the gateway (dnsmasq)")
	}
	if !pkgInstalled("adguardhome") {
		return ProbeDNS(), false, fmt.Errorf("adguardhome is not installed")
	}
	switch action {
	case "enable":
		return adGuardDoHEnable(upstreams)
	case "disable":
		return adGuardDoHDisable()
	default:
		return ProbeDNS(), false, fmt.Errorf("unsupported action %q", action)
	}
}

func adGuardDoHEnable(upstreams []string) (*DNSConfig, bool, error) {
	if err := validateDoHUpstreams(upstreams); err != nil {
		return ProbeDNS(), false, err
	}
	data, err := os.ReadFile(adGuardConfigPath)
	if err != nil {
		return ProbeDNS(), false, fmt.Errorf("reading %s: %w", adGuardConfigPath, err)
	}
	doc, err := parseAdGuardYAML(data)
	if err != nil {
		return ProbeDNS(), false, fmt.Errorf("parsing %s: %w", adGuardConfigPath, err)
	}
	prev := captureDoHConfig(doc)
	if err := saveAdGuardDohBackup(&adGuardDohBackup{
		Version:      1,
		UpstreamDNS:  presentPtr(prev.upstreams, prev.upPresent),
		BootstrapDNS: presentPtr(prev.bootstrap, prev.bootPresent),
		Created:      time.Now().Unix(),
	}); err != nil {
		return ProbeDNS(), false, fmt.Errorf("saving DoH backup: %w", err)
	}
	setDoHUpstream(doc, upstreams, adGuardBootstrapDNSServers)
	if err := adGuardDoHApply(doc); err != nil {
		adGuardDoHRestore(prev)
		_ = os.Remove(adGuardDohBackupPath)
		return ProbeDNS(), true, err
	}
	// Only drop the backup once the new config has passed the healthcheck.
	_ = os.Remove(adGuardDohBackupPath)
	return ProbeDNS(), false, nil
}

func adGuardDoHDisable() (*DNSConfig, bool, error) {
	data, err := os.ReadFile(adGuardConfigPath)
	if err != nil {
		return ProbeDNS(), false, fmt.Errorf("reading %s: %w", adGuardConfigPath, err)
	}
	doc, err := parseAdGuardYAML(data)
	if err != nil {
		return ProbeDNS(), false, fmt.Errorf("parsing %s: %w", adGuardConfigPath, err)
	}
	cur := captureDoHConfig(doc)
	backup := loadAdGuardDohBackup()
	if backup != nil {
		applyDoHConfig(doc, dohConfig{
			upstreams:   derefPtr(backup.UpstreamDNS),
			upPresent:   backup.UpstreamDNS != nil,
			bootstrap:   derefPtr(backup.BootstrapDNS),
			bootPresent: backup.BootstrapDNS != nil,
		})
	} else {
		applyDoHConfig(doc, dohConfig{})
	}
	if err := adGuardDoHApply(doc); err != nil {
		// Re-apply the DoH-enabled state and keep the backup so a retry works.
		adGuardDoHRestore(cur)
		return ProbeDNS(), true, err
	}
	_ = os.Remove(adGuardDohBackupPath)
	return ProbeDNS(), false, nil
}
