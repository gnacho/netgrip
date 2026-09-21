// multiwan.go — the internet connections this router has, and which one is
// carrying traffic.
//
// OpenWrt can fail over or share load between several uplinks, but only
// through mwan3, whose model is members, policies, metrics and weights. A
// user does not think in those terms; they think "this one, and that one if
// it dies". This module reads the uplinks whether or not mwan3 is installed
// — they exist either way — and, when it is, reports the shape of its config
// in those two words.
//
// Nothing here keys on the name "wan". On a router with a fibre line and a
// cellular backup, the interface literally named "wan" is routinely the
// standby one.
package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// wanProtos are the protocols an uplink is configured with. Everything else
// — bridges, tunnels, the IPv6 side of a link — is not something to fail
// over between.
var wanProtos = map[string]bool{
	"dhcp": true, "static": true, "pppoe": true, "pppoa": true,
	"qmi": true, "ncm": true, "mbim": true, "wwan": true,
	"modemmanager": true, "3g": true, "pptp": true, "l2tp": true,
}

// meteredProtos are the mobile-broadband protocols. Traffic sent over one
// of these usually costs money, which decides whether it may carry load by
// default.
var meteredProtos = map[string]bool{
	"qmi": true, "ncm": true, "mbim": true, "wwan": true,
	"modemmanager": true, "3g": true,
}

// defaultTrackIPs are the addresses a link is pinged at to decide whether it
// is really up. Two well-known public resolvers on separate networks: the
// gateway alone would call a link healthy while the ISP behind it is down.
// Editable per uplink.
var defaultTrackIPs = []string{"1.1.1.1", "9.9.9.9"}

// Section names netgrip owns. mwan3 requires an interface section to be
// named exactly after the network interface, so those cannot be prefixed and
// carry a marker option instead.
const (
	mwanPkg    = "mwan3"
	mwanPrefix = "netgrip_"
	// mwan3 silently drops a policy or rule whose section name is longer
	// than 15 characters — it logs a warning and carries on without it, so
	// the config looks right while nothing steers any traffic. Every name
	// below stays inside that limit.
	mwanMaxSectionLen = 15
	mwanMemberPrefix  = "netgrip_m_"
	mwanPolicyName    = "netgrip_pol"
	mwanRuleName      = "netgrip_rule"
	mwanManagedOpt    = "netgrip_managed"
	mwanDisabledOpt   = "netgrip_disabled"
	mwanWanMarker     = "netgrip_wan"
)

// Modes. "custom" is a config that works but is not one of the two shapes
// netgrip writes — a hand-written one, typically — and is never silently
// overwritten.
const (
	MWModeOff      = "off"
	MWModeFailover = "failover"
	MWModeBalance  = "balance"
	MWModeCustom   = "custom"
)

// WanCandidate is one internet connection as the panel presents it.
type WanCandidate struct {
	Name     string `json:"name"` // UCI network section, and the mwan3 section name
	Proto    string `json:"proto"`
	Device   string `json:"device,omitempty"`
	L3Device string `json:"l3_device,omitempty"`
	Port     string `json:"port,omitempty"` // physical port, when it has one
	Up       bool   `json:"up"`
	// Active: carrying traffic right now.
	Active bool `json:"active"`
	// Primary: the preferred uplink in failover mode.
	Primary bool `json:"primary"`
	// Online/Tracking come from mwan3 when it runs; empty otherwise.
	Online   string   `json:"online,omitempty"`
	Tracking string   `json:"tracking,omitempty"`
	SharePct int      `json:"share_pct"`
	IPv4     []string `json:"ipv4"`
	Gateway  string   `json:"gateway,omitempty"`
	Metric   int      `json:"metric"`
	Uptime   int64    `json:"uptime"`
	Weight   int      `json:"weight"`
	Balance  bool     `json:"balance"`
	Metered  bool     `json:"metered"`
	Managed  bool     `json:"managed"`
	Track    []string `json:"track"`
	// Reason records which signal qualified this interface, so a surprising
	// list can be explained instead of argued with.
	Reason string `json:"reason"` // zone|route|marker
}

// MultiWanProbe is the whole read-only picture for one poll.
type MultiWanProbe struct {
	Applicable       bool           `json:"applicable"`
	Candidates       []WanCandidate `json:"candidates"`
	MultiWanPossible bool           `json:"multi_wan_possible"`
	Installed        bool           `json:"installed"`
	Enabled          bool           `json:"enabled"`
	Running          bool           `json:"running"`
	Mode             string         `json:"mode"`
	Managed          bool           `json:"managed"`
	Foreign          bool           `json:"foreign"`
	ForeignSections  []string       `json:"foreign_sections"`
	PrimaryIface     string         `json:"primary_iface,omitempty"`
	ActivePolicy     string         `json:"active_policy,omitempty"`
	// Sticky: each device stays on one connection while balancing.
	Sticky        bool     `json:"sticky"`
	DefaultTrack  []string `json:"default_track"`
	PackageID     string   `json:"package_id"`
	ConfigPresent bool     `json:"config_present"`
}

// ---------------------------------------------------------------------------
// Discovery (pure)
// ---------------------------------------------------------------------------

