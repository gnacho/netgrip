package modules

import "testing"

// The factory config is how a stock rule is recognised, so that the rules
// every OpenWrt ships with (DHCP renew, ISAKMP, the ICMP ones) do not bury
// the one rule somebody actually opened. Read from /rom, never hardcoded.
func TestParseStockNames(t *testing.T) {
	const cfg = `
config defaults
	option input 'REJECT'

config zone
	option name		lan
	option input 'ACCEPT'

config rule
	option name		Allow-DHCP-Renew
	option src		wan
	option proto		udp
	option dest_port	68
	option target		ACCEPT

config rule
	option name 'Allow-ISAKMP'
	option dest_port	500
`
	got := parseStockNames(cfg)
	for _, want := range []string{"lan", "Allow-DHCP-Renew", "Allow-ISAKMP"} {
		if !got[want] {
			t.Errorf("%q should be recognised as stock: %v", want, got)
		}
	}
	if got["Allow-Something-Else"] {
		t.Error("a rule that is not in the factory config is not stock")
	}
	if len(got) != 3 {
		t.Errorf("names: %v", got)
	}
}

// No /rom (a container, an image without the read-only base): nothing is
// treated as stock, which errs towards showing a rule rather than hiding it.
func TestParseStockNamesOnNothing(t *testing.T) {
	if got := parseStockNames(""); len(got) != 0 {
		t.Errorf("names: %v", got)
	}
}

// The section name encodes the external port and the destination, so an edit
// that changes either has to move the rule: a stale name would collide with
// a later rule for the old port.
func TestFwdSectionEncodesPortAndDestination(t *testing.T) {
	if got := fwdSection("443", "192.168.1.10"); got != "netgrip_fwd_443_192_168_1_10" {
		t.Fatalf("fwdSection() = %q", got)
	}
}

func TestFwdUpdateOpsMoveWhenTheNameChanges(t *testing.T) {
	const old = "netgrip_fwd_443_192_168_1_10"

	moved := fwdUpdateOps(old, "netgrip_fwd_8443_192_168_1_20", "8443", "192.168.1.20", "8443", "tcp")
	if len(moved) == 0 || moved[0].Kind != "uci_delete" || moved[0].Args[0] != "firewall."+old {
		t.Fatalf("la primera op debe borrar la seccion vieja: %+v", moved)
	}
	if moved[len(moved)-2].Kind != "uci_commit" || moved[len(moved)-1].Kind != "initd" {
		t.Fatalf("la tanda debe terminar en commit + reload: %+v", moved)
	}

	same := fwdUpdateOps(old, old, "443", "192.168.1.10", "8443", "tcp")
	for _, op := range same {
		if op.Kind == "uci_delete" {
			t.Fatalf("sin cambio de nombre no se borra nada: %+v", same)
		}
	}
}
