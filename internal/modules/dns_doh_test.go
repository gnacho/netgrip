package modules

import (
	"strings"
	"testing"
)

// A realistic full wizard-generated AdGuardHome.yaml: dns section already
// present with plain-IP upstreams and bootstrap (the pre-DoH state).
const wizardYAML = `http:
  address: 0.0.0.0:80
  session_ttl: 720h
users:
  - name: admin
    password: $2y$10$HASHED
    id: 1
auth_attempts: 5
block_auth_min: 15
language: ""
theme: auto
dns:
  bind_hosts:
    - 0.0.0.0
  port: 53
  anonymize_client_ip: false
  ratelimit: 20
  refuse_any: true
  upstream_dns:
    - 1.1.1.1
    - 8.8.8.8
  upstream_dns_file: ""
  bootstrap_dns:
    - 1.1.1.1
    - 8.8.8.8
  all_servers: false
  fastest_addr: false
  allowed_clients: []
  disallowed_clients: []
  blocked_hosts: []
  trusted_proxies:
    - 127.0.0.0/8
    - ::1/128
`

// A config with no dns section at all (e.g. freshly written minimal http).
const noDNSYAML = "http:\n  address: 0.0.0.0:3000\n"

func TestParseAdGuardYAMLRoundTrip(t *testing.T) {
	for _, in := range []string{adGuardMinimalYAML(5353), wizardYAML, noDNSYAML} {
		doc, err := parseAdGuardYAML([]byte(in))
		if err != nil {
			t.Fatalf("parse %q: %v", in, err)
		}
		out, err := renderAdGuardYAML(doc)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		again, err := parseAdGuardYAML(out)
		if err != nil {
			t.Fatalf("re-parse: %v", err)
		}
		if strings.Join(getUpstreamDNS(doc), "|") != strings.Join(getUpstreamDNS(again), "|") {
			t.Errorf("upstream_dns changed across round-trip: %v -> %v", getUpstreamDNS(doc), getUpstreamDNS(again))
		}
		if strings.Join(getBootstrapDNS(doc), "|") != strings.Join(getBootstrapDNS(again), "|") {
			t.Errorf("bootstrap_dns changed across round-trip: %v -> %v", getBootstrapDNS(doc), getBootstrapDNS(again))
		}
	}
}

func TestGetUpstreamDNS(t *testing.T) {
	min, _ := parseAdGuardYAML([]byte(adGuardMinimalYAML(5353)))
	if got := getUpstreamDNS(min); len(got) != 0 {
		t.Errorf("minimal yaml must have no upstreams, got %v", got)
	}
	wiz, _ := parseAdGuardYAML([]byte(wizardYAML))
	want := []string{"1.1.1.1", "8.8.8.8"}
	if got := getUpstreamDNS(wiz); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wizard upstreams = %v, want %v", got, want)
	}
	// Nil-safe and absent-section.
	if got := getUpstreamDNS(nil); len(got) != 0 {
		t.Errorf("nil doc must give empty, got %v", got)
	}
	nd, _ := parseAdGuardYAML([]byte(noDNSYAML))
	if got := getUpstreamDNS(nd); len(got) != 0 {
		t.Errorf("no-dns yaml must give empty, got %v", got)
	}
}

func TestGetBootstrapDNS(t *testing.T) {
	wiz, _ := parseAdGuardYAML([]byte(wizardYAML))
	want := []string{"1.1.1.1", "8.8.8.8"}
	if got := getBootstrapDNS(wiz); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wizard bootstrap = %v, want %v", got, want)
	}
	min, _ := parseAdGuardYAML([]byte(adGuardMinimalYAML(5353)))
	if got := getBootstrapDNS(min); len(got) != 0 {
		t.Errorf("minimal yaml must have no bootstrap, got %v", got)
	}
}

func TestSetDoHUpstreamCreatesDNS(t *testing.T) {
	doc, _ := parseAdGuardYAML([]byte(noDNSYAML))
	setDoHUpstream(doc, []string{"https://dns.cloudflare.com/dns-query"}, adGuardBootstrapDNSServers)
	if got := getUpstreamDNS(doc); len(got) != 1 || got[0] != "https://dns.cloudflare.com/dns-query" {
		t.Errorf("upstreams after set = %v", got)
	}
	if got := getBootstrapDNS(doc); strings.Join(got, "|") != "9.9.9.9|1.1.1.1" {
		t.Errorf("bootstrap after set = %v", got)
	}
	// The existing http: block must survive.
	out, err := renderAdGuardYAML(doc)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "3000") {
		t.Errorf("http port lost after set: %s", out)
	}
}