// classifyWanCandidates decides which interfaces are internet uplinks.
//
// The rules, in order: it must have a UCI interface section (which excludes
// every interface netifd invented at runtime), it must not be one of those
// runtime children, its protocol must be one an uplink uses, it must not sit
// on the LAN, and something must actually suggest it faces the internet — a
// masquerading zone, a default route, or an explicit marker.
//
// Runtime children are then folded into their parent: on a modem uplink the
// address and the route belong to a child interface that cannot be
// configured, so its facts are reported under the name that can.
func classifyWanCandidates(
	netSections map[string]uciSection,
	zones []FWZone,
	dump []ubus.InterfaceState,
	lanDevices []string,
) []WanCandidate {
	byName := map[string]ubus.InterfaceState{}
	for _, s := range dump {
		byName[s.Name] = s
	}
	zoneNets := internetZoneNetworks(zones)

	out := []WanCandidate{}
	for name, sec := range netSections {
		if sec.Type != "interface" {
			continue
		}
		state := byName[name]
		if state.Dynamic {
			continue
		}
		proto := sec.Option("proto")
		if proto == "" {
			proto = state.Proto
		}
		if !wanProtos[proto] {
			continue
		}
		c := WanCandidate{
			Name:     name,
			Proto:    proto,
			Device:   firstNonBlank(state.Device, sec.Option("device")),
			L3Device: state.L3Device,
			Up:       state.Up,
			IPv4:     append([]string{}, state.IPv4...),
			Gateway:  state.Gateway,
			Uptime:   state.Uptime,
			Metric:   routeCost(state),
			Metered:  meteredProtos[proto],
			Weight:   1,
			Track:    []string{},
		}
		hasRoute := state.HasDefaultRoute
		// Fold in whatever netifd spawned underneath this interface.
		for _, child := range dump {
			if !child.Dynamic || child.Name == name || child.L3Device == "" || child.L3Device != state.L3Device {
				continue
			}
			if len(c.IPv4) == 0 {
				c.IPv4 = append([]string{}, child.IPv4...)
			}
			if c.Gateway == "" {
				c.Gateway = child.Gateway
			}
			if c.Uptime == 0 {
				c.Uptime = child.Uptime
			}
			if child.HasDefaultRoute && !hasRoute {
				hasRoute = true
				c.Metric = routeCost(child)
			}
			c.Up = c.Up || child.Up
		}
		if isLANDevice(c.L3Device, lanDevices) || isLANDevice(c.Device, lanDevices) {
			continue
		}
		switch {
		case sec.Option(mwanWanMarker) == "1":
			c.Reason = "marker"
		case zoneNets[name]:
			c.Reason = "zone"
		case hasRoute:
			c.Reason = "route"
		default:
			continue
		}
		c.Port = physicalPort(c.Device)
		c.Active = hasRoute
		out = append(out, c)
	}

	// Several uplinks can hold a default route at once; the kernel uses the
	// cheapest, so only that one is actually carrying traffic.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	best := -1
	for i := range out {
		if !out[i].Active {
			continue
		}
		if best < 0 || out[i].Metric < out[best].Metric {
			best = i
		}
	}
	for i := range out {
		out[i].Active = i == best
	}
	return out
}

// internetZoneNetworks lists the networks of every zone that both
// masquerades and does not forward freely — the shape of a zone facing the
// internet. A VPN zone masquerades too, but the interfaces in it never pass
// the protocol test, so it contributes nothing.
func internetZoneNetworks(zones []FWZone) map[string]bool {
	nets := map[string]bool{}
	for _, z := range zones {
		if !z.Masq || strings.EqualFold(z.Forward, "ACCEPT") {
			continue
		}
		for _, n := range z.Network {
			nets[n] = true
		}
	}
	return nets
}

// routeCost is what orders two default routes. Interfaces without one sort
// last, which keeps them out of the "active" comparison.
func routeCost(s ubus.InterfaceState) int {
	if !s.HasDefaultRoute {
		return 1 << 30
	}
	if s.RouteMetric > 0 {
		return s.RouteMetric
	}
	return s.Metric
}

var reVLANSuffix = regexp.MustCompile(`\.\d+$`)

// physicalPort is the socket an uplink is plugged into, when it has one at
// all: a modem reports a character device, and a VLAN-tagged uplink reports
// the port with its tag appended.
func physicalPort(dev string) string {
	if dev == "" || strings.HasPrefix(dev, "/dev/") || strings.HasPrefix(dev, "br-") {
		return ""
	}
	return reVLANSuffix.ReplaceAllString(dev, "")
}

func isLANDevice(dev string, lanDevices []string) bool {
	if dev == "" {
		return false
	}
	for _, l := range lanDevices {
		if l == "" {
			continue
		}
		// A VLAN of the LAN bridge is still the LAN.
		if dev == l || strings.HasPrefix(dev, l+".") {
			return true
		}
	}
	return false
}

