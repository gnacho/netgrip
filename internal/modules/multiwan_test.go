package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// A router with a fibre uplink and a cellular backup, behind a firewall zone
// that is NOT called "wan" — and with a decoy interface that IS called "wan"
// sitting on the LAN. Anything that keys on the name gets this wrong.
const netShowTwoUplinks = `network.loopback=interface
network.loopback.proto='static'
network.loopback.device='lo'
network.lan=interface
network.lan.proto='static'
network.lan.device='br-home.10'
network.wan=interface
network.wan.proto='static'
network.wan.device='br-home.20'
network.fiber=interface
network.fiber.proto='pppoe'
network.fiber.device='lan4'
network.cell=interface
network.cell.proto='qmi'
network.cell.device='/dev/cdc-wdm0'
network.tunnel=interface
network.tunnel.proto='wireguard'
`

const fwShowEdgeZone = `firewall.@zone[0]=zone
firewall.@zone[0].name='local'
firewall.@zone[0].network='lan' 'wan'
firewall.@zone[0].forward='ACCEPT'
firewall.@zone[1]=zone
firewall.@zone[1].name='edge'
firewall.@zone[1].masq='1'
firewall.@zone[1].forward='REJECT'
firewall.@zone[1].network='fiber'
firewall.@zone[1].network='fiber6'
firewall.@zone[1].network='cell'
firewall.@zone[2]=zone
firewall.@zone[2].name='vpn'
firewall.@zone[2].masq='1'
firewall.@zone[2].forward='REJECT'
firewall.@zone[2].network='tunnel'
`

// The dump that goes with it: fibre routed at metric 10, the modem parent
// holding nothing while its runtime child holds the address and the route,
// and the IPv6 side of the fibre link as a second child.
func dumpTwoUplinks() []ubus.InterfaceState {
	return []ubus.InterfaceState{
		{Name: "lan", Proto: "static", Device: "br-home.10", L3Device: "br-home.10", Up: true, IPv4: []string{"192.0.2.1"}},
		{Name: "wan", Proto: "static", Device: "br-home.20", L3Device: "br-home.20", Up: true, IPv4: []string{"198.51.100.1"}},
		{
			Name: "fiber", Proto: "pppoe", Device: "lan4", L3Device: "pppoe-fiber", Up: true,
			Metric: 10, RouteMetric: 10, HasDefaultRoute: true, Gateway: "203.0.113.1",
			IPv4: []string{"203.0.113.10"}, Uptime: 4000,
		},
		{Name: "fiber6", Proto: "dhcpv6", L3Device: "pppoe-fiber", Up: true, Dynamic: true},
		{Name: "cell", Proto: "qmi", L3Device: "wwan0", Up: true, Metric: 20},
		{
			Name: "cell_4", Proto: "dhcp", Device: "wwan0", L3Device: "wwan0", Up: true, Dynamic: true,
			Metric: 20, RouteMetric: 20, HasDefaultRoute: true, Gateway: "192.0.2.78",
			IPv4: []string{"192.0.2.77"}, Uptime: 900,
		},
		{Name: "tunnel", Proto: "wireguard", L3Device: "wg0", Up: true},
	}
}

func classifyFixture(t *testing.T) []WanCandidate {
	t.Helper()
	return classifyWanCandidates(
		parseUCIShow(netShowTwoUplinks, "network"),
		parseFWZonesFrom(fwShowEdgeZone),
		dumpTwoUplinks(),
		[]string{"br-home.10", "br-home"},
	)
}

func byName(cs []WanCandidate, name string) *WanCandidate {
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i]
		}
	}
	return nil
}

func TestClassifyFindsBothUplinksAndIgnoresTheDecoy(t *testing.T) {
	got := classifyFixture(t)
	if len(got) != 2 {
		t.Fatalf("got %d candidates %v, want fiber and cell", len(got), names(got))
	}
	if byName(got, "wan") != nil {
		t.Fatalf("the interface named wan is on the LAN here and must not be an uplink: %v", names(got))
	}
	if byName(got, "tunnel") != nil {
		t.Fatalf("a wireguard interface in a masquerading zone is not an uplink: %v", names(got))
	}
	if byName(got, "lan") != nil {
		t.Fatalf("the LAN is not an uplink: %v", names(got))
	}
}

func TestClassifyDropsTheV6SiblingAndRuntimeChildren(t *testing.T) {
	for _, c := range classifyFixture(t) {
		if strings.HasSuffix(c.Name, "6") || strings.HasSuffix(c.Name, "_4") {
			t.Fatalf("a runtime child leaked into the list: %q", c.Name)
		}
	}
}

func TestClassifyFoldsTheModemChildIntoItsParent(t *testing.T) {
	cell := byName(classifyFixture(t), "cell")
	if cell == nil {
		t.Fatal("the modem uplink is missing")
	}
	// The parent has no address of its own; without folding the row would
	// show a connected link with nothing to identify it by.
	if len(cell.IPv4) != 1 || cell.IPv4[0] != "192.0.2.77" {
		t.Fatalf("IPv4 = %v, want the child's address", cell.IPv4)
	}
	if cell.Gateway != "192.0.2.78" || cell.Uptime != 900 {
		t.Fatalf("gateway=%q uptime=%d, want the child's", cell.Gateway, cell.Uptime)
	}
	if !cell.Metered {
		t.Fatal("a mobile-broadband uplink must be marked metered")
	}
	if cell.Port != "" {
		t.Fatalf("Port = %q, a modem has no socket", cell.Port)
	}
}

