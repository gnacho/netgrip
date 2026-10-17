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
// rule (LuCI, the firmware factory config): listed read-only, never edited
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

// FWRule is one firewall traffic rule. It models fw4's address-based rule
// options: a source IP/CIDR (empty = any), an optional destination IP/CIDR
// and port range, a protocol (empty = any), the action (ACCEPT/DROP/REJECT)
// and the schedule (start_time/stop_time/weekdays/utc_time). The zone-based
// options (src/dest zones) are not part of this editor; system rules that use
// them still show here read-only, with empty address fields.
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
	// Managed: this rule was created by NetGrip and can be edited/removed
	// here. Anything set up in LuCI or shipped by the firmware is read-only.
	Managed bool `json:"managed"`
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
		Rules:      parseFWRulesFrom(out),
		MgmtCIDR:   mgmtCIDR(),
	}
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
}

// parseFWRulesFrom parses `uci show firewall` output for "rule" sections.
// Section headers print as "firewall.<section>=rule". Managed is true for
// sections named netgrip_fw_N. Never returns nil.
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
		}
	}
	result := make([]FWRule, 0, len(rules))
	for _, r := range rules {
		result = append(result, FWRule{
			Name:      r.name,
			Section:   r.section,
			Proto:     r.proto,
			SrcIP:     r.srcIP,
			DestIP:    r.destIP,
			SrcPort:   r.srcPort,
			DestPort:  r.destPort,
			Target:    r.target,
			StartTime: r.startTime,
			StopTime:  r.stopTime,
			Weekdays:  r.weekdays,
			UTCTime:   r.utcTime,
			Enabled:   r.enabled,
			Managed:   reFwRuleSection.MatchString(r.section),
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

// DeleteFirewallRule deletes one managed traffic rule by section.
func DeleteFirewallRule(section string) (*FirewallProbe, bool, error) {
	InvalidateKey("ucishow|firewall")
	if !reFwRuleSection.MatchString(section) {
		return ProbeFirewall(), false, fmt.Errorf("only NetGrip traffic rules can be deleted")
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