// uciListParts splits the values of a list option. `uci show` prints a whole
// list on one line (key='a' 'b' 'c'), and the generic parser only strips the
// outermost quotes, so four tracking addresses arrive as one string.
func uciListParts(vals []string) []string {
	out := []string{}
	for _, v := range vals {
		for _, part := range strings.Split(v, "' '") {
			if p := strings.TrimSpace(strings.Trim(part, "'")); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func firstNonBlank(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Reading the mwan3 config (pure)
// ---------------------------------------------------------------------------

// mwanConfig is what the config says, independently of what is running.
type mwanConfig struct {
	Mode    string
	Primary string
	Managed bool
	Foreign []string
	// Members maps an mwan3 member section to its interface, metric and weight.
	Members map[string]mwanMember
	// Ifaces lists the interface sections, whether netgrip wrote them and
	// what they track.
	Ifaces map[string]mwanIface
	// RulePolicies maps a rule section to the policy it invokes.
	RulePolicies map[string]string
	// Sticky: our own rule keeps each device on one connection.
	Sticky bool
	// All maps every section name to its type. `uci delete` fails on a
	// missing entry and takes the whole apply down with it, so nothing is
	// deleted without looking here first.
	All map[string]string
}

type mwanMember struct {
	Section   string
	Interface string
	Metric    int
	Weight    int
	Ours      bool
}

type mwanIface struct {
	Enabled bool
	Ours    bool
	// Disabled: switched off by netgrip during a takeover, and therefore
	// ours to switch back on when multi-WAN is turned off again.
	Disabled bool
	Track    []string
}

// readMwanConfig interprets `uci show mwan3`. The mode is inferred from the
// members rather than stored: a config edited elsewhere then describes
// itself honestly instead of claiming whatever netgrip last wrote.
func readMwanConfig(show string) mwanConfig {
	cfg := mwanConfig{
		Mode:         MWModeOff,
		Members:      map[string]mwanMember{},
		Ifaces:       map[string]mwanIface{},
		Foreign:      []string{},
		All:          map[string]string{},
		RulePolicies: map[string]string{},
	}
	ours, foreign := 0, 0
	for name, sec := range parseUCIShow(show, mwanPkg) {
		isOurs := strings.HasPrefix(name, mwanPrefix)
		cfg.All[name] = sec.Type
		switch sec.Type {
		case "interface":
			cfg.Ifaces[name] = mwanIface{
				Enabled:  sec.Option("enabled") != "0",
				Ours:     sec.Option(mwanManagedOpt) == "1",
				Disabled: sec.Option(mwanDisabledOpt) == "1",
				Track:    uciListParts(sec.Options["track_ip"]),
			}
		case "member":
			m := mwanMember{Section: name, Interface: sec.Option("interface"), Ours: isOurs}
			m.Metric, _ = strconv.Atoi(sec.Option("metric"))
			m.Weight, _ = strconv.Atoi(sec.Option("weight"))
			if m.Weight == 0 {
				m.Weight = 1
			}
			cfg.Members[name] = m
			if isOurs {
				ours++
			} else {
				foreign++
				cfg.Foreign = append(cfg.Foreign, name)
			}
		case "policy", "rule":
			if sec.Type == "rule" {
				cfg.RulePolicies[name] = sec.Option("use_policy")
				if name == mwanRuleName {
					cfg.Sticky = sec.Option("sticky") == "1"
				}
			}
			if isOurs {
				ours++
			} else {
				foreign++
				cfg.Foreign = append(cfg.Foreign, name)
			}
		}
	}
	sort.Strings(cfg.Foreign)
	cfg.Managed = ours > 0 && foreign == 0
	cfg.Mode, cfg.Primary = mwanModeFrom(cfg.Members)
	if cfg.Mode == MWModeOff && foreign > 0 {
		cfg.Mode = MWModeCustom
	}
	return cfg
}

// mwanModeFrom reads the two modes out of member metrics. mwan3 only uses
// the members in the lowest metric group that has a link online, so several
// members sharing the lowest metric means they share traffic, and one alone
// at the lowest metric means the others are standby.
func mwanModeFrom(members map[string]mwanMember) (mode, primary string) {
	lowest, count, first := 0, 0, ""
	for _, m := range members {
		if !m.Ours || m.Interface == "" {
			continue
		}
		switch {
		case count == 0 || m.Metric < lowest:
			lowest, count, first = m.Metric, 1, m.Interface
		case m.Metric == lowest:
			count++
			if m.Interface < first {
				first = m.Interface
			}
		}
	}
	if count == 0 {
		return MWModeOff, ""
	}
	if count > 1 {
		return MWModeBalance, ""
	}
	return MWModeFailover, first
}

// ---------------------------------------------------------------------------
// Reading what mwan3 is doing right now (pure)
// ---------------------------------------------------------------------------

type mwanLive struct {
	Online   string // online|offline|unknown
	Tracking string // active|down
}

var reMwanIface = regexp.MustCompile(`^\s*interface (\S+) is (\w+) and tracking is (\w+)`)

// parseMwanInterfaces reads `mwan3 interfaces`, which is the only place the
// tracker's own verdict on a link is published.
func parseMwanInterfaces(out string) map[string]mwanLive {
	res := map[string]mwanLive{}
	for _, line := range strings.Split(out, "\n") {
		m := reMwanIface.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		res[m[1]] = mwanLive{Online: m[2], Tracking: m[3]}
	}
	return res
}

var reMwanShare = regexp.MustCompile(`^\s+(\S+)\s+\((\d+)%\)`)

// parseMwanPolicies reads `mwan3 policies`. Under mwan3 the route table no
// longer says where traffic goes — the marks do — so this is the only honest
// source for "which uplink is carrying it", and it is per address family.
func parseMwanPolicies(out string) (policy string, shares map[string]int) {
	shares = map[string]int{}
	inV4 := false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Current ipv4 policies"):
			inV4 = true
			continue
		case strings.HasPrefix(trimmed, "Current ipv6 policies"):
			inV4 = false
			continue
		}
		if !inV4 || trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			if policy == "" {
				policy = strings.TrimSuffix(trimmed, ":")
			}
			continue
		}
		if m := reMwanShare.FindStringSubmatch(line); m != nil {
			pct, _ := strconv.Atoi(m[2])
			shares[m[1]] = pct
		}
	}
	return policy, shares
}

// ---------------------------------------------------------------------------
// The probe
// ---------------------------------------------------------------------------

// buildMultiWanProbe assembles the probe from already-gathered inputs, so
// every state — including "mwan3 is not installed" — is reachable in a test
// without a router.
func buildMultiWanProbe(
	candidates []WanCandidate,
	installed, enabled, running, configPresent bool,
	cfg mwanConfig,
	live map[string]mwanLive,
	activePolicy string,
	shares map[string]int,
) *MultiWanProbe {
	p := &MultiWanProbe{
		Applicable:       len(candidates) > 0,
		Candidates:       candidates,
		MultiWanPossible: len(candidates) >= 2,
		Installed:        installed,
		Enabled:          enabled,
		Running:          running,
		Mode:             MWModeOff,
		DefaultTrack:     defaultTrackIPs,
		PackageID:        mwanPkg,
		ConfigPresent:    configPresent,
		ForeignSections:  []string{},
	}
	// A removed package leaves its config behind. Offering modes for a
	// service that cannot run would be a lie.
	if !installed {
		return p
	}
	p.Mode = cfg.Mode
	p.Sticky = cfg.Sticky
	p.Managed = cfg.Managed
	p.Foreign = len(cfg.Foreign) > 0
	p.ForeignSections = cfg.Foreign
	p.PrimaryIface = cfg.Primary
	p.ActivePolicy = activePolicy

	for i := range p.Candidates {
		c := &p.Candidates[i]
		if l, ok := live[c.Name]; ok {
			c.Online, c.Tracking = l.Online, l.Tracking
		}
		if ifc, ok := cfg.Ifaces[c.Name]; ok {
			c.Managed = ifc.Ours
			if len(ifc.Track) > 0 {
				c.Track = ifc.Track
			}
		}
		if len(c.Track) == 0 {
			c.Track = defaultTrackIPs
		}
		for _, m := range cfg.Members {
			if m.Interface != c.Name || !m.Ours {
				continue
			}
			c.Weight = m.Weight
			c.Primary = cfg.Mode == MWModeFailover && m.Interface == cfg.Primary
			c.Balance = cfg.Mode == MWModeBalance && m.Metric == lowestOurMetric(cfg.Members)
		}
		// Once mwan3 publishes a distribution it outranks the route table,
		// and it outranks it for every uplink: one missing from the list is
		// one carrying nothing, however its route looks.
		if len(shares) > 0 {
			c.SharePct = shares[c.Name]
			c.Active = c.SharePct > 0
		}
	}
	return p
}

func lowestOurMetric(members map[string]mwanMember) int {
	lowest, found := 0, false
	for _, m := range members {
		if !m.Ours {
			continue
		}
		if !found || m.Metric < lowest {
			lowest, found = m.Metric, true
		}
	}
	return lowest
}

// wanCandidates gathers the live inputs and runs the classifier.
func wanCandidates() []WanCandidate {
	show, err := uciShowNetwork()
	if err != nil {
		return []WanCandidate{}
	}
	dump, err := ubus.DumpInterfaces()
	if err != nil {
		dump = []ubus.InterfaceState{}
	}
	zones := []FWZone{}
	if out, err := exec.Command("uci", "show", "firewall").Output(); err == nil {
		zones = parseFWZonesFrom(string(out))
	}
	return classifyWanCandidates(parseUCIShow(show, "network"), zones, dump, []string{LANDevice(), LANBridge()})
}

// ProbeMultiWAN reports the uplinks and, when mwan3 is installed, what it is
// doing with them.
func ProbeMultiWAN() *MultiWanProbe {
	candidates := wanCandidates()
	installed := pkgInstalled(mwanPkg)
	cfg := mwanConfig{Mode: MWModeOff, Members: map[string]mwanMember{}, Ifaces: map[string]mwanIface{},
		Foreign: []string{}, All: map[string]string{}, RulePolicies: map[string]string{}}
	live := map[string]mwanLive{}
	policy, shares := "", map[string]int{}
	configPresent := false

	if out, err := exec.Command("uci", "show", mwanPkg).Output(); err == nil {
		configPresent = strings.TrimSpace(string(out)) != ""
		cfg = readMwanConfig(string(out))
	}
	running := installed && executor.ServiceRunning(mwanPkg)
	if running {
		if out, err := exec.Command(mwanPkg, "interfaces").Output(); err == nil {
			live = parseMwanInterfaces(string(out))
		}
		if out, err := exec.Command(mwanPkg, "policies").Output(); err == nil {
			policy, shares = parseMwanPolicies(string(out))
		}
	}
	return buildMultiWanProbe(candidates, installed, installed && executor.ServiceEnabled(mwanPkg),
		running, configPresent, cfg, live, policy, shares)
}

// ---------------------------------------------------------------------------
// Writing: what the two modes mean in mwan3's own terms
// ---------------------------------------------------------------------------

// MultiWanRequest is what the panel asks for. Weights and Balance only mean
// anything in balance mode; Primary only in failover.
type MultiWanRequest struct {
	Mode    string              `json:"mode"`
	Primary string              `json:"primary"`
	Weights map[string]int      `json:"weights"`
	Balance map[string]bool     `json:"balance"`
	Track   map[string][]string `json:"track"`
	// Sticky keeps each device on the connection it started on. Unset
	// means yes, which is what stops a video call dying mid-sentence;
	// turning it off spreads every new connection by weight instead.
	Sticky         *bool `json:"sticky"`
	ConfirmForeign bool  `json:"confirm_foreign"`
}

// mwanPlan is the resolved intent: exactly which sections will exist
// afterwards. Building it is pure, so both modes can be asserted without a
// router anywhere near them.
type mwanPlan struct {
	Mode    string
	Ifaces  []mwanPlanIface
	Members []mwanPlanMember
	Sticky  bool
}

type mwanPlanIface struct {
	Name  string
	Track []string
}

type mwanPlanMember struct {
	Iface  string
	Metric int
	Weight int
}

// Tracking probe timings. Five failed probes ten seconds apart before a link
// is declared down: long enough that a PPPoE renegotiation does not trip a
// failover, short enough that a real outage moves within a minute.
const (
	mwanTrackCount    = "1"
	mwanTrackSize     = "56"
	mwanTrackTTL      = "60"
	mwanTrackTimeout  = "4"
	mwanTrackInterval = "10"
	mwanTrackFailInt  = "5"
	mwanTrackRecInt   = "5"
	mwanTrackDown     = "5"
	mwanTrackUp       = "3"
	mwanStickyTimeout = "600"
)

var reIPv4Only = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)