func TestClassifyMarksOnlyTheCheapestRouteActive(t *testing.T) {
	got := classifyFixture(t)
	fiber, cell := byName(got, "fiber"), byName(got, "cell")
	if !fiber.Active {
		t.Fatalf("the metric-10 uplink should be the active one: %+v", fiber)
	}
	if cell.Active {
		t.Fatalf("the metric-20 uplink holds a route but is not carrying traffic: %+v", cell)
	}
	if !cell.Up {
		t.Fatal("the modem is up through its child")
	}
	if fiber.Port != "lan4" {
		t.Fatalf("Port = %q, want the physical socket", fiber.Port)
	}
}

// A VLAN-tagged uplink reports the port with its tag; the socket is the port.
func TestPhysicalPortStripsVlanAndSkipsNonPorts(t *testing.T) {
	for in, want := range map[string]string{
		"lan1":          "lan1",
		"lan1.7":        "lan1",
		"/dev/cdc-wdm0": "",
		"br-home.10":    "",
		"":              "",
	} {
		if got := physicalPort(in); got != want {
			t.Fatalf("physicalPort(%q) = %q, want %q", in, got, want)
		}
	}
}

// An uplink in no zone and with no route still counts when it is marked by
// hand — the escape hatch for a setup nothing else recognises.
func TestClassifyHonoursTheManualMarker(t *testing.T) {
	netShow := `network.spare=interface
network.spare.proto='dhcp'
network.spare.device='lan3'
network.spare.` + mwanWanMarker + `='1'
`
	got := classifyWanCandidates(parseUCIShow(netShow, "network"), nil,
		[]ubus.InterfaceState{{Name: "spare", Proto: "dhcp", Device: "lan3", L3Device: "lan3"}},
		[]string{"br-home"})
	if len(got) != 1 || got[0].Reason != "marker" {
		t.Fatalf("got %+v, want one candidate qualified by the marker", got)
	}
}

func TestClassifyOnAccessPointFindsNothing(t *testing.T) {
	netShow := "network.lan=interface\nnetwork.lan.proto='static'\nnetwork.lan.device='br-home'\n"
	got := classifyWanCandidates(parseUCIShow(netShow, "network"), nil,
		[]ubus.InterfaceState{{Name: "lan", Proto: "static", Device: "br-home", L3Device: "br-home", Up: true}},
		[]string{"br-home"})
	if len(got) != 0 {
		t.Fatalf("got %v, want nothing on an access point", names(got))
	}
}

func names(cs []WanCandidate) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

// ---------------------------------------------------------------------------
// mwan3 config
// ---------------------------------------------------------------------------

// A hand-written config of the kind found on a router that was set up
// before netgrip: a working failover, none of it prefixed.
const mwanShowForeign = `mwan3.globals=globals
mwan3.globals.mmx_mask='0x3F00'
mwan3.cell=interface
mwan3.cell.enabled='1'
mwan3.cell.track_ip='198.51.100.10'
mwan3.cell6=interface
mwan3.cell6.enabled='0'
mwan3.fiber=interface
mwan3.fiber.enabled='1'
mwan3.fiber.track_ip='198.51.100.10' '198.51.100.11'
mwan3.fiber_m1=member
mwan3.fiber_m1.interface='fiber'
mwan3.fiber_m1.metric='1'
mwan3.fiber_m1.weight='1'
mwan3.cell_m2=member
mwan3.cell_m2.interface='cell'
mwan3.cell_m2.metric='2'
mwan3.cell_m2.weight='1'
mwan3.fiber_failover=policy
mwan3.fiber_failover.last_resort='unreachable'
mwan3.default_v4=rule
mwan3.default_v4.dest_ip='0.0.0.0/0'
mwan3.default_v4.use_policy='fiber_failover'
`

func TestReadMwanConfigReportsAHandWrittenSetupAsForeign(t *testing.T) {
	cfg := readMwanConfig(mwanShowForeign)
	if cfg.Managed {
		t.Fatal("nothing here was written by netgrip")
	}
	if cfg.Mode != MWModeCustom {
		t.Fatalf("Mode = %q, want custom — it works, it is just not one of our two shapes", cfg.Mode)
	}
	want := []string{"cell_m2", "default_v4", "fiber_failover", "fiber_m1"}
	if strings.Join(cfg.Foreign, ",") != strings.Join(want, ",") {
		t.Fatalf("Foreign = %v, want %v (interface sections are adoptable, not listed)", cfg.Foreign, want)
	}
	if ifc := cfg.Ifaces["fiber"]; len(ifc.Track) != 2 || ifc.Ours {
		t.Fatalf("fiber interface = %+v", ifc)
	}
	if cfg.Ifaces["cell6"].Enabled {
		t.Fatal("a section with enabled='0' is not enabled")
	}
}

