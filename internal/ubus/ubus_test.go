package ubus

import (
	"testing"
	"time"
)

func TestParseLeases(t *testing.T) {
	content := `1756000000 aa:bb:cc:dd:ee:ff 192.168.1.50 device-one 01:aa:bb:cc:dd:ee:ff
1756000100 11:22:33:44:55:66 192.168.1.51 device-two *
`
	leases := ParseLeases(content)
	if len(leases) != 2 {
		t.Fatalf("expected 2 leases, got %d", len(leases))
	}
	first := leases[0]
	if first.MAC != "aa:bb:cc:dd:ee:ff" || first.IP != "192.168.1.50" || first.Hostname != "device-one" {
		t.Errorf("unexpected first lease: %+v", first)
	}
	if first.ClientID != "01:aa:bb:cc:dd:ee:ff" {
		t.Errorf("unexpected client id: %q", first.ClientID)
	}
	if !first.Expires.Equal(time.Unix(1756000000, 0)) {
		t.Errorf("unexpected expiry: %v", first.Expires)
	}
	if leases[1].ClientID != "*" {
		t.Errorf("unexpected second client id: %q", leases[1].ClientID)
	}
}

func TestParseLeasesSkipsGarbage(t *testing.T) {
	content := "\nnot-a-lease\n1756000000 aa:bb:cc:dd:ee:ff 192.168.1.50\n"
	leases := ParseLeases(content)
	if len(leases) != 0 {
		t.Fatalf("expected 0 leases, got %d", len(leases))
	}
}

func TestMACRegexp(t *testing.T) {
	if !macRe.MatchString("aa:bb:cc:dd:ee:ff") {
		t.Error("valid MAC not matched")
	}
	if macRe.MatchString("freq") || macRe.MatchString("192.168.1.1") {
		t.Error("invalid MAC matched")
	}
}
