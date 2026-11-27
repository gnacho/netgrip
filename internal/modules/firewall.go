package modules

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// fwRulePrefix names the UCI sections NetGrip owns for traffic rules
// (firewall.rule sections named netgrip_fw_N). Anything else is a system
// rule (another panel, the firmware factory config): listed read-only, never edited
// or deleted from here.
const fwRulePrefix = "netgrip_fw_"

// reFwRuleSection matches a managed traffic rule section name (netgrip_fw_1).
var reFwRuleSection = regexp.MustCompile(`^` + fwRulePrefix + `[0-9]+$`)

type FWZone struct {
	Name    string   `json:"name"`
	Input   string   `json:"input"`
	Output  string   `json:"output"`
	Forward string   `json:"forward"`
	Network []string `json:"network"`
	Masq    bool     `json:"masq"`
}

// FWOption is one rule option the panel does not model (a src/dest zone, a
// source MAC, an ipset, the address family, a limit...). The editor never
// writes nor deletes these: they are reported so the UI can say that those
// conditions are kept as they are instead of pretending the rule is only
// what the form shows.
type FWOption struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// FWRule is one firewall traffic rule. It models fw4's address-based rule
// options: a source IP/CIDR (empty = any), an optional destination IP/CIDR
// and port range, a protocol (empty = any), the action (ACCEPT/DROP/REJECT)
// and the schedule (start_time/stop_time/weekdays/utc_time). The zone-based
// options (src/dest zones) are not part of this editor: a rule that carries
// them lists them in ExtraOptions and the UI warns that they are preserved.
type FWRule struct {
	Name      string `json:"name"`
	Section   string `json:"section"`
	Proto     string `json:"proto"`
	SrcIP     string `json:"src_ip,omitempty"`
	DestIP    string `json:"dest_ip,omitempty"`
	SrcPort   string `json:"src_port,omitempty"`
	DestPort  string `json:"dest_port,omitempty"`
	Target    string `json:"target"`
	StartTime string `json:"start_time,omitempty"`
	StopTime  string `json:"stop_time,omitempty"`
	Weekdays  string `json:"weekdays,omitempty"`
	UTCTime   bool   `json:"utc_time,omitempty"`
	Enabled   bool   `json:"enabled"`
	// Managed: this rule was created by NetGrip. Only these can be deleted
	// from here; rules set up elsewhere can be edited but not removed.
	Managed bool `json:"managed"`
	// Stock: the rule ships with the firmware (its name appears in the
	// factory config). It is listed so the firewall state is not hidden,
	// but never edited: losing the DHCP/ping allowance cuts the LAN.
	Stock bool `json:"stock,omitempty"`
	// Editable: the editor can open this rule. False for firmware rules
	// and for the ones whose action or protocol the form cannot express.
	Editable bool `json:"editable"`
	// ExtraOptions: options the rule carries that this panel does not
	// model. Kept untouched on every update.
	ExtraOptions []FWOption `json:"extra_options,omitempty"`
}

type FirewallProbe struct {
	Applicable bool     `json:"applicable"`
	Zones      []FWZone `json:"zones"`
	Rules      []FWRule `json:"rules"`
	// MgmtCIDR is the LAN subnet the panel is reached from, used by the UI
	// to warn when a deny/reject rule could cut panel access.
	MgmtCIDR string `json:"mgmt_cidr,omitempty"`
}

// mgmtCIDR returns the LAN subnet (ipaddr/netmask) as a CIDR string, or ""
// when unresolvable. The panel is reached over the LAN, so this is the
// management subnet the anti-lockout check cares about.
func mgmtCIDR() string {
	return subnetCIDR(uciGet("network.lan.ipaddr"), uciGet("network.lan.netmask"))
}

