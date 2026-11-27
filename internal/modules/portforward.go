package modules

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

const fwdPrefix = "netgrip_fwd_"

// FwdRule is one port forwarding rule.
type FwdRule struct {
	Section  string `json:"section"`
	Name     string `json:"name"`
	SrcDport string `json:"src_dport"`
	DestIP   string `json:"dest_ip"`
	DestPort string `json:"dest_port"`
	Proto    string `json:"proto"`
	// Kind tells the two ways in which something is reachable from the
	// Internet apart: "forward" is a DNAT redirect to a host on the LAN,
	// "input" is a port on the ROUTER itself (a VPN listener, an exposed
	// admin page). Only forwards were ever listed, so a WireGuard port was
	// invisible here.
	Kind string `json:"kind"`
	// Managed: this rule was created by NetGrip and can be removed here.
	// Anything set up outside NetGrip or by hand is listed read-only: hiding it
	// would be worse than not offering to delete it.
	Managed bool `json:"managed"`
}

// FwdProbe is the read-only port forwarding state.
type FwdProbe struct {
	HasWan   bool      `json:"has_wan"`
	Firewall bool      `json:"firewall"`
	Rules    []FwdRule `json:"rules"`
	// RouterIP is the LAN address of the router itself: a forward pointing at
	// it (the DNS redirects, say) is not a service on another machine.
	RouterIP string `json:"router_ip,omitempty"`
}

var (
	reIPv4       = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	reFwdSection = regexp.MustCompile(`^` + fwdPrefix + `[a-z0-9_]+$`)
	// reAutoFwdName matches the descriptive name NetGrip writes on the
	// redirects it creates. It is not shown as-is: the card prints "Puerto
	// <n>" for it, and an edit that keeps it has to follow the new port.
	reAutoFwdName = regexp.MustCompile(`^netgrip-fwd-\d+$`)
)

// maxFwdNameLen is the longest descriptive name accepted. It is a label for
// humans in the firewall config, not an identifier, so the limit is generous
// but bounded: an unbounded string would end up in every `uci show`.
const maxFwdNameLen = 64

// validFwdName accepts the empty string (meaning "use the automatic name")
// and any printable label up to maxFwdNameLen runes. The executor already
// rejects `;`, newlines and backticks; control characters are rejected here
// so a name cannot smuggle terminal escapes into the config.
func validFwdName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	if len([]rune(name)) > maxFwdNameLen {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// resolveFwdName turns the name sent by the client into the one to write.
//
// Empty means "automatic": a rule NetGrip owns gets its generated name back,
// and a rule created elsewhere keeps the name it already had (writing an
// empty option over somebody else's rule would erase their label). An
// automatic name that arrives stale, because the external port changed in
// the same edit, is regenerated so it does not lie about the port.
func resolveFwdName(requested, current string, managed bool, srcDport string) string {
	name := strings.TrimSpace(requested)
	if reAutoFwdName.MatchString(name) {
		return "netgrip-fwd-" + srcDport
	}
	if name != "" {
		return name
	}
	if managed {
		return "netgrip-fwd-" + srcDport
	}
	return current
}

// firewallSections lists the UCI section names of a firewall type
// ("redirect", "rule", "zone"). The show is memoized 2s (#356): it is
// read on every polling burst and the block list reads the same package.
func firewallSections(kind string) []string {
	out, ok := uciShowCached("firewall")
	if !ok {
		return nil
	}
	var sections []string
	for _, line := range strings.Split(out, "\n") {
		// firewall.<section>=<kind>
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) != 2 || parts[1] != kind {
			continue
		}
		if dot := strings.LastIndex(parts[0], "."); dot >= 0 {
			sections = append(sections, parts[0][dot+1:])
		}
	}
	return sections
}

// internetZones names the firewall zones that face the Internet: the ones
// carrying the network of the active uplink.
//
// Resolved, not assumed. The zone is not always called "wan" and "has NAT"
// is not the test either: a VPN zone masquerades too, and traffic arriving
// through the tunnel is not traffic arriving from the Internet. Falls back
// to the conventional name when the uplink cannot be resolved, so a router
// with the WAN down still reports something sensible.
func internetZones() map[string]bool {
	wanNet := ubus.ActiveWANInterfaceName()
	zones := map[string]bool{}
	for _, section := range firewallSections("zone") {
		base := "firewall." + section
		name := uciGet(base + ".name")
		if name == "" {
			continue
		}
		for _, n := range strings.Fields(uciGet(base + ".network")) {
			if n == wanNet {
				zones[name] = true
			}
		}
	}
	if len(zones) == 0 {
		zones["wan"] = true
	}
	return zones
}