func TestReadMwanConfigRecognisesItsOwnFailover(t *testing.T) {
	show := `mwan3.fiber=interface
mwan3.fiber.enabled='1'
mwan3.fiber.` + mwanManagedOpt + `='1'
mwan3.cell=interface
mwan3.cell.enabled='1'
mwan3.cell.` + mwanManagedOpt + `='1'
mwan3.` + mwanMemberPrefix + `fiber=member
mwan3.` + mwanMemberPrefix + `fiber.interface='fiber'
mwan3.` + mwanMemberPrefix + `fiber.metric='1'
mwan3.` + mwanMemberPrefix + `cell=member
mwan3.` + mwanMemberPrefix + `cell.interface='cell'
mwan3.` + mwanMemberPrefix + `cell.metric='2'
mwan3.` + mwanPolicyName + `=policy
mwan3.` + mwanRuleName + `=rule
`
	cfg := readMwanConfig(show)
	if !cfg.Managed || len(cfg.Foreign) != 0 {
		t.Fatalf("managed=%v foreign=%v, want fully ours", cfg.Managed, cfg.Foreign)
	}
	if cfg.Mode != MWModeFailover || cfg.Primary != "fiber" {
		t.Fatalf("mode=%q primary=%q, want failover/fiber", cfg.Mode, cfg.Primary)
	}
}