// ProbeFirewall reads the firewall zones and rules. Both parse the same
// `uci show firewall` output, read once and memoized 2s (#356).
func ProbeFirewall() *FirewallProbe {
	if !executor.ServiceEnabled("firewall") {
		return &FirewallProbe{Applicable: false, Zones: []FWZone{}, Rules: []FWRule{}, MgmtCIDR: mgmtCIDR()}
	}
	out, ok := uciShowCached("firewall")
	if !ok {
		out = ""
	}
	return &FirewallProbe{
		Applicable: true,
		Zones:      parseFWZonesFrom(out),
		Rules:      applyStockRules(parseFWRulesFrom(out)),
		MgmtCIDR:   mgmtCIDR(),
	}
}

// protectedRuleNames are the bootstrap allowances every OpenWrt image ships
// and whose loss breaks the LAN: DHCP renew, IGMP/MLD, the IPsec helpers and
// the ICMPv6 baseline. They are listed so the firewall state is not hidden,
// but never edited.
//
// Why not "whatever is in /rom": on real images the factory config can carry
// the whole firewall. Verified on a GL-MT6000 running 25.12.6: /rom held every
// GL-* rule and Allow-SSH-WAN, so treating the factory file as the source of
// truth turned the user's own rules read-only and hid the editor. This set is
// small, stable and the only one whose removal takes DHCP or ping away.
var protectedRuleNames = map[string]bool{
	"Allow-DHCP-Renew":     true,
	"Allow-DHCPv6":         true,
	"Allow-IGMP":           true,
	"Allow-MLD":            true,
	"Allow-IPSec-ESP":      true,
	"Allow-ISAKMP":         true,
	"Allow-ICMPv6-Input":   true,
	"Allow-ICMPv6-Forward": true,
	"Allow-ICMPv6-Output":  true,
}

// applyStockRules flags the bootstrap rules and decides which ones the editor
// may open.
func applyStockRules(rules []FWRule) []FWRule {
	for i := range rules {
		rules[i].Stock = rules[i].Name != "" && protectedRuleNames[rules[i].Name]
		rules[i].Editable = fwRuleEditable(rules[i])
	}
	return rules
}

// fwRuleEditable reports whether the editor can represent this rule. The
// firmware rules never can. So does a rule whose action or protocol the form
// does not offer (target NOTRACK, proto icmp): opening it would rewrite those
// fields with whatever the form has, so it stays read-only instead.
func fwRuleEditable(r FWRule) bool {
	return !r.Stock && validTrafficTarget(r.Target) && validTrafficProto(r.Proto)
}

type rawZone struct {
	section string
	name    string
	input   string
	output  string
	forward string
	network []string
	masq    bool
}

