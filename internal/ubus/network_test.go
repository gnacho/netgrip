package ubus

import (
	"encoding/json"
	"testing"
)

// Real ubus payload shape from a multi-WAN router: "isp" is the active
// PPPoE uplink, "wan" is a QMI failover interface that exists but is down
// (empty dns-server, no route).
const dumpActiveUplinkNotNamedWan = `{
	"interface": [
		{
			"interface": "loopback",
			"up": true,
			"l3_device": "lo"
		},
		{
			"interface": "lan",
			"up": true,
			"l3_device": "br-lan",
			"ipv4-address": [
				{"address": "192.0.2.1", "mask": 24}
			]
		},
		{
			"interface": "wan",
			"up": false,
			"available": true,
			"dns-server": []
		},
		{
			"interface": "isp",
			"up": true,
			"uptime": 989739,
			"l3_device": "pppoe-isp",
			"proto": "pppoe",
			"ipv4-address": [
				{"address": "203.0.113.116", "mask": 32}
			],
			"route": [
				{"target": "0.0.0.0", "mask": 0, "nexthop": "203.0.113.1"}
			],
			"dns-server": ["198.51.100.53", "198.51.100.54"]
		}
	]
}`

func TestBuildWanStatusPicksActiveNonWanNamedInterface(t *testing.T) {
	status, err := buildWanStatus([]byte(dumpActiveUplinkNotNamedWan))
	if err != nil {
		t.Fatalf("buildWanStatus: %v", err)
	}
	if !status.Present || !status.Up {
		t.Fatalf("status = %+v, want present+up", status)
	}
	if len(status.IPv4) != 1 || status.IPv4[0] != "203.0.113.116" {
		t.Fatalf("IPv4 = %v, want [203.0.113.116]", status.IPv4)
	}
	if status.Gateway != "203.0.113.1" {
		t.Fatalf("Gateway = %q, want 203.0.113.1", status.Gateway)
	}
	if len(status.DNS) != 2 || status.DNS[0] != "198.51.100.53" {
		t.Fatalf("DNS = %v, want [198.51.100.53 198.51.100.54]", status.DNS)
	}
}

// Single-WAN router, uplink temporarily down: must still report
// present+down, not "absent", even though nothing has a default route.
const dumpSingleWanDown = `{
	"interface": [
		{"interface": "loopback", "up": true},
		{"interface": "lan", "up": true, "l3_device": "br-lan"},
		{"interface": "wan", "up": false, "dns-server": []}
	]
}`

func TestBuildWanStatusFallsBackToNamedWanWhenNoneActive(t *testing.T) {
	status, err := buildWanStatus([]byte(dumpSingleWanDown))
	if err != nil {
		t.Fatalf("buildWanStatus: %v", err)
	}
	if !status.Present {
		t.Fatalf("status = %+v, want present", status)
	}
	if status.Up {
		t.Fatalf("status = %+v, want down", status)
	}
	if status.DNS == nil || len(status.DNS) != 0 {
		t.Fatalf("DNS = %v, want non-nil empty slice", status.DNS)
	}
}

// Both interfaces report up (the QMI
// modem reconnected) but only "isp" actually carries the default route -
// "wan" is up with an empty route/dns-server, same shape as a link that's
// merely connected but not selected as the active gateway.
const dumpBothUpOnlyOneRouted = `{
	"interface": [
		{"interface": "loopback", "up": true},
		{"interface": "lan", "up": true, "l3_device": "br-lan"},
		{
			"interface": "wan",
			"up": true,
			"proto": "qmi",
			"route": [],
			"dns-server": []
		},
		{
			"interface": "isp",
			"up": true,
			"proto": "pppoe",
			"ipv4-address": [{"address": "203.0.113.116", "mask": 32}],
			"route": [
				{"target": "0.0.0.0", "mask": 0, "nexthop": "198.51.100.1"}
			],
			"dns-server": ["198.51.100.53", "198.51.100.54"]
		}
	]
}`

func TestBuildWanStatusPrefersRoutedOverMerelyUpInterface(t *testing.T) {
	status, err := buildWanStatus([]byte(dumpBothUpOnlyOneRouted))
	if err != nil {
		t.Fatalf("buildWanStatus: %v", err)
	}
	if !status.Present || !status.Up {
		t.Fatalf("status = %+v, want present+up", status)
	}
	if status.Gateway != "198.51.100.1" {
		t.Fatalf("Gateway = %q, want 198.51.100.1 (isp, not the merely-up wan)", status.Gateway)
	}
	if len(status.IPv4) != 1 || status.IPv4[0] != "203.0.113.116" {
		t.Fatalf("IPv4 = %v, want [203.0.113.116]", status.IPv4)
	}
}