func TestMwanModeFromMetrics(t *testing.T) {
	member := func(iface string, metric int) mwanMember {
		return mwanMember{Interface: iface, Metric: metric, Weight: 1, Ours: true}
	}
	for name, tc := range map[string]struct {
		members    map[string]mwanMember
		mode, prim string
	}{
		"nothing configured": {map[string]mwanMember{}, MWModeOff, ""},
		"distinct metrics are an order of preference": {
			map[string]mwanMember{"a": member("fiber", 1), "b": member("cell", 2)}, MWModeFailover, "fiber"},
		"a shared lowest metric is a shared load": {
			map[string]mwanMember{"a": member("fiber", 1), "b": member("dsl", 1)}, MWModeBalance, ""},
		"two sharing plus a standby tail is still balancing": {
			map[string]mwanMember{"a": member("fiber", 1), "b": member("dsl", 1), "c": member("cell", 2)},
			MWModeBalance, ""},
	} {
		t.Run(name, func(t *testing.T) {
			mode, prim := mwanModeFrom(tc.members)
			if mode != tc.mode || prim != tc.prim {
				t.Fatalf("got %q/%q, want %q/%q", mode, prim, tc.mode, tc.prim)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// mwan3 runtime output
// ---------------------------------------------------------------------------

const mwanInterfacesOut = `Interface status:
 interface cell is online and tracking is active (online 00h:15m:20s, uptime 00h:15m:22s)
 interface cell6 is unknown and tracking is down (31)
 interface fiber is online and tracking is active (online 32h:46m:33s, uptime 32h:46m:36s)
`

func TestParseMwanInterfaces(t *testing.T) {
	got := parseMwanInterfaces(mwanInterfacesOut)
	if len(got) != 3 {
		t.Fatalf("got %d interfaces, want 3: %+v", len(got), got)
	}
	if got["fiber"].Online != "online" || got["fiber"].Tracking != "active" {
		t.Fatalf("fiber = %+v", got["fiber"])
	}
	if got["cell6"].Online != "unknown" || got["cell6"].Tracking != "down" {
		t.Fatalf("cell6 = %+v", got["cell6"])
	}
	if len(parseMwanInterfaces("something else entirely\n")) != 0 {
		t.Fatal("an unparsable line must be skipped, not guessed at")
	}
}

const mwanPoliciesOut = `Current ipv4 policies:
fiber_failover:
 fiber (100%)

Current ipv6 policies:
fiber_failover:
 unreachable

`

func TestParseMwanPoliciesReadsTheV4Distribution(t *testing.T) {
	policy, shares := parseMwanPolicies(mwanPoliciesOut)
	if policy != "fiber_failover" {
		t.Fatalf("policy = %q", policy)
	}
	if len(shares) != 1 || shares["fiber"] != 100 {
		t.Fatalf("shares = %v, want fiber at 100", shares)
	}
}

func TestParseMwanPoliciesSplitsABalancedPair(t *testing.T) {
	out := `Current ipv4 policies:
netgrip_policy_default:
 fiber (75%)
 dsl (25%)

Current ipv6 policies:
netgrip_policy_default:
 unreachable
`
	_, shares := parseMwanPolicies(out)
	if shares["fiber"] != 75 || shares["dsl"] != 25 {
		t.Fatalf("shares = %v", shares)
	}
}

// ---------------------------------------------------------------------------
// The assembled probe
// ---------------------------------------------------------------------------

func TestProbeWithoutThePackageStillListsTheUplinks(t *testing.T) {
	p := buildMultiWanProbe(classifyFixture(t), false, false, false, false,
		readMwanConfig(""), nil, "", nil)
	if len(p.Candidates) != 2 || !p.MultiWanPossible {
		t.Fatalf("the uplinks exist whether or not mwan3 does: %+v", p)
	}
	if p.Mode != MWModeOff || p.Foreign || p.Managed {
		t.Fatalf("nothing can be configured without the package: %+v", p)
	}
}

// A package removed by hand leaves its config file behind. Offering modes
// for a service that cannot run would be a lie.
func TestProbeIgnoresLeftoverConfigWhenThePackageIsGone(t *testing.T) {
	p := buildMultiWanProbe(classifyFixture(t), false, false, false, true,
		readMwanConfig(mwanShowForeign), nil, "", nil)
	if p.Mode != MWModeOff || p.Foreign {
		t.Fatalf("mode=%q foreign=%v, want the leftovers ignored", p.Mode, p.Foreign)
	}
	if !p.ConfigPresent {
		t.Fatal("the leftover file should still be reported")
	}
}

func TestProbeUsesMwanDistributionOverTheRouteTable(t *testing.T) {
	// The route table says fibre. mwan3 says the modem is carrying it all,
	// which is what actually happens once its marks are in place.
	live := map[string]mwanLive{"fiber": {Online: "offline", Tracking: "active"}, "cell": {Online: "online", Tracking: "active"}}
	p := buildMultiWanProbe(classifyFixture(t), true, true, true, true,
		readMwanConfig(mwanShowForeign), live, "fiber_failover", map[string]int{"cell": 100})
	fiber, cell := byName(p.Candidates, "fiber"), byName(p.Candidates, "cell")
	if fiber.Active {
		t.Fatalf("fiber is not carrying traffic according to mwan3: %+v", fiber)
	}
	if !cell.Active || cell.SharePct != 100 {
		t.Fatalf("cell = %+v, want active at 100%%", cell)
	}
	if fiber.Online != "offline" || cell.Tracking != "active" {
		t.Fatalf("tracker state not carried through: %+v %+v", fiber, cell)
	}
	if !p.Foreign || p.Mode != MWModeCustom {
		t.Fatalf("probe = %+v, want the hand-written config flagged", p)
	}
	// Tracking targets fall back to the defaults only where none are set.
	if len(cell.Track) != 1 || len(fiber.Track) != 2 {
		t.Fatalf("track lists = %v / %v", cell.Track, fiber.Track)
	}
}

// `uci show` prints a whole list on one line, which is how four tracking
// addresses once arrived as a single string with quotes inside it.
func TestTrackListSplitsAOneLineUciList(t *testing.T) {
	cfg := readMwanConfig(mwanShowForeign)
	got := cfg.Ifaces["fiber"].Track
	if len(got) != 2 || got[0] != "198.51.100.10" || got[1] != "198.51.100.11" {
		t.Fatalf("Track = %q, want the two addresses split apart", got)
	}
	if one := cfg.Ifaces["cell"].Track; len(one) != 1 || one[0] != "198.51.100.10" {
		t.Fatalf("a single-value option must survive unchanged: %q", one)
	}
}

// ---------------------------------------------------------------------------
// Planning and op building
// ---------------------------------------------------------------------------

func planFor(t *testing.T, req MultiWanRequest) mwanPlan {
	t.Helper()
	plan, err := buildMwanPlan(req, classifyFixture(t))
	if err != nil {
		t.Fatalf("buildMwanPlan: %v", err)
	}
	return plan
}

func opKeys(ops []executor.Op) []string {
	out := []string{}
	for _, o := range ops {
		out = append(out, o.Kind+" "+strings.Join(o.Args, " "))
	}
	return out
}

func hasOp(ops []executor.Op, want string) bool {
	for _, o := range opKeys(ops) {
		if o == want {
			return true
		}
	}
	return false
}

func TestFailoverPlanOrdersByPreference(t *testing.T) {
	plan := planFor(t, MultiWanRequest{Mode: MWModeFailover, Primary: "cell"})
	if len(plan.Members) != 2 {
		t.Fatalf("members = %+v", plan.Members)
	}
	// Distinct metrics are what make mwan3 treat these as an order rather
	// than a pool, and the chosen one has to come first.
	if plan.Members[0].Iface != "cell" || plan.Members[0].Metric != 1 {
		t.Fatalf("first member = %+v, want the chosen uplink at metric 1", plan.Members[0])
	}
	if plan.Members[1].Iface != "fiber" || plan.Members[1].Metric != 2 {
		t.Fatalf("second member = %+v", plan.Members[1])
	}
	if plan.Sticky {
		t.Fatal("failover has one link at a time; stickiness is meaningless")
	}
}

func TestBalancePlanKeepsAMeteredLinkOutOfThePool(t *testing.T) {
	plan := planFor(t, MultiWanRequest{Mode: MWModeBalance})
	var pooled, fallback []string
	for _, m := range plan.Members {
		if m.Metric == 1 {
			pooled = append(pooled, m.Iface)
		} else {
			fallback = append(fallback, m.Iface)
		}
	}
	if len(pooled) != 1 || pooled[0] != "fiber" {
		t.Fatalf("pooled = %v, want only the unmetered link by default", pooled)
	}
	// Excluded, not discarded: it should still save the house if the pool
	// goes down.
	if len(fallback) != 1 || fallback[0] != "cell" {
		t.Fatalf("fallback = %v, want the metered link kept as a standby", fallback)
	}
	if !plan.Sticky {
		t.Fatal("balanced traffic must stay on the link a flow started on")
	}
}

func TestBalancePlanHonoursWeightsAndOptIn(t *testing.T) {
	plan := planFor(t, MultiWanRequest{
		Mode:    MWModeBalance,
		Balance: map[string]bool{"cell": true},
		Weights: map[string]int{"fiber": 4, "cell": 1},
	})
	got := map[string][2]int{}
	for _, m := range plan.Members {
		got[m.Iface] = [2]int{m.Metric, m.Weight}
	}
	if got["fiber"] != [2]int{1, 4} || got["cell"] != [2]int{1, 1} {
		t.Fatalf("members = %v, want both pooled at 4:1", got)
	}
}

func TestPlanRejectsWhatCannotWork(t *testing.T) {
	cands := classifyFixture(t)
	for name, tc := range map[string]MultiWanRequest{
		"unknown mode":                {Mode: "bonding"},
		"no main connection chosen":   {Mode: MWModeFailover},
		"main connection unknown":     {Mode: MWModeFailover, Primary: "dsl"},
		"main chosen while balancing": {Mode: MWModeBalance, Primary: "fiber"},
		"nothing left to carry it":    {Mode: MWModeBalance, Balance: map[string]bool{"fiber": false, "cell": false}},
		"share out of range":          {Mode: MWModeBalance, Weights: map[string]int{"fiber": 99}},
		"tracking a non-address":      {Mode: MWModeFailover, Primary: "fiber", Track: map[string][]string{"fiber": {"not-an-ip"}}},
		"too many tracking targets": {Mode: MWModeFailover, Primary: "fiber",
			Track: map[string][]string{"fiber": {"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4", "192.0.2.5"}}},
		"tracking an unknown link": {Mode: MWModeFailover, Primary: "fiber", Track: map[string][]string{"dsl": {"192.0.2.1"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := buildMwanPlan(tc, cands); err == nil {
				t.Fatalf("%+v was accepted", tc)
			}
		})
	}
	// And one uplink is not multi-WAN at all.
	if _, err := buildMwanPlan(MultiWanRequest{Mode: MWModeFailover, Primary: "fiber"}, cands[:1]); err == nil {
		t.Fatal("failover with a single uplink was accepted")
	}
}

func TestFailoverOpsWriteTheRightSections(t *testing.T) {
	ops := buildMwanOps(planFor(t, MultiWanRequest{Mode: MWModeFailover, Primary: "fiber"}), readMwanConfig(""), false)
	for _, want := range []string{
		"uci_set mwan3." + mwanMemberPrefix + "fiber.metric 1",
		"uci_set mwan3." + mwanMemberPrefix + "cell.metric 2",
		"uci_set mwan3." + mwanPolicyName + ".last_resort unreachable",
		"uci_add_list mwan3." + mwanPolicyName + ".use_member " + mwanMemberPrefix + "fiber",
		"uci_set mwan3." + mwanRuleName + ".use_policy " + mwanPolicyName,
		"uci_set mwan3." + mwanRuleName + ".dest_ip 0.0.0.0/0",
		"uci_set mwan3.fiber.initial_state online",
		"uci_set mwan3.fiber." + mwanManagedOpt + " 1",
		"uci_add_list mwan3.fiber.track_ip 1.1.1.1",
		"uci_commit mwan3",
		"initd mwan3 restart",
	} {
		if !hasOp(ops, want) {
			t.Fatalf("missing op %q in:\n%s", want, strings.Join(opKeys(ops), "\n"))
		}
	}
}

// The prefix is what tells netgrip's sections from anyone else's. It cannot
// go on interface sections — mwan3 matches those to the network interface by
// name — so those carry a marker instead, and everything else must carry the
// prefix.
func TestEverySectionWeInventIsPrefixed(t *testing.T) {
	ops := buildMwanOps(planFor(t, MultiWanRequest{Mode: MWModeBalance}), readMwanConfig(""), false)
	for _, o := range ops {
		if o.Kind != "uci_set" || len(o.Args) != 2 {
			continue
		}
		section := strings.SplitN(strings.TrimPrefix(o.Args[0], mwanPkg+"."), ".", 2)[0]
		switch o.Args[1] {
		case "member", "policy", "rule":
			if !strings.HasPrefix(section, mwanPrefix) {
				t.Fatalf("section %q of type %q is not prefixed", section, o.Args[1])
			}
		case "interface":
			if strings.HasPrefix(section, mwanPrefix) {
				t.Fatalf("interface section %q must keep the network's own name", section)
			}
		}
	}
}

// Bouncing the network is the one thing that can strand the user behind a
// PPPoE link, so no op may ever do it.
func TestOpsNeverBounceTheNetwork(t *testing.T) {
	for _, mode := range []MultiWanRequest{
		{Mode: MWModeFailover, Primary: "fiber"},
		{Mode: MWModeBalance},
		{Mode: MWModeOff},
	} {
		plan, err := buildMwanPlan(mode, classifyFixture(t))
		if err != nil {
			t.Fatalf("%v: %v", mode.Mode, err)
		}
		for _, o := range buildMwanOps(plan, readMwanConfig(mwanShowForeign), true) {
			if o.Kind == "ifup" || o.Kind == "ifdown" {
				t.Fatalf("%s: %v restarts an interface", mode.Mode, o)
			}
			if o.Kind == "initd" && o.Args[0] != mwanPkg {
				t.Fatalf("%s: %v touches a service other than the manager", mode.Mode, o)
			}
		}
	}
}

// Every op has to survive the executor's allowlist, or it fails at the point
// of no return instead of here.
func TestOpsPassTheExecutorAllowlist(t *testing.T) {
	plan := planFor(t, MultiWanRequest{Mode: MWModeFailover, Primary: "fiber"})
	for _, o := range buildMwanOps(plan, readMwanConfig(mwanShowForeign), true) {
		if err := executor.Validate(o); err != nil {
			t.Fatalf("%v: %v", o, err)
		}
	}
}

// `uci delete` fails on an entry that is not there, and that failure aborts
// the apply, so teardown only removes what exists and only what is ours.
func TestTeardownRemovesOnlyOurSectionsThatExist(t *testing.T) {
	show := `mwan3.fiber=interface
mwan3.fiber.` + mwanManagedOpt + `='1'
mwan3.` + mwanMemberPrefix + `fiber=member
mwan3.` + mwanMemberPrefix + `fiber.interface='fiber'
mwan3.` + mwanPolicyName + `=policy
mwan3.` + mwanRuleName + `=rule
mwan3.someone_elses=member
mwan3.someone_elses.interface='fiber'
`
	ops := opKeys(teardownOps(readMwanConfig(show), false))
	want := []string{
		"uci_delete mwan3." + mwanRuleName,
		"uci_delete mwan3." + mwanPolicyName,
		"uci_delete mwan3." + mwanMemberPrefix + "fiber",
	}
	if strings.Join(ops, "|") != strings.Join(want, "|") {
		t.Fatalf("teardown = %v, want rules then policies then members, ours only", ops)
	}
	for _, o := range ops {
		if strings.Contains(o, "someone_elses") || strings.Contains(o, "mwan3.fiber") {
			t.Fatalf("teardown touched something it does not own: %q", o)
		}
	}
}

// Turning it off leaves mwan3 with nothing to do and stops the service.
func TestOffTearsDownAndStopsTheService(t *testing.T) {
	plan, err := buildMwanPlan(MultiWanRequest{Mode: MWModeOff}, classifyFixture(t))
	if err != nil {
		t.Fatalf("buildMwanPlan: %v", err)
	}
	ops := opKeys(buildMwanOps(plan, readMwanConfig(mwanShowForeign), true))
	last := strings.Join(ops[len(ops)-3:], "|")
	if last != "uci_commit mwan3|initd mwan3 stop|initd mwan3 disable" {
		t.Fatalf("tail = %v", ops)
	}
	for _, o := range ops {
		if strings.Contains(o, mwanPolicyName) || strings.Contains(o, mwanRuleName) {
			if !strings.HasPrefix(o, "uci_delete") {
				t.Fatalf("turning it off must not write policy or rules: %q", o)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Taking over a setup somebody else wrote
// ---------------------------------------------------------------------------

// Without consent nothing of anyone else's is touched, whatever is asked.
func TestTeardownLeavesForeignSectionsAloneByDefault(t *testing.T) {
	ops := opKeys(teardownOps(readMwanConfig(mwanShowForeign), false))
	if len(ops) != 0 {
		t.Fatalf("teardown = %v, want nothing: none of that config is ours", ops)
	}
}

func TestTakeoverRemovesTheOldRulesAndAdoptsTheInterfaces(t *testing.T) {
	cfg := readMwanConfig(mwanShowForeign)
	plan := planFor(t, MultiWanRequest{Mode: MWModeFailover, Primary: "fiber"})
	ops := opKeys(buildMwanOps(plan, cfg, true))

	// The old steering is removed: rule first, then policy, then members.
	for _, want := range []string{
		"uci_delete mwan3.default_v4",
		"uci_delete mwan3.fiber_failover",
		"uci_delete mwan3.fiber_m1",
		"uci_delete mwan3.cell_m2",
	} {
		if !hasOp(buildMwanOps(plan, cfg, true), want) {
			t.Fatalf("missing %q in:\n%s", want, strings.Join(ops, "\n"))
		}
	}
	// The interface sections are rewritten in place, never recreated:
	// recreating one makes mwan3 bring the interface up again.
	for _, o := range ops {
		if o == "uci_delete mwan3.fiber" || o == "uci_delete mwan3.cell" {
			t.Fatalf("an interface section was deleted: %q", o)
		}
	}
	if !hasOp(buildMwanOps(plan, cfg, true), "uci_set mwan3.fiber."+mwanManagedOpt+" 1") {
		t.Fatal("an adopted interface section must be marked as ours")
	}
	// The IPv6 half is switched off rather than deleted, and marked so it
	// can be switched back on later.
	for _, want := range []string{
		"uci_set mwan3.cell6.enabled 0",
		"uci_set mwan3.cell6." + mwanDisabledOpt + " 1",
	} {
		if !hasOp(buildMwanOps(plan, cfg, true), want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestTurningItOffRestoresWhatTheTakeoverDisabled(t *testing.T) {
	show := mwanShowForeign + "mwan3.cell6." + mwanDisabledOpt + "='1'\n"
	plan, err := buildMwanPlan(MultiWanRequest{Mode: MWModeOff}, classifyFixture(t))
	if err != nil {
		t.Fatalf("buildMwanPlan: %v", err)
	}
	ops := buildMwanOps(plan, readMwanConfig(show), true)
	for _, want := range []string{
		"uci_set mwan3.cell6.enabled 1",
		"uci_delete mwan3.cell6." + mwanDisabledOpt,
	} {
		if !hasOp(ops, want) {
			t.Fatalf("missing %q in:\n%s", want, strings.Join(opKeys(ops), "\n"))
		}
	}
	// A section netgrip never disabled is left exactly as it was.
	for _, o := range opKeys(ops) {
		if strings.HasPrefix(o, "uci_set mwan3.fiber.enabled") {
			t.Fatalf("turning it off must not re-enable somebody else's section: %q", o)
		}
	}
}

// mwan3 drops a policy or rule whose section name is over 15 characters,
// logs one warning and carries on. The config then reads correctly and
// steers nothing, which is exactly what happened on a live router.
func TestGeneratedSectionNamesFitMwanLimit(t *testing.T) {
	for _, req := range []MultiWanRequest{
		{Mode: MWModeFailover, Primary: "fiber"},
		{Mode: MWModeBalance},
	} {
		plan, err := buildMwanPlan(req, classifyFixture(t))
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range buildMwanOps(plan, readMwanConfig(""), false) {
			if o.Kind != "uci_set" || len(o.Args) != 2 {
				continue
			}
			section := strings.SplitN(strings.TrimPrefix(o.Args[0], mwanPkg+"."), ".", 2)[0]
			switch o.Args[1] {
			case "policy", "rule":
				if len(section) > mwanMaxSectionLen {
					t.Fatalf("%s section %q is %d chars; mwan3 ignores anything over %d",
						o.Args[1], section, len(section), mwanMaxSectionLen)
				}
			}
		}
	}
}

// A rule with no family is applied to IPv6 as well, where the catch-all
// IPv4 destination is not a valid address and mwan3 rejects the rule.
func TestCatchAllRuleIsPinnedToIPv4(t *testing.T) {
	plan := planFor(t, MultiWanRequest{Mode: MWModeFailover, Primary: "fiber"})
	ops := buildMwanOps(plan, readMwanConfig(""), false)
	if !hasOp(ops, "uci_set mwan3."+mwanRuleName+".family ipv4") {
		t.Fatalf("the catch-all rule must declare its family:\n%s", strings.Join(opKeys(ops), "\n"))
	}
}

// Writing the config is not the same as mwan3 accepting it. The check that
// matters is what it reports loading afterwards.
func TestHealthcheckRejectsAConfigMwanDidNotLoad(t *testing.T) {
	plan := planFor(t, MultiWanRequest{Mode: MWModeFailover, Primary: "fiber"})
	loaded := func(policy string) *MultiWanProbe {
		p := buildMultiWanProbe(classifyFixture(t), true, true, true, true,
			readMwanConfig(`mwan3.`+mwanMemberPrefix+`fiber=member
mwan3.`+mwanMemberPrefix+`fiber.interface='fiber'
mwan3.`+mwanMemberPrefix+`fiber.metric='1'
mwan3.`+mwanMemberPrefix+`cell=member
mwan3.`+mwanMemberPrefix+`cell.interface='cell'
mwan3.`+mwanMemberPrefix+`cell.metric='2'
`), nil, policy, map[string]int{"fiber": 100})
		return p
	}
	if err := mwanHealthcheck(loaded(""), plan); err == nil {
		t.Fatal("a config the manager never loaded must not pass as applied")
	}
	if err := mwanHealthcheck(loaded("somebody_elses"), plan); err == nil {
		t.Fatal("another policy being in force must not pass as applied")
	}
	if err := mwanHealthcheck(loaded(mwanPolicyName), plan); err != nil {
		t.Fatalf("a loaded policy should pass: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Which uplink the rest of the panel should call "the WAN"
// ---------------------------------------------------------------------------

// The reported symptom: with the mobile link set as the main one, the WAN
// card still showed the wired address. mwan3 steers with marks and leaves
// the kernel routes alone, so the route table is not the answer.
func TestActiveUplinkFollowsThePolicyNotTheRouteTable(t *testing.T) {
	show := `mwan3.fiber=interface
mwan3.cell=interface
mwan3.` + mwanMemberPrefix + `cell=member
mwan3.` + mwanMemberPrefix + `cell.interface='cell'
mwan3.` + mwanMemberPrefix + `cell.metric='1'
mwan3.` + mwanMemberPrefix + `fiber=member
mwan3.` + mwanMemberPrefix + `fiber.interface='fiber'
mwan3.` + mwanMemberPrefix + `fiber.metric='2'
mwan3.` + mwanPolicyName + `=policy
mwan3.` + mwanRuleName + `=rule
mwan3.` + mwanRuleName + `.use_policy='` + mwanPolicyName + `'
`
	cfg := readMwanConfig(show)
	both := map[string]bool{"fiber": true, "cell": true}
	if got := pickMwanActive(cfg, both); got != "cell" {
		t.Fatalf("active = %q, want the uplink the policy prefers", got)
	}
	// When the preferred one drops, traffic moves to the next metric.
	if got := pickMwanActive(cfg, map[string]bool{"fiber": true}); got != "fiber" {
		t.Fatalf("active = %q, want the surviving uplink", got)
	}
	if got := pickMwanActive(cfg, map[string]bool{}); got != "" {
		t.Fatalf("active = %q, want nothing while every link is down", got)
	}
}

// Balanced mode has several links carrying traffic; the single-WAN views
// need one address, and the heaviest member is the honest choice.
func TestActiveUplinkPrefersTheHeaviestOfAPool(t *testing.T) {
	show := `mwan3.a=interface
mwan3.b=interface
mwan3.` + mwanMemberPrefix + `a=member
mwan3.` + mwanMemberPrefix + `a.interface='a'
mwan3.` + mwanMemberPrefix + `a.metric='1'
mwan3.` + mwanMemberPrefix + `a.weight='1'
mwan3.` + mwanMemberPrefix + `b=member
mwan3.` + mwanMemberPrefix + `b.interface='b'
mwan3.` + mwanMemberPrefix + `b.metric='1'
mwan3.` + mwanMemberPrefix + `b.weight='4'
mwan3.` + mwanPolicyName + `=policy
mwan3.` + mwanRuleName + `=rule
mwan3.` + mwanRuleName + `.use_policy='` + mwanPolicyName + `'
`
	if got := pickMwanActive(readMwanConfig(show), map[string]bool{"a": true, "b": true}); got != "b" {
		t.Fatalf("active = %q, want the member carrying the larger share", got)
	}
}

// A config that mwan3 never loaded must not be believed: this is the exact
// state a too-long policy name leaves behind, and answering with one of its
// members would put a wrong address on the WAN card.
func TestActiveUplinkIgnoresAConfigThatCannotSteer(t *testing.T) {
	long := "netgrip_policy_default" // 22 chars: over mwan3's limit
	show := `mwan3.fiber=interface
mwan3.` + mwanMemberPrefix + `fiber=member
mwan3.` + mwanMemberPrefix + `fiber.interface='fiber'
mwan3.` + mwanMemberPrefix + `fiber.metric='1'
mwan3.` + long + `=policy
mwan3.netgrip_rule_default=rule
mwan3.netgrip_rule_default.use_policy='` + long + `'
`
	if got := pickMwanActive(readMwanConfig(show), map[string]bool{"fiber": true}); got != "" {
		t.Fatalf("active = %q, want nothing: mwan3 refused those sections", got)
	}
	// A rule pointing at a policy that does not exist steers nothing either.
	orphan := `mwan3.fiber=interface
mwan3.` + mwanMemberPrefix + `fiber=member
mwan3.` + mwanMemberPrefix + `fiber.interface='fiber'
mwan3.` + mwanRuleName + `=rule
mwan3.` + mwanRuleName + `.use_policy='gone'
`
	if got := pickMwanActive(readMwanConfig(orphan), map[string]bool{"fiber": true}); got != "" {
		t.Fatalf("active = %q, want nothing: the rule names no policy that exists", got)
	}
}

// Balancing pins each device to one connection by default, which is what
// keeps calls and logins alive — and also why one device sees one address
// for minutes at a time. It has to be possible to turn that off.
func TestBalanceStickinessIsOnByDefaultAndCanBeTurnedOff(t *testing.T) {
	off := false
	on := true
	for name, tc := range map[string]struct {
		req  MultiWanRequest
		want bool
	}{
		"default":   {MultiWanRequest{Mode: MWModeBalance}, true},
		"asked on":  {MultiWanRequest{Mode: MWModeBalance, Sticky: &on}, true},
		"asked off": {MultiWanRequest{Mode: MWModeBalance, Sticky: &off}, false},
		// Failover has one link at a time; pinning means nothing there.
		"failover ignores it": {MultiWanRequest{Mode: MWModeFailover, Primary: "fiber", Sticky: &on}, false},
	} {
		t.Run(name, func(t *testing.T) {
			plan, err := buildMwanPlan(tc.req, classifyFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			if plan.Sticky != tc.want {
				t.Fatalf("sticky = %v, want %v", plan.Sticky, tc.want)
			}
			ops := buildMwanOps(plan, readMwanConfig(""), false)
			want := "uci_set mwan3." + mwanRuleName + ".sticky 0"
			if tc.want {
				want = "uci_set mwan3." + mwanRuleName + ".sticky 1"
			}
			if !hasOp(ops, want) {
				t.Fatalf("missing %q", want)
			}
		})
	}
}

// And the panel has to show the switch in the position the router is in.
func TestProbeReportsStickiness(t *testing.T) {
	show := `mwan3.` + mwanRuleName + `=rule
mwan3.` + mwanRuleName + `.sticky='1'
mwan3.` + mwanRuleName + `.use_policy='` + mwanPolicyName + `'
mwan3.` + mwanPolicyName + `=policy
`
	if !readMwanConfig(show).Sticky {
		t.Fatal("a sticky rule must be reported as sticky")
	}
	if readMwanConfig(strings.Replace(show, "sticky='1'", "sticky='0'", 1)).Sticky {
		t.Fatal("a rule that does not pin must not be reported as sticky")
	}
}