// buildMwanPlan validates the request against what the router actually has
// and resolves it into sections. Returns an error the panel can show as-is.
func buildMwanPlan(req MultiWanRequest, candidates []WanCandidate) (mwanPlan, error) {
	plan := mwanPlan{Mode: req.Mode}
	switch req.Mode {
	case MWModeOff:
		return plan, nil
	case MWModeFailover, MWModeBalance:
	default:
		return plan, fmt.Errorf("unknown mode %q", req.Mode)
	}
	if len(candidates) < 2 {
		return plan, fmt.Errorf("multi-WAN needs two internet connections; this router has %d", len(candidates))
	}

	byName := map[string]WanCandidate{}
	order := []string{}
	for _, c := range candidates {
		byName[c.Name] = c
		order = append(order, c.Name)
	}
	sort.Strings(order)

	for name, ips := range req.Track {
		if _, ok := byName[name]; !ok {
			return plan, fmt.Errorf("%q is not an internet connection on this router", name)
		}
		if len(ips) > 4 {
			return plan, fmt.Errorf("at most four tracking addresses per connection")
		}
		for _, ip := range ips {
			if !reIPv4Only.MatchString(ip) {
				return plan, fmt.Errorf("%q is not an IPv4 address", ip)
			}
		}
	}

	if req.Mode == MWModeFailover {
		if req.Primary == "" {
			return plan, fmt.Errorf("choose which connection is the main one")
		}
		if _, ok := byName[req.Primary]; !ok {
			return plan, fmt.Errorf("%q is not an internet connection on this router", req.Primary)
		}
		// The main one first, then the rest as fallbacks in a stable order.
		// Distinct metrics are what makes mwan3 treat them as an order of
		// preference rather than a pool.
		metric := 1
		plan.Members = append(plan.Members, mwanPlanMember{Iface: req.Primary, Metric: metric, Weight: 1})
		for _, name := range order {
			if name == req.Primary {
				continue
			}
			metric++
			plan.Members = append(plan.Members, mwanPlanMember{Iface: name, Metric: metric, Weight: 1})
		}
	} else {
		if req.Primary != "" {
			return plan, fmt.Errorf("a main connection cannot be chosen while balancing")
		}
		included := 0
		for _, name := range order {
			c := byName[name]
			// A metered link stays out of the pool unless it is asked for
			// by name: balancing onto mobile data costs money.
			use := !c.Metered
			if v, ok := req.Balance[name]; ok {
				use = v
			}
			weight := 1
			if w, ok := req.Weights[name]; ok {
				weight = w
			}
			if weight < 1 || weight > 10 {
				return plan, fmt.Errorf("the share of %q must be between 1 and 10", name)
			}
			if use {
				included++
				plan.Members = append(plan.Members, mwanPlanMember{Iface: name, Metric: 1, Weight: weight})
			} else {
				// Kept as a fallback rather than dropped: a link excluded
				// from the pool should still save the house when the pool
				// goes down.
				plan.Members = append(plan.Members, mwanPlanMember{Iface: name, Metric: 2, Weight: 1})
			}
		}
		if included == 0 {
			return plan, fmt.Errorf("at least one connection has to carry traffic")
		}
		// Flows stay on the link they started on: alternating source
		// addresses mid-session breaks logins and video calls. It also
		// means one device sees one connection for minutes at a time,
		// which is worth being able to turn off.
		plan.Sticky = req.Sticky == nil || *req.Sticky
	}

	for _, m := range plan.Members {
		c := byName[m.Iface]
		track := c.Track
		if v, ok := req.Track[m.Iface]; ok && len(v) > 0 {
			track = v
		}
		if len(track) == 0 {
			track = defaultTrackIPs
		}
		plan.Ifaces = append(plan.Ifaces, mwanPlanIface{Name: m.Iface, Track: track})
	}
	return plan, nil
}