// parseFWZonesFrom parses `uci show firewall` output. Sections print as
// "firewall.<section>=<type>" (anonymous sections look like "@zone[0]"),
// options as "firewall.<section>.<option>='value'". Never returns nil.
func parseFWZonesFrom(out string) []FWZone {
	zones := map[string]*rawZone{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "firewall.") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "'")

		rel := strings.TrimPrefix(key, "firewall.")
		sec, opt, hasOpt := strings.Cut(rel, ".")
		if !hasOpt {
			if val == "zone" {
				zones[sec] = &rawZone{section: sec}
			}
			continue
		}
		z, ok := zones[sec]
		if !ok {
			continue
		}
		switch opt {
		case "name":
			z.name = val
		case "input":
			z.input = val
		case "output":
			z.output = val
		case "forward":
			z.forward = val
		case "masq":
			z.masq = val == "1"
		case "network":
			z.network = append(z.network, uciListValues(line)...)
		}
	}
	result := make([]FWZone, 0, len(zones))
	for _, z := range zones {
		if z.name == "" {
			continue
		}
		if z.network == nil {
			z.network = []string{}
		}
		result = append(result, FWZone{
			Name:    z.name,
			Input:   z.input,
			Output:  z.output,
			Forward: z.forward,
			Network: z.network,
			Masq:    z.masq,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

type rawRule struct {
	section   string
	name      string
	proto     string
	srcIP     string
	destIP    string
	srcPort   string
	destPort  string
	target    string
	startTime string
	stopTime  string
	weekdays  string
	utcTime   bool
	enabled   bool
	// extra holds every option the panel does not model, by name.
	extra map[string]string
}

// parseFWRulesFrom parses `uci show firewall` output for "rule" sections.
// Section headers print as "firewall.<section>=rule". Managed is true for
// sections named netgrip_fw_N; the options the panel does not model land in
// ExtraOptions. Never returns nil.
func parseFWRulesFrom(out string) []FWRule {
	rules := map[string]*rawRule{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "firewall.") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "'")

		rel := strings.TrimPrefix(key, "firewall.")
		sec, opt, hasOpt := strings.Cut(rel, ".")
		if !hasOpt {
			if val == "rule" {
				rules[sec] = &rawRule{section: sec, enabled: true}
			}
			continue
		}
		r, ok := rules[sec]
		if !ok {
			continue
		}
		switch opt {
		case "name":
			r.name = val
		case "proto":
			r.proto = val
		case "src_ip":
			r.srcIP = val
		case "dest_ip":
			r.destIP = val
		case "src_port":
			r.srcPort = val
		case "dest_port":
			r.destPort = val
		case "target":
			r.target = val
		case "start_time":
			r.startTime = val
		case "stop_time":
			r.stopTime = val
		case "weekdays":
			r.weekdays = val
		case "utc_time":
			r.utcTime = val == "1"
		case "enabled":
			r.enabled = val != "0"
		default:
			if r.extra == nil {
				r.extra = map[string]string{}
			}
			if list := uciListValues(line); len(list) > 0 {
				r.extra[opt] = strings.Join(list, " ")
			} else {
				r.extra[opt] = val
			}
		}
	}
	result := make([]FWRule, 0, len(rules))
	for _, r := range rules {
		extra := make([]FWOption, 0, len(r.extra))
		for name, value := range r.extra {
			extra = append(extra, FWOption{Name: name, Value: value})
		}
		sort.Slice(extra, func(i, j int) bool { return extra[i].Name < extra[j].Name })
		result = append(result, FWRule{
			Name:         r.name,
			Section:      r.section,
			Proto:        r.proto,
			SrcIP:        r.srcIP,
			DestIP:       r.destIP,
			SrcPort:      r.srcPort,
			DestPort:     r.destPort,
			Target:       r.target,
			StartTime:    r.startTime,
			StopTime:     r.stopTime,
			Weekdays:     r.weekdays,
			UTCTime:      r.utcTime,
			Enabled:      r.enabled,
			Managed:      reFwRuleSection.MatchString(r.section),
			ExtraOptions: extra,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// FirewallRuleAdd is the write model for a traffic rule.
type FirewallRuleAdd struct {
	Name      string `json:"name"`
	Proto     string `json:"proto"`
	SrcIP     string `json:"src_ip"`
	DestIP    string `json:"dest_ip"`
	SrcPort   string `json:"src_port"`
	DestPort  string `json:"dest_port"`
	Target    string `json:"target"`
	StartTime string `json:"start_time"`
	StopTime  string `json:"stop_time"`
	Weekdays  string `json:"weekdays"`
	UTCTime   bool   `json:"utc_time"`
}

// validTrafficCIDR accepts an IP, a CIDR, or "" (any).
func validTrafficCIDR(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// validTrafficPorts accepts "", a single port (1-65535) or a range "a-b"
// with 1 <= a <= b <= 65535.
func validTrafficPorts(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) == 1 {
		return validPort(s)
	}
	a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	return err1 == nil && err2 == nil && a >= 1 && a <= 65535 && b >= 1 && b <= 65535 && a <= b
}

// validTrafficProto accepts "", "tcp" or "udp" (empty = any protocol).
func validTrafficProto(p string) bool {
	return p == "" || p == "tcp" || p == "udp"
}

// validTrafficTarget accepts the three policies the editor exposes.
func validTrafficTarget(t string) bool {
	return t == "ACCEPT" || t == "DROP" || t == "REJECT"
}

// validTrafficTime accepts "" or HH:MM[:SS].
func validTrafficTime(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return false
	}
	if len(parts) == 3 {
		sec, errS := strconv.Atoi(parts[2])
		if errS != nil || sec < 0 || sec > 59 {
			return false
		}
	}
	return true
}

var reWeekdays = regexp.MustCompile(`^(Mon|Tue|Wed|Thu|Fri|Sat|Sun)( (Mon|Tue|Wed|Thu|Fri|Sat|Sun))*$`)

// validTrafficWeekdays accepts "" or space-separated short day names, the
// format fw4 writes to the `weekdays` option.
func validTrafficWeekdays(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	return reWeekdays.MatchString(s)
}

// validateTrafficRule checks every field of a traffic rule before any write.
func validateTrafficRule(rule FirewallRuleAdd) error {
	if strings.TrimSpace(rule.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !validTrafficTarget(rule.Target) {
		return fmt.Errorf("target must be ACCEPT, DROP or REJECT")
	}
	if !validTrafficCIDR(rule.SrcIP) {
		return fmt.Errorf("invalid src_ip: use an IP, a CIDR or leave empty for any")
	}
	if !validTrafficCIDR(rule.DestIP) {
		return fmt.Errorf("invalid dest_ip: use an IP or a CIDR")
	}
	if !validTrafficPorts(rule.SrcPort) {
		return fmt.Errorf("invalid src_port: use 1-65535 or a range start-end")
	}
	if !validTrafficPorts(rule.DestPort) {
		return fmt.Errorf("invalid dest_port: use 1-65535 or a range start-end")
	}
	if !validTrafficProto(rule.Proto) {
		return fmt.Errorf("proto must be tcp, udp or empty (any)")
	}
	if !validTrafficTime(rule.StartTime) || !validTrafficTime(rule.StopTime) {
		return fmt.Errorf("invalid start_time/stop_time: use HH:MM or HH:MM:SS")
	}
	if !validTrafficWeekdays(rule.Weekdays) {
		return fmt.Errorf("invalid weekdays: space-separated Mon..Sun")
	}
	return nil
}

// lockoutRisk reports whether a deny/reject rule with a broad source could
// cut access to the panel. It is a UI warning, never a hard block: a
// deny/reject whose source is everything (empty or 0.0.0.0/0) or that covers
// a management subnet, and whose destination is not narrowed to a specific
// host or port, would also drop the panel's own management traffic.
func lockoutRisk(rule FirewallRuleAdd, mgmtCIDRs []string) bool {
	if rule.Target != "DROP" && rule.Target != "REJECT" {
		return false
	}
	// A destination narrowed to a host or a port does not cover the panel.
	if rule.DestIP != "" || rule.DestPort != "" || rule.SrcPort != "" {
		return false
	}
	src := strings.TrimSpace(rule.SrcIP)
	if src == "" || src == "0.0.0.0/0" || src == "0.0.0.0" {
		return true
	}
	_, srcNet, err := net.ParseCIDR(src)
	if err != nil {
		return false
	}
	srcOnes, _ := srcNet.Mask.Size()
	for _, c := range mgmtCIDRs {
		_, mgmtNet, err := net.ParseCIDR(strings.TrimSpace(c))
		if err != nil {
			continue
		}
		mgmtOnes, _ := mgmtNet.Mask.Size()
		// The source covers the management subnet when it is broader (or
		// equal) and contains its network address.
		if srcOnes <= mgmtOnes && srcNet.Contains(mgmtNet.IP) {
			return true
		}
	}
	return false
}

// nextTrafficRuleSection returns the next free netgrip_fw_N section name,
// given the existing section names.
func nextTrafficRuleSection(existing []string) string {
	max := 0
	for _, s := range existing {
		if !reFwRuleSection.MatchString(s) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(s, fwRulePrefix))
		if err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%d", fwRulePrefix, max+1)
}

// trafficRuleOps builds the UCI ops that create one named rule section. Only
// non-empty options are written (fw4 treats an absent option as its default:
// any source, any destination, any protocol). The batch ends with commit and
// a firewall reload, the same path AddFwdRule uses.
func trafficRuleOps(section string, rule FirewallRuleAdd) []executor.Op {
	base := "firewall." + section
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{base, "rule"}},
	}
	set := func(opt, val string) {
		if val != "" {
			ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + "." + opt, val}})
		}
	}
	set("name", rule.Name)
	set("proto", rule.Proto)
	set("src_ip", rule.SrcIP)
	set("dest_ip", rule.DestIP)
	set("src_port", rule.SrcPort)
	set("dest_port", rule.DestPort)
	set("target", rule.Target)
	set("start_time", rule.StartTime)
	set("stop_time", rule.StopTime)
	set("weekdays", rule.Weekdays)
	if rule.UTCTime {
		set("utc_time", "1")
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
	)
	return ops
}

