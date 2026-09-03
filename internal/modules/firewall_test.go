package modules

import (
	"encoding/json"
	"strings"
	"testing"
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
firewall.@rule[0].src='wan'
firewall.@rule[0].dest='lan'
firewall.@rule[0].proto='tcp'
firewall.@rule[0].dest_port='22'
firewall.@rule[0].target='ACCEPT'
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
	if len(rules) != 1 {
		t.Fatalf("want 1 rule, got %d: %+v", len(rules), rules)
	}
	r := rules[0]
	if r.Name != "allow-in" || r.Src != "wan" || r.Dest != "lan" || r.Proto != "tcp" || r.DestPort != "22" || r.Target != "ACCEPT" {
		t.Errorf("rule fields wrong: %+v", r)
	}
	if r.Section != "@rule[0]" {
		t.Errorf("rule section should be the anonymous id, got %q", r.Section)
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