// teardownOps removes what netgrip put in mwan3, newest reference first so a
// half-applied state never leaves a policy pointing at a member that is
// gone. Only sections that exist are deleted: `uci delete` fails on a
// missing entry, and that failure would abort the whole apply.
func teardownOps(cfg mwanConfig, takeover bool) []executor.Op {
	ops := []executor.Op{}
	del := func(name string) {
		if _, ok := cfg.All[name]; ok {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{mwanPkg + "." + name}})
		}
	}
	// Rules first (they are what actually steers traffic), then policies,
	// then the members they name.
	for _, typ := range []string{"rule", "policy", "member"} {
		for _, name := range sortedNames(cfg.All, typ) {
			// Someone else's sections are only removed with consent. There
			// is no gentler option for these three types: mwan3 has no
			// "enabled" flag for them, and a second catch-all rule would
			// race ours by evaluation order rather than being ignored.
			if strings.HasPrefix(name, mwanPrefix) || takeover {
				del(name)
			}
		}
	}
	return ops
}

func sortedNames(all map[string]string, typ string) []string {
	out := []string{}
	for name, t := range all {
		if t == typ {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// buildMwanOps turns a plan into the ops that realise it. Interface sections
// are written in place rather than deleted and recreated: recreating one
// makes mwan3 bring the interface up again on restart, and that is the one
// thing that can drop a live connection.
func buildMwanOps(plan mwanPlan, cfg mwanConfig, takeover bool) []executor.Op {
	ops := teardownOps(cfg, takeover)
	set := func(key, val string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{mwanPkg + "." + key, val}})
	}
	addList := func(key, val string) {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{mwanPkg + "." + key, val}})
	}

	planned := map[string]bool{}
	for _, ifc := range plan.Ifaces {
		planned[ifc.Name] = true
	}
	// Interface sections nobody planned for - the IPv6 halves of these
	// links, typically - are switched off rather than deleted, and marked
	// so that turning multi-WAN off can switch them back on. Deleting them
	// would throw away tracking settings netgrip never wrote.
	if takeover && plan.Mode != MWModeOff {
		for _, name := range sortedNames(cfg.All, "interface") {
			if planned[name] || cfg.Ifaces[name].Disabled {
				continue
			}
			set(name+".enabled", "0")
			set(name+"."+mwanDisabledOpt, "1")
		}
	}
	if plan.Mode == MWModeOff {
		for _, name := range sortedNames(cfg.All, "interface") {
			if !cfg.Ifaces[name].Disabled {
				continue
			}
			set(name+".enabled", "1")
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{mwanPkg + "." + name + "." + mwanDisabledOpt}})
		}
	}

	if plan.Mode != MWModeOff {
		// mwan3 needs a globals section to exist. Never touch an existing
		// one: its mark mask is shared with the firewall.
		if _, ok := cfg.All["globals"]; !ok {
			set("globals", "globals")
			set("globals.netgrip_created", "1")
		}
		for _, ifc := range plan.Ifaces {
			base := ifc.Name
			if cfg.All[base] != "interface" {
				set(base, "interface")
			}
			set(base+".enabled", "1")
			// Usable before the first probe lands, so applying this does
			// not black out a working link for one tracking interval.
			set(base+".initial_state", "online")
			set(base+".family", "ipv4")
			set(base+".track_method", "ping")
			set(base+".reliability", "1")
			set(base+".count", mwanTrackCount)
			set(base+".size", mwanTrackSize)
			set(base+".max_ttl", mwanTrackTTL)
			set(base+".timeout", mwanTrackTimeout)
			set(base+".interval", mwanTrackInterval)
			set(base+".failure_interval", mwanTrackFailInt)
			set(base+".recovery_interval", mwanTrackRecInt)
			set(base+".down", mwanTrackDown)
			set(base+".up", mwanTrackUp)
			set(base+"."+mwanManagedOpt, "1")
			if len(cfg.Ifaces[base].Track) > 0 {
				ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{mwanPkg + "." + base + ".track_ip"}})
			}
			for _, ip := range ifc.Track {
				addList(base+".track_ip", ip)
			}
		}
		for _, m := range plan.Members {
			name := mwanMemberPrefix + sanitizeUCIKey(m.Iface)
			set(name, "member")
			set(name+".interface", m.Iface)
			set(name+".metric", strconv.Itoa(m.Metric))
			set(name+".weight", strconv.Itoa(m.Weight))
		}
		set(mwanPolicyName, "policy")
		for _, m := range plan.Members {
			addList(mwanPolicyName+".use_member", mwanMemberPrefix+sanitizeUCIKey(m.Iface))
		}
		// Without a last resort mwan3 lets traffic leak out of the default
		// route when every member is down, which looks like a working
		// connection that silently ignores the policy.
		set(mwanPolicyName+".last_resort", "unreachable")

		set(mwanRuleName, "rule")
		// Without a family the same rule is applied to IPv6, where
		// 0.0.0.0/0 is not an address and mwan3 rejects it.
		set(mwanRuleName+".family", "ipv4")
		set(mwanRuleName+".dest_ip", "0.0.0.0/0")
		set(mwanRuleName+".proto", "all")
		if plan.Sticky {
			set(mwanRuleName+".sticky", "1")
			set(mwanRuleName+".sticky_timeout", mwanStickyTimeout)
		} else {
			set(mwanRuleName+".sticky", "0")
		}
		set(mwanRuleName+".use_policy", mwanPolicyName)
	}

	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{mwanPkg}})
	if plan.Mode == MWModeOff {
		ops = append(ops,
			executor.Op{Kind: "initd", Args: []string{mwanPkg, "stop"}},
			executor.Op{Kind: "initd", Args: []string{mwanPkg, "disable"}},
		)
	} else {
		ops = append(ops,
			executor.Op{Kind: "initd", Args: []string{mwanPkg, "enable"}},
			executor.Op{Kind: "initd", Args: []string{mwanPkg, "restart"}},
		)
	}
	return ops
}