// AddFirewallRule creates a named traffic rule with snapshot, reload and
// rollback (AddFwdRule / ADR-003 pattern).
func AddFirewallRule(rule FirewallRuleAdd) (*FirewallProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	if !executor.ServiceEnabled("firewall") {
		return ProbeFirewall(), false, fmt.Errorf("firewall not enabled")
	}
	if err := validateTrafficRule(rule); err != nil {
		return ProbeFirewall(), false, err
	}
	existing := []string{}
	for _, r := range ProbeFirewall().Rules {
		existing = append(existing, r.Section)
	}
	section := nextTrafficRuleSection(existing)

	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot firewall: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
		// El restore cambia la config: el probe final debe leer de nuevo,
		// no la cache poblada por la verificacion previa (#444).
		InvalidateKey("ucishow|firewall")
	}
	if err := executor.Apply(trafficRuleOps(section, rule), nil); err != nil {
		rollback()
		return ProbeFirewall(), true, err
	}
	// La lectura de "existing" de arriba repoblo la cache de `uci show
	// firewall` ANTES del alta: sin invalidar aqui, el healthcheck compara
	// contra la foto previa, no ve la seccion nueva y hace rollback de una
	// regla que si se aplico (reproducido en rt-lab, #444).
	InvalidateKey("ucishow|firewall")
	for _, r := range ProbeFirewall().Rules {
		if r.Section == section {
			return ProbeFirewall(), false, nil
		}
	}
	rollback()
	return ProbeFirewall(), true, fmt.Errorf("rule not present after reload, rolled back")
}

