package modules

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

// Real-shaped `uci show firewall` output: anonymous sections print as
// "firewall.@zone[0]=zone" and list options collapse into one quoted line.
const fwFixture = `firewall.@defaults[0]=defaults
firewall.@defaults[0].input='REJECT'
firewall.@defaults[0].output='ACCEPT'
firewall.@zone[0]=zone
firewall.@zone[0].name='lan'
firewall.@zone[0].input='ACCEPT'
firewall.@zone[0].output='ACCEPT'
firewall.@zone[0].forward='ACCEPT'
firewall.@zone[0].network='lan'
firewall.@zone[1]=zone
firewall.@zone[1].name='wan'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].output='ACCEPT'
firewall.@zone[1].forward='REJECT'
firewall.@zone[1].masq='1'
firewall.@zone[1].network='wan' 'wan6'
firewall.@rule[0]=rule
firewall.@rule[0].name='allow-in'
firewall.@rule[0].proto='tcp'
firewall.@rule[0].src_ip='192.168.1.50'
firewall.@rule[0].dest_ip='10.0.0.9'
firewall.@rule[0].dest_port='80-443'
firewall.@rule[0].target='ACCEPT'
firewall.netgrip_fw_1=rule
firewall.netgrip_fw_1.name='Block TV'
firewall.netgrip_fw_1.proto='tcp'
firewall.netgrip_fw_1.src_ip='192.168.1.51'
firewall.netgrip_fw_1.dest_port='443'
firewall.netgrip_fw_1.target='REJECT'
firewall.netgrip_fw_1.start_time='22:00:00'
firewall.netgrip_fw_1.stop_time='07:00:00'
firewall.netgrip_fw_1.weekdays='Mon Tue Wed Thu Fri Sun'
firewall.netgrip_fw_1.utc_time='1'
firewall.guestzone=zone
firewall.guestzone.name='guest'
firewall.guestzone.input='REJECT'
`

func TestParseFWZonesFromAnonymousSections(t *testing.T) {
	zones := parseFWZonesFrom(fwFixture)
	if len(zones) != 3 {
		t.Fatalf("want 3 zones, got %d: %+v", len(zones), zones)
	}
	if zones[0].Name != "guest" || zones[1].Name != "lan" || zones[2].Name != "wan" {
		t.Fatalf("zones not sorted by name: %+v", zones)
	}
	wan := zones[2]
	if !wan.Masq || wan.Input != "REJECT" {
		t.Errorf("wan zone wrong: %+v", wan)
	}
	if len(wan.Network) != 2 || wan.Network[0] != "wan" || wan.Network[1] != "wan6" {
		t.Errorf("wan network list not parsed: %+v", wan.Network)
	}
	lan := zones[1]
	if lan.Network == nil || len(lan.Network) != 1 {
		t.Errorf("lan network should be a non-nil single-item list: %+v", lan.Network)
	}
}

func TestParseFWRulesFrom(t *testing.T) {
	rules := parseFWRulesFrom(fwFixture)
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %d: %+v", len(rules), rules)
	}
	var r *FWRule
	for i := range rules {
		if rules[i].Section == "@rule[0]" {
			r = &rules[i]
		}
	}
	if r == nil {
		t.Fatal("anonymous rule @rule[0] not parsed")
	}
	if r.Name != "allow-in" || r.Proto != "tcp" || r.SrcIP != "192.168.1.50" || r.DestIP != "10.0.0.9" || r.DestPort != "80-443" || r.Target != "ACCEPT" {
		t.Errorf("rule fields wrong: %+v", r)
	}
	if r.Managed {
		t.Errorf("anonymous rule must not be managed: %+v", r)
	}
	if !r.Enabled {
		t.Errorf("rule without enabled option defaults to enabled")
	}
}

func TestParseTrafficRulesFrom(t *testing.T) {
	rules := parseFWRulesFrom(fwFixture)
	var managed *FWRule
	for i := range rules {
		if rules[i].Managed {
			managed = &rules[i]
		}
	}
	if managed == nil {
		t.Fatal("want one managed rule, got none")
	}
	if managed.Section != "netgrip_fw_1" {
		t.Errorf("section wrong: %q", managed.Section)
	}
	if managed.StartTime != "22:00:00" || managed.StopTime != "07:00:00" {
		t.Errorf("schedule times wrong: %+v", managed)
	}
	if managed.Weekdays != "Mon Tue Wed Thu Fri Sun" {
		t.Errorf("weekdays wrong: %q", managed.Weekdays)
	}
	if !managed.UTCTime {
		t.Errorf("utc_time not parsed")
	}
}