// ApplyMultiWAN puts the router into the requested mode.
//
// The sequence is the house one — gate, snapshot, apply, prove it worked,
// roll back if it did not — with one addition that matters here: the proof
// includes the internet still being reachable. Every other module can be
// wrong and leave the user annoyed; this one can be wrong and leave the
// house offline.
func ApplyMultiWAN(req MultiWanRequest) (*MultiWanProbe, bool, error) {
	probe := ProbeMultiWAN()
	if !probe.Installed {
		return probe, false, fmt.Errorf("the multi-WAN manager is not installed")
	}
	// A configuration set up elsewhere is never silently replaced.
	if probe.Foreign && !req.ConfirmForeign {
		return probe, false, fmt.Errorf("%s%s", errForeignPrefix, strings.Join(probe.ForeignSections, ", "))
	}

	plan, err := buildMwanPlan(req, probe.Candidates)
	if err != nil {
		return probe, false, err
	}

	show, _ := exec.Command("uci", "show", mwanPkg).Output()
	cfg := readMwanConfig(string(show))

	// Taking over deletes sections netgrip did not write, and mwan3 has no
	// way to disable them instead. A full snapshot first is what makes that
	// undoable from Tools afterwards, not just within this call.
	takeover := probe.Foreign && req.ConfirmForeign
	if takeover {
		if _, err := CreateSnapshot(); err != nil {
			return probe, false, fmt.Errorf("could not save a restore point first: %w", err)
		}
	}

	snap := ""
	if _, err := os.Stat("/etc/config/" + mwanPkg); err == nil {
		if s, err := executor.Snapshot(mwanPkg); err == nil {
			snap = s
		}
	}
	rollback := func() {
		if snap != "" {
			_ = executor.Restore(mwanPkg, snap)
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{mwanPkg, "restart"}})
	}

	if err := executor.Apply(buildMwanOps(plan, cfg, takeover), nil); err != nil {
		rollback()
		return ProbeMultiWAN(), true, err
	}

	// mwan3 needs a moment to install its rules and run a first probe.
	time.Sleep(mwanSettle)
	after := ProbeMultiWAN()
	if err := mwanHealthcheck(after, plan); err != nil {
		rollback()
		time.Sleep(time.Second)
		return ProbeMultiWAN(), true, err
	}
	return after, false, nil
}

