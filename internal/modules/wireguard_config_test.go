package modules

import (
	"strings"
	"testing"
)

// The profile NetGrip hands to the official clients (#659): every mandatory
// line present, a fixed ListenPort and the DNS pointing at the tunnel.
func TestBuildWGClientConfigHasEveryLine(t *testing.T) {
	got := buildWGClientConfig(wgClientConfig{
		PrivateKey:   "MOX0kFhNYQwqegTPfjC+RKGbwMcuiZzwkFUkZn4t0E4=",
		Address:      "10.66.0.2/32",
		DNS:          "10.66.0.1",
		ServerKey:    "fVOXQebJWhXsIHb/E9Rt2VqoHhIDiNx1xxGGxqA6R0o=",
		PresharedKey: "rvMZHh+Y/l+q+rnQcNEC7lqzoyn1Q1TjsbrcnNMRHKo=",
		Endpoint:     "hub.cloudless.club",
		Port:         "51820",
	})
	want := `[Interface]
PrivateKey = MOX0kFhNYQwqegTPfjC+RKGbwMcuiZzwkFUkZn4t0E4=
Address = 10.66.0.2/32
DNS = 10.66.0.1
ListenPort = 51820

[Peer]
PublicKey = fVOXQebJWhXsIHb/E9Rt2VqoHhIDiNx1xxGGxqA6R0o=
PresharedKey = rvMZHh+Y/l+q+rnQcNEC7lqzoyn1Q1TjsbrcnNMRHKo=
AllowedIPs = 0.0.0.0/0
Endpoint = hub.cloudless.club:51820
PersistentKeepalive = 25
`
	if got != want {
		t.Fatalf("profile mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatal("profile must end with a newline: qrencode and the apps rely on it")
	}
}

// A missing preshared key, DNS or endpoint drops only that line.
func TestBuildWGClientConfigOmitsEmptyOptions(t *testing.T) {
	got := buildWGClientConfig(wgClientConfig{
		PrivateKey: "priv",
		Address:    "10.66.0.3/32",
		ServerKey:  "pub",
		Port:       "",
	})
	for _, unwanted := range []string{"PresharedKey", "DNS", "Endpoint"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("%s must be left out when empty:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "ListenPort = 51820") {
		t.Fatalf("ListenPort is mandatory for a fixed client port:\n%s", got)
	}
}

// The endpoint port falls back to the default when UCI has none.
func TestBuildWGClientConfigDefaultPort(t *testing.T) {
	got := buildWGClientConfig(wgClientConfig{
		PrivateKey: "priv",
		Address:    "10.66.0.4/32",
		ServerKey:  "pub",
		Endpoint:   "1.2.3.4",
	})
	if !strings.Contains(got, "Endpoint = 1.2.3.4:51820\n") {
		t.Fatalf("missing default port:\n%s", got)
	}
}

// The DNS line uses the server's tunnel address, not the client's.
func TestWGClientDNS(t *testing.T) {
	probe := &WGProbe{Address: "10.66.0.1/24"}
	if got := wgClientDNS(probe); got != "10.66.0.1" {
		t.Fatalf("dns = %q, want the tunnel address", got)
	}
	if got := wgClientDNS(&WGProbe{}); got != "" {
		t.Fatalf("no address must mean no DNS line, got %q", got)
	}
}