// internetZone names the one firewall zone to attach a rule to when the rule
// is about traffic arriving from the Internet. internetZones answers the
// reading question ("which zones face outwards"); this answers the writing
// one, where exactly one name has to be chosen.
//
// A rule written against a zone the uplink is not in is accepted by UCI,
// reloaded without complaint and never matches anything - a forward that
// silently forwards nothing. Picking the lowest name keeps the choice stable
// across calls when a router has several outward zones, so repeated writes do
// not shuffle between them.
func internetZone() string {
	return pickInternetZone(internetZones())
}

// pickInternetZone chooses one name out of the outward-facing set.
func pickInternetZone(zones map[string]bool) string {
	names := make([]string, 0, len(zones))
	for name, ok := range zones {
		if ok && name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "wan"
	}
	sort.Strings(names)
	return names[0]
}

// uplinkNetwork names the network the internet arrives on, for the options
// that take an interface rather than a zone (ddns's ip_network). Falls back
// to the conventional name, so a router with the uplink down keeps writing
// what it wrote before.
func uplinkNetwork() string {
	if n := ubus.ActiveWANInterfaceName(); n != "" {
		return n
	}
	return "wan"
}

// hasUplink reports whether this router has an internet uplink at all,
// which decides whether the features that need one offer themselves.
//
// Asking whether a section named "wan" exists answers a different question:
// a router whose uplink is PPPoE named after the provider has no such
// section and looked like it had no internet, while one whose idle cellular
// modem happens to be called "wan" looked like it had. Asking whether the
// RESOLVED uplink's section exists answers it: the resolver falls back to
// the conventional name when nothing qualifies, so a pure access point
// (no default route, no wan section) reports false, a renamed uplink
// reports its own section, and a router whose uplink is merely down still
// has the section and reports true.
func hasUplink() bool {
	return uciSectionExists("network." + ubus.ActiveWANInterfaceName())
}

// stockRuleNames are the rules the firmware ships with, read from the
// read-only factory config. DHCP renew, ISAKMP and the ICMP rules accept
// traffic from the Internet on every OpenWrt install: listing them as
// "things you opened" would bury the one rule that somebody did open.
// Nothing is hardcoded — if /rom is not there, nothing is treated as stock.
func stockRuleNames() map[string]bool {
	b, err := os.ReadFile("/rom/etc/config/firewall")
	if err != nil {
		return map[string]bool{}
	}
	return parseStockNames(string(b))
}

// parseStockNames pulls the `option name` values out of a UCI config.
func parseStockNames(config string) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Split(config, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "option" && f[1] == "name" {
			if n := strings.Trim(f[2], "'\""); n != "" {
				names[n] = true
			}
		}
	}
	return names
}

// ProbeFwd reads what is reachable from the Internet.
//
// It used to list only redirects whose section name carried NetGrip's own
// prefix, so a port forward created elsewhere did not exist as far as this
// card was concerned — and it never looked at input rules at all, which is
// how a VPN listener is opened. Both cases reported "nothing is open to the
// Internet", the one answer a firewall page must never get wrong.
func ProbeFwd() *FwdProbe {
	p := &FwdProbe{
		HasWan:   hasUplink(),
		Firewall: executor.ServiceEnabled("firewall"),
		Rules:    []FwdRule{},
		RouterIP: uciGet("network.lan.ipaddr"),
	}
	wanZones := internetZones()
	stock := stockRuleNames()

	// DNAT redirects: a port on the router handed to a host on the LAN.
	// Only those coming FROM the Internet — a redirect from an internal
	// zone is something else entirely (forcing local DNS, say), and
	// counting it as an exposure would be alarming and wrong.
	for _, section := range firewallSections("redirect") {
		base := "firewall." + section
		if uciGet(base+".enabled") == "0" || !wanZones[uciGet(base+".src")] {
			continue
		}
		if t := uciGet(base + ".target"); t != "" && t != "DNAT" {
			continue
		}
		p.Rules = append(p.Rules, FwdRule{
			Section:  section,
			Name:     uciGet(base + ".name"),
			SrcDport: uciGet(base + ".src_dport"),
			DestIP:   uciGet(base + ".dest_ip"),
			DestPort: uciGet(base + ".dest_port"),
			Proto:    uciGet(base + ".proto"),
			Kind:     "forward",
			Managed:  reFwdSection.MatchString(section),
		})
	}

	// Input rules: a port open on the router itself. No dest zone means the
	// traffic ends here; with a dest it is a forward between zones, which
	// is a different question. A rule with no port is a protocol allowance
	// (ping, ESP), not an open port.
	for _, section := range firewallSections("rule") {
		base := "firewall." + section
		if uciGet(base+".enabled") == "0" || uciGet(base+".target") != "ACCEPT" {
			continue
		}
		if !wanZones[uciGet(base+".src")] || uciGet(base+".dest") != "" {
			continue
		}
		port := uciGet(base + ".dest_port")
		name := uciGet(base + ".name")
		if port == "" || stock[name] {
			continue
		}
		p.Rules = append(p.Rules, FwdRule{
			Section:  section,
			Name:     name,
			SrcDport: port,
			Proto:    uciGet(base + ".proto"),
			Kind:     "input",
			Managed:  reFwdSection.MatchString(section),
		})
	}
	return p
}

