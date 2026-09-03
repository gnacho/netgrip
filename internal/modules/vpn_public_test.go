package modules

import "testing"

func TestOvpnRemoteEndpointPreference(t *testing.T) {
	cases := []struct {
		name                     string
		explicit, host, wan, lan string
		want                     string
	}{
		{"explicit wins", "vpn.example.com", "ddns.example.org", "79.1.2.3", "192.168.1.1", "vpn.example.com"},
		{"public host beats wan", "", "ddns.example.org", "79.1.2.3", "192.168.1.1", "ddns.example.org"},
		{"wan beats lan", "", "", "79.1.2.3", "192.168.1.1", "79.1.2.3"},
		{"lan fallback", "", "", "", "192.168.1.1", "192.168.1.1"},
		{"all empty", "", "", "", "", ""},
	}
	for _, c := range cases {
		if got := ovpnRemoteEndpoint(c.explicit, c.host, c.wan, c.lan); got != c.want {
			t.Errorf("%s: ovpnRemoteEndpoint = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestValidPublicHostRe(t *testing.T) {
	valid := []string{"casa.duckdns.org", "rentals.cloudless.club", "vpn.example.com", "203.0.113.4", "a", "host-1.example.net"}
	for _, h := range valid {
		if !validPublicHostRe.MatchString(h) {
			t.Errorf("host %q should be valid", h)
		}
	}
	invalid := []string{"", " leading.space", "trailing ", "with space.example.com", "semi;injected", "a\nb", "-starts-with-dash", "http://x", "user@host"}
	for _, h := range invalid {
		if validPublicHostRe.MatchString(h) {
			t.Errorf("host %q should be invalid", h)
		}
	}
}