// trafficRuleUpdateOps rewrites the options of an existing rule. Only the
// options that change are touched, and one that must go empty is deleted:
// trafficRuleOps writes only non-empty values, so clearing a source or a
// schedule would otherwise leave the old one in place.
func trafficRuleUpdateOps(section string, current FWRule, rule FirewallRuleAdd) []executor.Op {
	base := "firewall." + section
	want := map[string]string{
		"name":       rule.Name,
		"proto":      rule.Proto,
		"src_ip":     rule.SrcIP,
		"dest_ip":    rule.DestIP,
		"src_port":   rule.SrcPort,
		"dest_port":  rule.DestPort,
		"target":     rule.Target,
		"start_time": rule.StartTime,
		"stop_time":  rule.StopTime,
		"weekdays":   rule.Weekdays,
		"utc_time":   "",
	}
	if rule.UTCTime {
		want["utc_time"] = "1"
	}
	have := map[string]string{
		"name":       current.Name,
		"proto":      current.Proto,
		"src_ip":     current.SrcIP,
		"dest_ip":    current.DestIP,
		"src_port":   current.SrcPort,
		"dest_port":  current.DestPort,
		"target":     current.Target,
		"start_time": current.StartTime,
		"stop_time":  current.StopTime,
		"weekdays":   current.Weekdays,
		"utc_time":   "",
	}
	if current.UTCTime {
		have["utc_time"] = "1"
	}

	var ops []executor.Op
	for _, opt := range []string{
		"name", "proto", "src_ip", "dest_ip", "src_port", "dest_port",
		"target", "start_time", "stop_time", "weekdays", "utc_time",
	} {
		w, h := want[opt], have[opt]
		if w == h {
			continue
		}
		if w == "" {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{base + "." + opt}})
			continue
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + "." + opt, w}})
	}
	return append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
	)
}