func TestActiveWANInterfaceNamePrefersRoutedOverMerelyUpInterface(t *testing.T) {
	var dump interfaceDump
	if err := json.Unmarshal([]byte(dumpBothUpOnlyOneRouted), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	iface, ok := pickWanInterface(dump.Interface)
	if !ok || iface.Interface != "isp" {
		t.Fatalf("pickWanInterface = %+v, ok=%v, want isp", iface, ok)
	}
}

// A config where PPPoE "isp" on physical port "lan1" carries the
// default route; "wan" is the QMI cellular modem (device is a /dev node,
// not an Ethernet port at all).
const dumpUplinkOnLanPortWanIsCellular = `{
	"interface": [
		{"interface": "loopback", "up": true, "device": "lo"},
		{"interface": "lan", "up": true, "device": "br-lan"},
		{"interface": "wan", "up": true, "proto": "qmi", "device": "/dev/cdc-wdm0", "route": [], "dns-server": []},
		{
			"interface": "isp",
			"up": true,
			"proto": "pppoe",
			"device": "lan1",
			"route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "198.51.100.1"}],
			"dns-server": ["198.51.100.53"]
		}
	]
}`

func TestActiveWANDevicePicksPhysicalPortOverCellular(t *testing.T) {
	dev := parseAndActiveWANDevice(t, dumpUplinkOnLanPortWanIsCellular)
	if dev != "lan1" {
		t.Fatalf("ActiveWANDevice-equivalent = %q, want lan1", dev)
	}
}

func parseAndActiveWANDevice(t *testing.T, raw string) string {
	t.Helper()
	var dump interfaceDump
	if err := json.Unmarshal([]byte(raw), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	iface, ok := pickWanInterface(dump.Interface)
	if !ok {
		t.Fatalf("pickWanInterface: no match")
	}
	return iface.Device
}

// Dumb AP: no wan-named interface and nothing with a default route.
const dumpApOnly = `{
	"interface": [
		{"interface": "loopback", "up": true},
		{"interface": "lan", "up": true, "l3_device": "br-lan"}
	]
}`

func TestBuildWanStatusAbsentOnAP(t *testing.T) {
	status, err := buildWanStatus([]byte(dumpApOnly))
	if err != nil {
		t.Fatalf("buildWanStatus: %v", err)
	}
	if status.Present {
		t.Fatalf("status = %+v, want absent", status)
	}
}

// Two uplinks connected at once, each with a default route: a fibre line at
// metric 10 and a backup at metric 20. The kernel routes over the lower
// metric, so that is "the WAN" whichever order the dump lists them in.
const dumpTwoUplinksByMetric = `{
	"interface": [
		{
			"interface": "backup",
			"up": true,
			"proto": "dhcp",
			"l3_device": "eth2",
			"metric": 20,
			"ipv4-address": [{"address": "198.51.100.20", "mask": 24}],
			"route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "198.51.100.1", "metric": 20}],
			"dns-server": ["198.51.100.1"]
		},
		{
			"interface": "fiber",
			"up": true,
			"proto": "pppoe",
			"device": "lan4",
			"l3_device": "pppoe-fiber",
			"metric": 10,
			"ipv4-address": [{"address": "203.0.113.10", "mask": 32}],
			"route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "203.0.113.1", "metric": 10}],
			"dns-server": ["203.0.113.53"]
		}
	]
}`

func TestPickWanInterfacePrefersLowestMetric(t *testing.T) {
	var dump interfaceDump
	if err := json.Unmarshal([]byte(dumpTwoUplinksByMetric), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Both orderings must give the same answer: the dump order is not the
	// routing decision.
	reversed := []interfaceStatus{dump.Interface[1], dump.Interface[0]}
	for name, ifaces := range map[string][]interfaceStatus{
		"as dumped": dump.Interface,
		"reversed":  reversed,
	} {
		iface, ok := pickWanInterface(ifaces)
		if !ok || iface.Interface != "fiber" {
			t.Fatalf("%s: picked %q (ok=%v), want fiber", name, iface.Interface, ok)
		}
	}
}

// A modem uplink: the configurable interface holds neither address nor
// route, because netifd spawns a dynamic child that holds both. Callers
// read and write network.<name>, and the child has no such section, so the
// parent's name is the only usable answer.
const dumpModemWithDynamicChild = `{
	"interface": [
		{
			"interface": "cell",
			"up": true,
			"proto": "qmi",
			"l3_device": "wwan0",
			"metric": 20,
			"dns-server": []
		},
		{
			"interface": "cell_4",
			"up": true,
			"proto": "dhcp",
			"device": "wwan0",
			"l3_device": "wwan0",
			"metric": 20,
			"dynamic": true,
			"ipv4-address": [{"address": "192.0.2.77", "mask": 32}],
			"route": [{"target": "0.0.0.0", "mask": 0, "nexthop": "192.0.2.78", "metric": 20}],
			"dns-server": ["192.0.2.53"]
		}
	]
}`

func TestPickWanInterfaceResolvesDynamicChildToParent(t *testing.T) {
	var dump interfaceDump
	if err := json.Unmarshal([]byte(dumpModemWithDynamicChild), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	iface, ok := pickWanInterface(dump.Interface)
	if !ok || iface.Interface != "cell" {
		t.Fatalf("picked %q (ok=%v), want the parent cell", iface.Interface, ok)
	}
	// The child's facts have to come with it, or the status card goes blank.
	if len(iface.IPv4) != 1 || iface.IPv4[0].Address != "192.0.2.77" {
		t.Fatalf("IPv4 = %+v, want the child's address", iface.IPv4)
	}
	if iface.defaultGateway() != "192.0.2.78" {
		t.Fatalf("gateway = %q, want the child's", iface.defaultGateway())
	}
}

func TestBuildWanStatusUnchangedOnExistingFixtures(t *testing.T) {
	// The metric and parent rules must not move the answer on any payload
	// seen before them.
	for _, tc := range []struct {
		name        string
		dump        string
		present, up bool
		ip, gateway string
	}{
		{"active uplink not named wan", dumpActiveUplinkNotNamedWan, true, true, "203.0.113.116", "203.0.113.1"},
		{"single uplink down", dumpSingleWanDown, true, false, "", ""},
		{"both up, only one routed", dumpBothUpOnlyOneRouted, true, true, "203.0.113.116", "198.51.100.1"},
		{"uplink on a lan-named port", dumpUplinkOnLanPortWanIsCellular, true, true, "", "198.51.100.1"},
		{"access point, no uplink", dumpApOnly, false, false, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, err := buildWanStatus([]byte(tc.dump))
			if err != nil {
				t.Fatalf("buildWanStatus: %v", err)
			}
			if status.Present != tc.present || status.Up != tc.up {
				t.Fatalf("present=%v up=%v, want %v/%v", status.Present, status.Up, tc.present, tc.up)
			}
			got := ""
			if len(status.IPv4) > 0 {
				got = status.IPv4[0]
			}
			if got != tc.ip || status.Gateway != tc.gateway {
				t.Fatalf("ip=%q gw=%q, want %q/%q", got, status.Gateway, tc.ip, tc.gateway)
			}
		})
	}
}

func TestParseInterfaceDumpKeepsUplinkFields(t *testing.T) {
	states, err := ParseInterfaceDump([]byte(dumpModemWithDynamicChild))
	if err != nil {
		t.Fatalf("ParseInterfaceDump: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("got %d interfaces, want 2", len(states))
	}
	parent, child := states[0], states[1]
	if parent.Proto != "qmi" || parent.L3Device != "wwan0" || parent.Dynamic {
		t.Fatalf("parent = %+v", parent)
	}
	if parent.HasDefaultRoute {
		t.Fatalf("the parent of a modem uplink has no route of its own: %+v", parent)
	}
	if !child.Dynamic || !child.HasDefaultRoute || child.RouteMetric != 20 {
		t.Fatalf("child = %+v", child)
	}
	if len(child.IPv4) != 1 || child.IPv4[0] != "192.0.2.77" {
		t.Fatalf("child IPv4 = %v", child.IPv4)
	}
}

// A policy manager can send traffic through an uplink that does not hold
// the cheapest route. When it says so, that is the WAN: the address the
// world sees is that one's, and showing the other is simply wrong.
func TestPickWanInterfaceFollowsAnExternalAnswer(t *testing.T) {
	var dump interfaceDump
	if err := json.Unmarshal([]byte(dumpTwoUplinksByMetric), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	iface, ok := pickWanInterfacePreferring(dump.Interface, "backup")
	if !ok || iface.Interface != "backup" {
		t.Fatalf("picked %q, want the uplink the policy names", iface.Interface)
	}
	if iface.defaultGateway() != "198.51.100.1" {
		t.Fatalf("gateway = %q, want the named uplink's", iface.defaultGateway())
	}
	// An answer naming something that is not there falls back to the route
	// table rather than reporting no WAN at all.
	if iface, ok := pickWanInterfacePreferring(dump.Interface, "gone"); !ok || iface.Interface != "fiber" {
		t.Fatalf("picked %q, want the routed uplink as a fallback", iface.Interface)
	}
}

// A modem uplink keeps its address on a runtime child, so naming the parent
// has to bring those facts along or the card shows a blank connection.
func TestPreferredModemUplinkCarriesItsChildsAddress(t *testing.T) {
	var dump interfaceDump
	if err := json.Unmarshal([]byte(dumpModemWithDynamicChild), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	status, err := buildWanStatusPreferring([]byte(dumpModemWithDynamicChild), "cell")
	if err != nil {
		t.Fatalf("buildWanStatus: %v", err)
	}
	if len(status.IPv4) != 1 || status.IPv4[0] != "192.0.2.77" {
		t.Fatalf("IPv4 = %v, want the address traffic actually leaves from", status.IPv4)
	}
	if status.Gateway != "192.0.2.78" || len(status.DNS) != 1 {
		t.Fatalf("status = %+v", status)
	}
}