func fwdApplicable() error {
	p := ProbeFwd()
	if !p.HasWan || !p.Firewall {
		return fmt.Errorf("port forwarding only applies on the gateway (needs WAN and firewall)")
	}
	return nil
}

func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

func validProto(p string) bool {
	return p == "tcp" || p == "udp" || p == "tcpudp" || p == "tcp udp"
}

// AddFwdRule creates a redirect rule with snapshot, reload and rollback.
func AddFwdRule(srcDport, destIP, destPort, proto string) (*FwdProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	InvalidateKey("portforward")
	if err := fwdApplicable(); err != nil {
		return ProbeFwd(), false, err
	}
	if !validPort(srcDport) || !validPort(destPort) || !reIPv4.MatchString(destIP) || !validProto(proto) {
		return ProbeFwd(), false, fmt.Errorf("invalid rule data (ports 1-65535, IPv4 dest, proto tcp/udp)")
	}
	for _, r := range ProbeFwd().Rules {
		if r.SrcDport == srcDport && (r.Proto == proto || r.Proto == "tcpudp" || r.Proto == "tcp udp") {
			return ProbeFwd(), false, fmt.Errorf("a rule for external port %s already exists", srcDport)
		}
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}

	section := fwdSection(srcDport, destIP)
	base := "firewall." + section
	proto = strings.ReplaceAll(proto, "tcpudp", "tcp udp")
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{base, "redirect"}},
		{Kind: "uci_set", Args: []string{base + ".name", "netgrip-fwd-" + srcDport}},
		{Kind: "uci_set", Args: []string{base + ".src", internetZone()}},
		{Kind: "uci_set", Args: []string{base + ".src_dport", srcDport}},
		{Kind: "uci_set", Args: []string{base + ".dest", "lan"}},
		{Kind: "uci_set", Args: []string{base + ".dest_ip", destIP}},
		{Kind: "uci_set", Args: []string{base + ".dest_port", destPort}},
		{Kind: "uci_set", Args: []string{base + ".proto", proto}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
		{Kind: "initd", Args: []string{"firewall", "reload"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeFwd(), true, err
	}
	for _, r := range ProbeFwd().Rules {
		if r.Section == section {
			return ProbeFwd(), false, nil
		}
	}
	rollback()
	return ProbeFwd(), true, fmt.Errorf("rule not present after reload, rolled back")
}

// fwdSection is the UCI section name a redirect created by NetGrip gets for
// an external port and a destination: the name encodes both, so an edit of
// one of those rules that changes either moves the section instead of leaving
// a stale name behind (a later rule for the old port would then collide).
func fwdSection(srcDport, destIP string) string {
	return fwdPrefix + srcDport + "_" + strings.ReplaceAll(destIP, ".", "_")
}

// fwdUpdateOps builds the batch that writes the rule at target. A rule that
// moves to a new section is rewritten whole; one that stays (same external
// port and destination, or a rule created elsewhere, which keeps its zones)
// only has its routing fields written.
//
// The name is written only when it changes, or when the rule moves and the
// new section therefore needs one: a rule set up elsewhere keeps its label
// untouched unless the person editing it asked for a new one.
func fwdUpdateOps(section, target, name, currentName, srcDport, destIP, destPort, proto string) []executor.Op {
	base := "firewall." + target
	ops := []executor.Op{}
	moved := target != section
	if moved {
		ops = append(ops,
			executor.Op{Kind: "uci_delete", Args: []string{"firewall." + section}},
			executor.Op{Kind: "uci_set", Args: []string{base, "redirect"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".src", internetZone()}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".dest", "lan"}},
		)
	}
	if name != "" && (moved || name != currentName) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".name", name}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_set", Args: []string{base + ".src_dport", srcDport}},
		executor.Op{Kind: "uci_set", Args: []string{base + ".dest_ip", destIP}},
		executor.Op{Kind: "uci_set", Args: []string{base + ".dest_port", destPort}},
		executor.Op{Kind: "uci_set", Args: []string{base + ".proto", proto}},
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
	)
	return ops
}