// planFWRuleUpdate is the single gate between the handler and the write: it
// decides whether a listed rule can be edited and builds the ops. Rules
// created outside NetGrip (LuCI, GL.iNet, by hand) are editable, like any
// forwarded port of the Puertos card; unlike those, deleting stays limited to
// NetGrip's own, because a missing rule is a change nobody can undo from here.
//
// Two kinds stay read-only: the firmware rules, whose loss would leave the
// router without DHCP or ping, and the rules the form cannot represent
// (target NOTRACK, proto icmp...), which an edit would silently rewrite.
func planFWRuleUpdate(rules []FWRule, section string, rule FirewallRuleAdd) ([]executor.Op, error) {
	var current *FWRule
	for i := range rules {
		if rules[i].Section == section {
			current = &rules[i]
			break
		}
	}
	if current == nil {
		return nil, fmt.Errorf("rule not found")
	}
	if current.Stock {
		return nil, fmt.Errorf("firmware traffic rules are read-only")
	}
	if !fwRuleEditable(*current) {
		return nil, fmt.Errorf("this rule uses an action or protocol the panel cannot edit; change it in LuCI")
	}
	return trafficRuleUpdateOps(section, *current, rule), nil
}

// UpdateFirewallRule edits a listed traffic rule in place, with the same
// validation, snapshot, reload and rollback as adding one. Rules created
// elsewhere are editable; firmware rules and the ones the form cannot
// represent are refused by planFWRuleUpdate. Delete is not part of this.
func UpdateFirewallRule(section string, rule FirewallRuleAdd) (*FirewallProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	if !executor.ServiceEnabled("firewall") {
		return ProbeFirewall(), false, fmt.Errorf("firewall not enabled")
	}
	if err := validateTrafficRule(rule); err != nil {
		return ProbeFirewall(), false, err
	}

	probe := ProbeFirewall()
	ops, err := planFWRuleUpdate(probe.Rules, section, rule)
	if err != nil {
		return probe, false, err
	}

	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot firewall: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
		InvalidateKey("ucishow|firewall")
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeFirewall(), true, err
	}
	// La comprobacion lee la cache poblada antes del cambio: hay que
	// invalidarla o el healthcheck compara con la foto vieja (#444).
	InvalidateKey("ucishow|firewall")
	for _, r := range ProbeFirewall().Rules {
		if r.Section == section && r.Target == rule.Target && r.Name == rule.Name {
			return ProbeFirewall(), false, nil
		}
	}
	rollback()
	return ProbeFirewall(), true, fmt.Errorf("rule not updated after reload, rolled back")
}

// planFWRuleDelete is the gate in front of the delete: the section has to be
// one the probe lists, and the bootstrap allowances stay. Deleting a rule
// removes the whole section, so the unrepresentable-action guard that blocks
// edits does not apply: there is nothing to rewrite.
func planFWRuleDelete(rules []FWRule, section string) error {
	for i := range rules {
		if rules[i].Section != section {
			continue
		}
		if rules[i].Stock {
			return fmt.Errorf("bootstrap traffic rules are read-only")
		}
		return nil
	}
	return fmt.Errorf("rule not found")
}

// DeleteFirewallRule deletes a listed traffic rule by section: NetGrip's own
// and the ones created elsewhere. The bootstrap allowances (DHCP renew, IGMP,
// IPsec, the ICMPv6 baseline) are refused by planFWRuleDelete.
func DeleteFirewallRule(section string) (*FirewallProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	if !executor.ServiceEnabled("firewall") {
		return ProbeFirewall(), false, fmt.Errorf("firewall not enabled")
	}
	if err := planFWRuleDelete(ProbeFirewall().Rules, section); err != nil {
		return ProbeFirewall(), false, err
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot firewall: %w", err)
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
		return ProbeFirewall(), true, err
	}
	// Sin invalidar, la verificacion leeria la cache previa al borrado y
	// daria la seccion por aun presente (#444).
	InvalidateKey("ucishow|firewall")
	for _, r := range ProbeFirewall().Rules {
		if r.Section == section {
			rollback()
			return ProbeFirewall(), true, fmt.Errorf("rule still present after delete, rolled back")
		}
	}
	return ProbeFirewall(), false, nil
}