func TestSetDoHUpstreamOverwrites(t *testing.T) {
	doc, _ := parseAdGuardYAML([]byte(wizardYAML))
	setDoHUpstream(doc, []string{"https://dns.google/dns-query"}, adGuardBootstrapDNSServers)
	got := getUpstreamDNS(doc)
	if len(got) != 1 || got[0] != "https://dns.google/dns-query" {
		t.Errorf("upstreams after overwrite = %v", got)
	}
	if got := getBootstrapDNS(doc); strings.Join(got, "|") != "9.9.9.9|1.1.1.1" {
		t.Errorf("bootstrap after overwrite = %v", got)
	}
}

func TestValidateDoHUpstreams(t *testing.T) {
	if err := validateDoHUpstreams(nil); err == nil {
		t.Error("empty list must be rejected")
	}
	if err := validateDoHUpstreams([]string{}); err == nil {
		t.Error("empty list must be rejected")
	}
	for _, bad := range []string{"1.1.1.1", "tls://dns.quad9.net", "quic://dns.adguard-dns.com", "https://", "not a url"} {
		if err := validateDoHUpstreams([]string{bad}); err == nil {
			t.Errorf("upstream %q must be rejected", bad)
		}
	}
	good := []string{"https://dns.cloudflare.com/dns-query", "https://dns.google/dns-query"}
	if err := validateDoHUpstreams(good); err != nil {
		t.Errorf("valid https upstreams rejected: %v", err)
	}
}

func TestCaptureAndApplyDoHConfigPreservesPresence(t *testing.T) {
	// wizard has both keys present.
	doc, _ := parseAdGuardYAML([]byte(wizardYAML))
	c := captureDoHConfig(doc)
	if !c.upPresent || !c.bootPresent {
		t.Errorf("wizard must have both keys present, got %+v", c)
	}
	// Removing both keys then restoring c must bring them back exactly.
	applyDoHConfig(doc, dohConfig{})
	if got := getUpstreamDNS(doc); len(got) != 0 {
		t.Errorf("after clearing, upstreams = %v", got)
	}
	applyDoHConfig(doc, c)
	if got := getUpstreamDNS(doc); strings.Join(got, "|") != "1.1.1.1|8.8.8.8" {
		t.Errorf("after restore, upstreams = %v", got)
	}
	if got := getBootstrapDNS(doc); strings.Join(got, "|") != "1.1.1.1|8.8.8.8" {
		t.Errorf("after restore, bootstrap = %v", got)
	}
}

func TestApplyDoHConfigDeletesEmptyDNSMap(t *testing.T) {
	// A config whose only dns content was the DoH keys collapses back to no
	// dns: section when both are removed, so a disable restores the exact
	// pre-DoH shape.
	doc, _ := parseAdGuardYAML([]byte(noDNSYAML))
	setDoHUpstream(doc, []string{"https://dns.google/dns-query"}, adGuardBootstrapDNSServers)
	applyDoHConfig(doc, dohConfig{})
	if _, ok := doc["dns"]; ok {
		t.Errorf("dns: section must be removed when emptied, got %v", doc)
	}
}

func TestDoHBackupRestoreSemantics(t *testing.T) {
	// A disable with no backup removes both keys even when the config has
	// other dns content (bind_hosts, port) that must survive.
	doc, _ := parseAdGuardYAML([]byte(wizardYAML))
	applyDoHConfig(doc, dohConfig{})
	if got := getUpstreamDNS(doc); len(got) != 0 {
		t.Errorf("disable without backup must clear upstreams, got %v", got)
	}
	if got := getBootstrapDNS(doc); len(got) != 0 {
		t.Errorf("disable without backup must clear bootstrap, got %v", got)
	}
	out, _ := renderAdGuardYAML(doc)
	if !strings.Contains(string(out), "53") {
		t.Errorf("dns.port must survive a DoH disable: %s", out)
	}

	// A restore from a backup that only recorded upstream_dns (bootstrap was
	// absent) must delete bootstrap_dns but not invent it.
	doc2, _ := parseAdGuardYAML([]byte(wizardYAML))
	onlyUpstream := []string{"1.1.1.1"}
	applyDoHConfig(doc2, dohConfig{upstreams: onlyUpstream, upPresent: true, bootPresent: false})
	if got := getBootstrapDNS(doc2); len(got) != 0 {
		t.Errorf("bootstrap must stay absent when the backup did not record it, got %v", got)
	}
	if got := getUpstreamDNS(doc2); strings.Join(got, "|") != "1.1.1.1" {
		t.Errorf("upstreams not restored from backup: %v", got)
	}
}

func TestDohEnabledDetection(t *testing.T) {
	if dohEnabled([]string{"1.1.1.1", "https://dns.cloudflare.com/dns-query"}) != true {
		t.Error("https upstream must mark DoH active")
	}
	if dohEnabled([]string{"1.1.1.1", "8.8.8.8"}) {
		t.Error("plain-IP only must not mark DoH active")
	}
	if dohEnabled(nil) {
		t.Error("empty list must not mark DoH active")
	}
}