// UpdateFwdRule edits a forward: new external port, destination, protocol or
// descriptive name, with the same guarantees as adding one (snapshot, reload
// and rollback).
//
// Any forward the panel lists can be edited, including the ones created
// outside NetGrip or by hand: refusing to touch them left an existing rule
// impossible to change from here. Those keep their section name and zones
// (only the routing fields are written, plus the name when it changed), so
// nothing that points at the section breaks and no part of the firewall the
// panel does not own is rewritten. Deleting still stays limited to NetGrip's
// own rules.
func UpdateFwdRule(section, name, srcDport, destIP, destPort, proto string) (*FwdProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	InvalidateKey("portforward")
	if err := fwdApplicable(); err != nil {
		return ProbeFwd(), false, err
	}
	if !validPort(srcDport) || !validPort(destPort) || !reIPv4.MatchString(destIP) || !validProto(proto) {
		return ProbeFwd(), false, fmt.Errorf("invalid rule data (ports 1-65535, IPv4 dest, proto tcp/udp)")
	}
	if !validFwdName(name) {
		return ProbeFwd(), false, fmt.Errorf("invalid rule name (printable, up to %d characters)", maxFwdNameLen)
	}

	var current *FwdRule
	for _, r := range ProbeFwd().Rules {
		if r.Section == section {
			rule := r
			current = &rule
			break
		}
	}
	if current == nil {
		return ProbeFwd(), false, fmt.Errorf("rule not found")
	}
	if current.Kind != "forward" {
		return ProbeFwd(), false, fmt.Errorf("only port forwards can be edited")
	}
	for _, r := range ProbeFwd().Rules {
		if r.Section == section || r.SrcDport != srcDport {
			continue
		}
		if proto == "tcpudp" || r.Proto == proto || r.Proto == "tcpudp" || r.Proto == "tcp udp" {
			return ProbeFwd(), false, fmt.Errorf("a rule for external port %s already exists", srcDport)
		}
	}

	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}

	managed := reFwdSection.MatchString(section)
	resolved := resolveFwdName(name, current.Name, managed, srcDport)
	target := section
	if managed {
		target = fwdSection(srcDport, destIP)
	}
	ops := fwdUpdateOps(section, target, resolved, current.Name, srcDport, destIP, destPort, strings.ReplaceAll(proto, "tcpudp", "tcp udp"))
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeFwd(), true, err
	}
	for _, r := range ProbeFwd().Rules {
		if r.Section == target && r.SrcDport == srcDport && r.DestIP == destIP && r.DestPort == destPort &&
			(resolved == "" || r.Name == resolved) {
			return ProbeFwd(), false, nil
		}
	}
	rollback()
	return ProbeFwd(), true, fmt.Errorf("rule not present after edit, rolled back")
}

// RemoveFwdRule deletes a redirect rule by section.
func RemoveFwdRule(section string) (*FwdProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	InvalidateKey("portforward")
	if err := fwdApplicable(); err != nil {
		return ProbeFwd(), false, err
	}
	if !strings.HasPrefix(section, fwdPrefix) || !reFwdSection.MatchString(section) {
		return ProbeFwd(), false, fmt.Errorf("not an netgrip rule")
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	ops := []executor.Op{
		{Kind: "uci_delete", Args: []string{"firewall." + section}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
		{Kind: "initd", Args: []string{"firewall", "reload"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeFwd(), true, err
	}
	for _, r := range ProbeFwd().Rules {
		if r.Section == section {
			rollback()
			return ProbeFwd(), true, fmt.Errorf("rule still present after delete, rolled back")
		}
	}
	return ProbeFwd(), false, nil
}
