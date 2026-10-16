package modules

import (
	"testing"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// leaseFixture is the real /var/run/miniupnpd.leases format written by
// miniupnpd 2.x (upnpredirect.c lease_file_add): PROTO:ext:int:port:ts:desc
// with an absolute unix timestamp. The OpenWrt build defines neither
// SUPPORT_REMOTEHOST nor LEASEFILE_USE_REMAINING_TIME.
const leaseFixture = `TCP:5000:192.168.1.10:5000:1700000000:Plex Media Server
UDP:51820:192.168.1.20:51820:0:wireguard
TCP:8080:192.168.1.30:80:1700000300:web:8080
`

func TestParseUpnpLeaseFile(t *testing.T) {
	now := time.Unix(1699999900, 0)
	leases := parseUpnpLeaseFile(leaseFixture, now)
	if len(leases) != 3 {
		t.Fatalf("leases = %d, want 3", len(leases))
	}
	l := leases[0]
	if l.Proto != "TCP" || l.ExtPort != 5000 || l.IntAddr != "192.168.1.10" || l.IntPort != 5000 {
		t.Errorf("lease[0] = %+v", l)
	}
	if l.LeaseTime != 100 { // 1700000000 - 1699999900
		t.Errorf("lease[0].lease_time = %d, want 100", l.LeaseTime)
	}
	if l.Desc != "Plex Media Server" {
		t.Errorf("lease[0].desc = %q", l.Desc)
	}
	// A 0 timestamp means no expiry.
	if leases[1].LeaseTime != 0 {
		t.Errorf("lease[1].lease_time = %d, want 0", leases[1].LeaseTime)
	}
	// The description may itself contain colons.
	if leases[2].Desc != "web:8080" {
		t.Errorf("lease[2].desc = %q", leases[2].Desc)
	}
	// An already-expired lease clamps to 0.
	if got := parseUpnpLeaseFile("TCP:1:192.168.1.1:1:100:old\n", time.Unix(1000, 0)); got[0].LeaseTime != 0 {
		t.Errorf("expired lease_time = %d, want 0", got[0].LeaseTime)
	}
	// Malformed lines are skipped.
	if got := parseUpnpLeaseFile("garbage\nno-colons\nTCP:1:2\n", now); len(got) != 0 {
		t.Errorf("malformed parsed %d entries, want 0", len(got))
	}
}

// upnpShowFixture is a trimmed `uci show upnpd` with the stock config plus
// one anonymous and one named perm_rule.
const upnpShowFixture = `upnpd.config=upnpd
upnpd.config.enabled='1'
upnpd.config.enable_natpmp='1'
upnpd.config.enable_upnp='1'
upnpd.config.secure_mode='1'
upnpd.config.internal_iface='lan'
upnpd.config.upnp_lease_file='/var/run/miniupnpd.leases'
upnpd.@perm_rule[0]=perm_rule
upnpd.@perm_rule[0].action='allow'
upnpd.@perm_rule[0].ext_ports='1024-65535'
upnpd.@perm_rule[0].int_addr='0.0.0.0/0'
upnpd.@perm_rule[0].int_ports='1024-65535'
upnpd.@perm_rule[0].comment='Allow high ports'
upnpd.netgrip_upnp_1=perm_rule
upnpd.netgrip_upnp_1.action='deny'
upnpd.netgrip_upnp_1.int_addr='192.168.1.66'
upnpd.netgrip_upnp_1.comment='blocked client'
`

func TestUpnpConfigAndRules(t *testing.T) {
	sections := parseUCIShow(upnpShowFixture, "upnpd")
	opts := upnpConfigOpts(sections)
	if got := upnpOpt(opts, "enabled"); got != "1" {
		t.Errorf("enabled = %q", got)
	}
	if got := upnpOpt(opts, "upnp_lease_file"); got != "/var/run/miniupnpd.leases" {
		t.Errorf("upnp_lease_file = %q", got)
	}
	if got := upnpLeaseFilePath(opts); got != "/var/run/miniupnpd.leases" {
		t.Errorf("lease path = %q", got)
	}

	rules := upnpRulesFromSections(sections)
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(rules))
	}
	// Sorted by section name: @perm_rule[0] before netgrip_upnp_1.
	if rules[0].Section != "@perm_rule[0]" || rules[0].Managed {
		t.Errorf("rules[0] = %+v, want anonymous non-managed", rules[0])
	}
	if rules[1].Section != "netgrip_upnp_1" || !rules[1].Managed {
		t.Errorf("rules[1] = %+v, want managed netgrip rule", rules[1])
	}
	if rules[1].Action != "deny" || rules[1].IntAddr != "192.168.1.66" || rules[1].Comment != "blocked client" {
		t.Errorf("rules[1] fields = %+v", rules[1])
	}
}