// errForeignPrefix is matched by the panel to offer the takeover dialog
// rather than showing a bare error.
const errForeignPrefix = "multiwan: configured outside netgrip: "

// mwanSettle is one tracking interval plus a little: long enough for the
// first probe to have landed before the result is judged.
var mwanSettle = 6 * time.Second

// mwanHealthcheck decides whether what was just applied is working. The
// last check is the one that matters: a config that parses, loads and
// leaves the house with no internet is a failed apply.
func mwanHealthcheck(after *MultiWanProbe, plan mwanPlan) error {
	if plan.Mode == MWModeOff {
		if after.Mode != MWModeOff {
			return fmt.Errorf("multi-WAN is still configured after turning it off")
		}
		return nil
	}
	if !after.Running {
		return fmt.Errorf("the multi-WAN manager did not start")
	}
	if after.Mode != plan.Mode {
		return fmt.Errorf("the router reports %q after applying %q", after.Mode, plan.Mode)
	}
	// The config being right is not the same as it being in force: mwan3
	// drops sections it dislikes with nothing but a log line, leaving a
	// configuration that reads correctly and steers nothing.
	if after.ActivePolicy != mwanPolicyName {
		return fmt.Errorf("the multi-WAN manager did not load the new rules (it is using %q)",
			firstNonBlank(after.ActivePolicy, "none"))
	}
	if st, err := ubus.GetWanStatus(); err == nil && st.Present && !st.Up {
		return fmt.Errorf("the internet connection went down applying this")
	}
	for _, c := range after.Candidates {
		if c.Active {
			return nil
		}
	}
	return fmt.Errorf("no connection is carrying traffic after applying this")
}