func TestParseFirewallNeverNil(t *testing.T) {
	empty := "firewall.@defaults[0]=defaults\nfirewall.@defaults[0].input='REJECT'\n"

	zones := parseFWZonesFrom(empty)
	if zones == nil {
		t.Fatal("zones: got nil slice on config without sections")
	}
	zb, err := json.Marshal(zones)
	if err != nil {
		t.Fatal(err)
	}
	if string(zb) != "[]" {
		t.Errorf("zones: empty config must marshal to [], got %s", zb)
	}

	rules := parseFWRulesFrom(empty)
	if rules == nil {
		t.Fatal("rules: got nil slice on config without sections")
	}
	rb, err := json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	if string(rb) != "[]" {
		t.Errorf("rules: empty config must marshal to [], got %s", rb)
	}

	// smoke: the fixture itself must never contain a nil slice either
	if parseFWZonesFrom(fwFixture) == nil || parseFWRulesFrom(fwFixture) == nil {
		t.Fatal("fixture parse returned nil slices")
	}
	if strings.Contains(fwFixture, ".type='zone'") {
		t.Fatal("fixture must mimic real uci show output (no .type keys)")
	}
}

func TestTrafficRuleOpsFullSchedule(t *testing.T) {
	rule := FirewallRuleAdd{
		Name:      "Block TV",
		Proto:     "tcp",
		SrcIP:     "192.168.1.51",
		DestPort:  "443",
		Target:    "REJECT",
		StartTime: "22:00",
		StopTime:  "07:00",
		Weekdays:  "Mon Tue Wed",
		UTCTime:   true,
	}
	ops := trafficRuleOps("netgrip_fw_1", rule)

	want := []executor.Op{
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1", "rule"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.name", "Block TV"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.proto", "tcp"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.src_ip", "192.168.1.51"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.dest_port", "443"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.target", "REJECT"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.start_time", "22:00"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.stop_time", "07:00"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.weekdays", "Mon Tue Wed"}},
		{Kind: "uci_set", Args: []string{"firewall.netgrip_fw_1.utc_time", "1"}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
		{Kind: "initd", Args: []string{"firewall", "reload"}},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops mismatch:\n got %+v\nwant %+v", ops, want)
	}
}

func TestTrafficRuleOpsOmitsEmpty(t *testing.T) {
	rule := FirewallRuleAdd{Name: "Any to any deny", Target: "DROP"}
	ops := trafficRuleOps("netgrip_fw_1", rule)
	for _, op := range ops {
		for _, arg := range op.Args {
			if strings.Contains(arg, "dest_ip") || strings.Contains(arg, "src_port") ||
				strings.Contains(arg, "start_time") || strings.Contains(arg, "weekdays") ||
				strings.Contains(arg, "utc_time") {
				t.Errorf("empty option must be omitted, got %v", op.Args)
			}
		}
	}
}

func TestTrafficRuleValidate(t *testing.T) {
	base := FirewallRuleAdd{Name: "ok", Target: "ACCEPT", Proto: "tcp", SrcIP: "192.168.1.0/24", DestPort: "80-443"}
	if err := validateTrafficRule(base); err != nil {
		t.Errorf("valid rule rejected: %v", err)
	}

	cases := []struct {
		mut   func(*FirewallRuleAdd)
		label string
	}{
		{func(r *FirewallRuleAdd) { r.Name = "  " }, "empty name"},
		{func(r *FirewallRuleAdd) { r.Target = "ALLOW" }, "bad target"},
		{func(r *FirewallRuleAdd) { r.Proto = "icmp" }, "bad proto"},
		{func(r *FirewallRuleAdd) { r.SrcIP = "999.1.1.1" }, "bad src ip"},
		{func(r *FirewallRuleAdd) { r.SrcIP = "192.168.1.0/33" }, "bad src cidr"},
		{func(r *FirewallRuleAdd) { r.DestIP = "not-an-ip" }, "bad dest ip"},
		{func(r *FirewallRuleAdd) { r.DestPort = "0" }, "port below range"},
		{func(r *FirewallRuleAdd) { r.DestPort = "65536" }, "port above range"},
		{func(r *FirewallRuleAdd) { r.DestPort = "90-80" }, "reversed port range"},
		{func(r *FirewallRuleAdd) { r.StartTime = "25:00" }, "bad start hour"},
		{func(r *FirewallRuleAdd) { r.StartTime = "22:60" }, "bad start minute"},
		{func(r *FirewallRuleAdd) { r.StopTime = "07" }, "stop missing minutes"},
		{func(r *FirewallRuleAdd) { r.Weekdays = "Monday" }, "full weekday name"},
		{func(r *FirewallRuleAdd) { r.Weekdays = "Mon , Tue" }, "malformed weekdays"},
	}
	for _, c := range cases {
		r := base
		c.mut(&r)
		if err := validateTrafficRule(r); err == nil {
			t.Errorf("%s: expected error, got none", c.label)
		}
	}
}

func TestTrafficRuleLockout(t *testing.T) {
	mgmt := []string{"192.168.1.0/24"}

	// allow rules never lock out.
	if lockoutRisk(FirewallRuleAdd{Target: "ACCEPT", SrcIP: "0.0.0.0/0"}, mgmt) {
		t.Error("allow rule must not be a lockout risk")
	}

	// deny any source with no dest narrows -> risk.
	if !lockoutRisk(FirewallRuleAdd{Target: "DROP"}, mgmt) {
		t.Error("deny with any source must be a lockout risk")
	}
	if !lockoutRisk(FirewallRuleAdd{Target: "REJECT", SrcIP: "0.0.0.0/0"}, mgmt) {
		t.Error("reject 0.0.0.0/0 must be a lockout risk")
	}

	// source covering the management subnet -> risk.
	if !lockoutRisk(FirewallRuleAdd{Target: "DROP", SrcIP: "192.168.0.0/16"}, mgmt) {
		t.Error("deny from a supernet of the management subnet must be a risk")
	}
	if !lockoutRisk(FirewallRuleAdd{Target: "DROP", SrcIP: "192.168.1.0/24"}, mgmt) {
		t.Error("deny from the management subnet itself must be a risk")
	}

	// narrow source, or a dest that excludes the panel -> no risk.
	if lockoutRisk(FirewallRuleAdd{Target: "DROP", SrcIP: "192.168.1.50"}, mgmt) {
		t.Error("deny from a single host is not a lockout risk")
	}
	if lockoutRisk(FirewallRuleAdd{Target: "DROP", SrcIP: "10.0.0.0/8"}, mgmt) {
		t.Error("deny from an unrelated subnet is not a lockout risk")
	}
	if lockoutRisk(FirewallRuleAdd{Target: "DROP", SrcIP: "0.0.0.0/0", DestIP: "10.0.0.9"}, mgmt) {
		t.Error("deny narrowed to a dest host is not a lockout risk")
	}
	if lockoutRisk(FirewallRuleAdd{Target: "DROP", SrcIP: "0.0.0.0/0", DestPort: "443"}, mgmt) {
		t.Error("deny narrowed to a dest port is not a lockout risk")
	}
}

func TestTrafficRuleNextSection(t *testing.T) {
	cases := []struct {
		existing []string
		want     string
	}{
		{nil, "netgrip_fw_1"},
		{[]string{"netgrip_fw_1", "netgrip_fw_2"}, "netgrip_fw_3"},
		{[]string{"netgrip_fw_3", "netgrip_fw_1"}, "netgrip_fw_4"},
		{[]string{"allow-in", "netgrip_fw_5"}, "netgrip_fw_6"},
		{[]string{"netgrip_fwd_9"}, "netgrip_fw_1"},
	}
	for _, c := range cases {
		if got := nextTrafficRuleSection(c.existing); got != c.want {
			t.Errorf("nextTrafficRuleSection(%v) = %q, want %q", c.existing, got, c.want)
		}
	}
}

func TestTrafficRuleSectionGuard(t *testing.T) {
	if reFwRuleSection.MatchString("netgrip_fw_1") == false {
		t.Error("netgrip_fw_1 must match the managed prefix")
	}
	for _, s := range []string{"netgrip_fw_", "netgrip_fw_x", "netgrip_fwd_1", "allow-in", "@rule[0]", ""} {
		if reFwRuleSection.MatchString(s) {
			t.Errorf("%q must not match the managed prefix", s)
		}
	}
}