func TestUpnpLeaseFilePathDefault(t *testing.T) {
	if got := upnpLeaseFilePath(nil); got != upnpLeaseFileDef {
		t.Errorf("default lease path = %q", got)
	}
}

func TestUpnpRuleOps(t *testing.T) {
	ops, err := upnpRuleOps("netgrip_upnp_1", UpnpRuleConfig{
		Action:   "allow",
		IntAddr:  "192.168.1.0/24",
		IntPorts: "1024-65535",
		ExtPorts: "1024-65535",
		Comment:  "whole lan",
	})
	if err != nil {
		t.Fatalf("upnpRuleOps: %v", err)
	}
	// First op declares the named section type, last two commit + reload.
	if ops[0].Kind != "uci_set" || ops[0].Args[0] != "upnpd.netgrip_upnp_1" || ops[0].Args[1] != "perm_rule" {
		t.Errorf("ops[0] = %+v", ops[0])
	}
	if ops[len(ops)-2].Kind != "uci_commit" || ops[len(ops)-1].Kind != "initd" {
		t.Errorf("trailing ops = %+v / %+v", ops[len(ops)-2], ops[len(ops)-1])
	}
	// Every write must validate against the executor allowlist.
	for _, op := range ops {
		if err := executor.Validate(op); err != nil {
			t.Errorf("op %+v failed allowlist: %v", op, err)
		}
	}
}

func TestUpnpRuleOpsValidation(t *testing.T) {
	if _, err := upnpRuleOps("netgrip_upnp_1", UpnpRuleConfig{Action: "maybe", IntAddr: "192.168.1.1"}); err == nil {
		t.Errorf("bad action accepted")
	}
	if _, err := upnpRuleOps("netgrip_upnp_1", UpnpRuleConfig{Action: "allow", IntAddr: "not-an-ip"}); err == nil {
		t.Errorf("bad int_addr accepted")
	}
	if _, err := upnpRuleOps("netgrip_upnp_1", UpnpRuleConfig{Action: "allow", IntAddr: "192.168.1.1", ExtPorts: "1;2"}); err == nil {
		t.Errorf("bad ext_ports accepted")
	}
	if _, err := upnpRuleOps("netgrip_upnp_1", UpnpRuleConfig{Action: "allow", IntAddr: "192.168.1.1", Comment: "a;b"}); err == nil {
		t.Errorf("bad comment accepted")
	}
}

func TestUpnpNextRuleSection(t *testing.T) {
	sections := parseUCIShow(upnpShowFixture, "upnpd")
	if got := upnpNextRuleSection(sections); got != "netgrip_upnp_2" {
		t.Errorf("next section = %q, want netgrip_upnp_2", got)
	}
	if got := upnpNextRuleSection(nil); got != "netgrip_upnp_1" {
		t.Errorf("next section from empty = %q", got)
	}
}

func TestUpnpActionsValid(t *testing.T) {
	for _, a := range []string{"start", "stop", "reload", "restart", "enable", "disable"} {
		if !validUpnpActions[a] {
			t.Errorf("action %q should be valid", a)
		}
	}
	if validUpnpActions["destroy"] {
		t.Errorf("action destroy should not be valid")
	}
}
