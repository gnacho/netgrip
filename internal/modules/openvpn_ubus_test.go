package modules

import "testing"

// Real ubus payloads captured from the routers.

const ubusWanPPPoE = `{
	"uptime": 989739,
	"l3_device": "pppoe-wan",
	"proto": "pppoe",
	"up": true,
	"delegation": true,
	"ipv4-address": [
		{
			"address": "79.112.56.116",
			"mask": 32,
			"ptpaddr": "79.112.56.1"
		}
	],
	"ipv6-address": [
	],
	"inactive": {
		"ipv4-address": [
		],
		"ipv6-address": [
		]
	}
}`

const ubusLan = `{
	"uptime": 989730,
	"l3_device": "br-lan",
	"proto": "static",
	"up": true,
	"ipv4-address": [
		{
			"address": "192.168.1.1",
			"mask": 24
		}
	]
}`

// Dumb AP: address learned via DHCP, ubus reports no static ipv4-address.
const ubusApNoAddress = `{
	"uptime": 1234,
	"l3_device": "br-lan",
	"proto": "none",
	"up": true,
	"ipv4-address": [
	]
}`

func TestParseUbusIPv4WanPPPoE(t *testing.T) {
	if got := parseUbusIPv4(ubusWanPPPoE); got != "79.112.56.116" {
		t.Fatalf("parseUbusIPv4(wan) = %q, want 79.112.56.116", got)
	}
}

func TestParseUbusIPv4Lan(t *testing.T) {
	if got := parseUbusIPv4(ubusLan); got != "192.168.1.1" {
		t.Fatalf("parseUbusIPv4(lan) = %q, want 192.168.1.1", got)
	}
}

func TestParseUbusIPv4ApEmpty(t *testing.T) {
	if got := parseUbusIPv4(ubusApNoAddress); got != "" {
		t.Fatalf("parseUbusIPv4(ap) = %q, want empty", got)
	}
}

func TestParseUbusIPv4Garbage(t *testing.T) {
	for _, in := range []string{"", "not json", `{"ipv4-address": [`} {
		if got := parseUbusIPv4(in); got != "" {
			t.Fatalf("parseUbusIPv4(%q) = %q, want empty", in, got)
		}
	}
}

// Regression: the old grep pipeline returned the "ipv4-address" key line
// itself for this payload; the JSON parser must return the real address.
func TestParseUbusIPv4KeyLineRegression(t *testing.T) {
	if got := parseUbusIPv4(ubusWanPPPoE); got == "ipv4-address" || got == "" {
		t.Fatalf("parseUbusIPv4 regression: got %q", got)
	}
}

func TestLanRouteFromUbus(t *testing.T) {
	if got := lanRouteFromUbus(ubusLan); got != "192.168.1.0 255.255.255.0" {
		t.Fatalf("lanRouteFromUbus(lan) = %q, want 192.168.1.0 255.255.255.0", got)
	}
}

func TestLanRouteFromUbusNon24(t *testing.T) {
	in := `{"ipv4-address": [{"address": "10.30.0.1", "mask": 16}]}`
	if got := lanRouteFromUbus(in); got != "10.30.0.0 255.255.0.0" {
		t.Fatalf("lanRouteFromUbus(/16) = %q, want 10.30.0.0 255.255.0.0", got)
	}
}

func TestLanRouteFromUbusApEmpty(t *testing.T) {
	if got := lanRouteFromUbus(ubusApNoAddress); got != "" {
		t.Fatalf("lanRouteFromUbus(ap) = %q, want empty", got)
	}
}

func TestLanRouteFromUbusGarbage(t *testing.T) {
	for _, in := range []string{"", "not json", `{"ipv4-address": [{"address": "nope"}]}`} {
		if got := lanRouteFromUbus(in); got != "" {
			t.Fatalf("lanRouteFromUbus(%q) = %q, want empty", in, got)
		}
	}
}