// SetMultiWANPrimary makes one uplink the main one. It re-runs the whole
// failover path rather than patching two metrics: which link is primary is
// encoded entirely in those metrics, rewriting them all is idempotent, and
// it keeps one code path carrying the healthcheck and the rollback.
func SetMultiWANPrimary(iface string) (*MultiWanProbe, bool, error) {
	probe := ProbeMultiWAN()
	if probe.Mode != MWModeFailover {
		return probe, false, fmt.Errorf("a main connection can only be chosen in failover mode")
	}
	track := map[string][]string{}
	for _, c := range probe.Candidates {
		if len(c.Track) > 0 {
			track[c.Name] = c.Track
		}
	}
	return ApplyMultiWAN(MultiWanRequest{Mode: MWModeFailover, Primary: iface, Track: track, ConfirmForeign: true})
}

// ---------------------------------------------------------------------------
// Which uplink is actually carrying traffic
// ---------------------------------------------------------------------------

// mwan3 steers with packet marks and leaves the kernel's default routes
// untouched, so every part of the panel that asks "which uplink is the WAN"
// gets the wrong answer the moment a policy sends traffic somewhere else:
// the address shown is the one nobody is using. This is where the honest
// answer comes from, and it is wired into the ubus layer at startup so the
// WAN card, the settings form, the port list and the rest all follow it.
func init() { ubus.ActiveUplinkResolver = MwanActiveUplink }

const (
	mwanRunDir     = "/var/run/" + mwanPkg
	mwanStateDir   = mwanRunDir + "/iface_state"
	mwanConfigFile = "/etc/config/" + mwanPkg
)

// mwanActive caches the answer: it is asked for on every probe, and while
// each ingredient is only a file read, there are several of them.
var mwanActive struct {
	mu   sync.Mutex
	at   time.Time
	name string
}

var mwanActiveTTL = 2 * time.Second

// MwanActiveUplink names the uplink mwan3 is currently steering traffic
// through, or "" when it is not steering at all — not installed, not
// running, or configured in a way it refused to load.
func MwanActiveUplink() string {
	mwanActive.mu.Lock()
	defer mwanActive.mu.Unlock()
	if time.Since(mwanActive.at) < mwanActiveTTL {
		return mwanActive.name
	}
	mwanActive.at = time.Now()
	mwanActive.name = ""

	if _, err := os.Stat(mwanStateDir); err != nil {
		return ""
	}
	show, err := exec.Command("uci", "show", mwanPkg).Output()
	if err != nil {
		return ""
	}
	cfg := readMwanConfig(string(show))
	online := map[string]bool{}
	entries, err := os.ReadDir(mwanStateDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		b, err := os.ReadFile(mwanStateDir + "/" + e.Name())
		if err == nil && strings.TrimSpace(string(b)) == "online" {
			online[e.Name()] = true
		}
	}
	mwanActive.name = pickMwanActive(cfg, online)
	return mwanActive.name
}

// pickMwanActive works out which member is carrying traffic from the config
// and the tracker's verdict on each link, the same way mwan3 does: only the
// lowest metric group with something online is used, and within it the
// heaviest member carries the most.
//
// It returns "" when no rule can actually be steering anything — an
// unreferenced policy, or one mwan3 will have refused for its name being too
// long. A config that looks right but was never loaded must not be believed.
func pickMwanActive(cfg mwanConfig, online map[string]bool) string {
	steering := false
	for name, policy := range cfg.RulePolicies {
		if len(name) > mwanMaxSectionLen || len(policy) > mwanMaxSectionLen {
			continue
		}
		if cfg.All[policy] == "policy" {
			steering = true
			break
		}
	}
	if !steering {
		return ""
	}
	best, bestMetric, bestWeight := "", 0, 0
	for _, m := range cfg.Members {
		if m.Interface == "" || !online[m.Interface] {
			continue
		}
		switch {
		case best == "", m.Metric < bestMetric,
			m.Metric == bestMetric && m.Weight > bestWeight,
			m.Metric == bestMetric && m.Weight == bestWeight && m.Interface < best:
			best, bestMetric, bestWeight = m.Interface, m.Metric, m.Weight
		}
	}
	return best
}
