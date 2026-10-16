package modules

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// UPnP/NAT-PMP integration (#448). miniupnpd owns its nft table; NetGrip only
// reads UCI (/etc/config/upnpd) and drives /etc/init.d/miniupnpd
// (start/stop/reload/restart/enable/disable). It never writes nft rules.
//
// Options used below are the ones the OpenWrt package ships (openwrt/packages
// net/miniupnpd, files/upnpd.config): enabled, enable_natpmp, enable_upnp,
// secure_mode, internal_iface, upnp_lease_file and the perm_rule ACL sections
// (action, ext_ports, int_addr, int_ports, comment). secure_mode=1 restricts
// a mapping to the requesting LAN host and is the "restricted to LAN" switch.

const (
	upnpInitd        = "/etc/init.d/miniupnpd"
	upnpLeaseFileDef = "/var/run/miniupnpd.leases"
	upnpRulePrefix   = "netgrip_upnp_"
)

// UpnpLease is one active port mapping read from the miniupnpd lease file.
type UpnpLease struct {
	Proto     string `json:"proto"` // "TCP" or "UDP"
	ExtPort   int    `json:"ext_port"`
	IntAddr   string `json:"int_addr"`
	IntPort   int    `json:"int_port"`
	LeaseTime int64  `json:"lease_time"` // seconds remaining; 0 = no expiry
	Desc      string `json:"desc"`
}

// UpnpRule is one perm_rule (ACL) entry. Miniupnpd applies these before a
// mapping is accepted: an allow rule permits, a deny rule rejects.
type UpnpRule struct {
	Section  string `json:"section"`
	Action   string `json:"action"` // "allow" or "deny"
	IntAddr  string `json:"int_addr"`
	IntPorts string `json:"int_ports"`
	ExtPorts string `json:"ext_ports"`
	Comment  string `json:"comment"`
	Managed  bool   `json:"managed"`
}

// UpnpProbe is the API view of the UPnP/NAT-PMP page.
type UpnpProbe struct {
	Installed  bool        `json:"installed"`
	Enabled    bool        `json:"enabled"` // upnpd.config.enabled
	Running    bool        `json:"running"`
	Applicable bool        `json:"applicable"` // gateway with an uplink
	SecureMode bool        `json:"secure_mode"`
	NatPmp     bool        `json:"natpmp"` // enable_natpmp
	Upnp       bool        `json:"upnp"`   // enable_upnp
	LeaseFile  string      `json:"lease_file"`
	Leases     []UpnpLease `json:"leases"`
	Rules      []UpnpRule  `json:"rules"`
}

// UpnpStatus is the lightweight status for the Services overview card.
type UpnpStatus struct {
	Installed  bool `json:"installed"`
	Enabled    bool `json:"enabled"`
	Running    bool `json:"running"`
	Applicable bool `json:"applicable"`
}

// upnpInstalled reports whether the miniupnpd init script is present.
func upnpInstalled() bool {
	_, err := os.Stat(upnpInitd)
	return err == nil
}

// upnpSections parses `uci show upnpd` once into its sections.
func upnpSections() map[string]uciSection {
	out, err := exec.Command("uci", "show", "upnpd").Output()
	if err != nil {
		return nil
	}
	return parseUCIShow(string(out), "upnpd")
}

// upnpConfigOpts returns the options of the "config" section (the named
// master section the stock config ships), falling back to whatever section
// carries upnpd options.
func upnpConfigOpts(sections map[string]uciSection) map[string][]string {
	if s, ok := sections["config"]; ok {
		return s.Options
	}
	for _, s := range sections {
		if len(s.Options) > 0 {
			return s.Options
		}
	}
	return nil
}

// upnpOpt returns the first value of an option, or "" when unset.
func upnpOpt(opts map[string][]string, key string) string {
	if v := opts[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// upnpRulesFromSections builds the ACL list from the parsed sections. Both
// anonymous (@perm_rule[N]) and named sections count; only the named ones
// prefixed netgrip_upnp_ are managed (removable) by the panel.
func upnpRulesFromSections(sections map[string]uciSection) []UpnpRule {
	var out []UpnpRule
	for name, s := range sections {
		if s.Type != "perm_rule" {
			continue
		}
		out = append(out, UpnpRule{
			Section:  name,
			Action:   s.Option("action"),
			IntAddr:  s.Option("int_addr"),
			IntPorts: s.Option("int_ports"),
			ExtPorts: s.Option("ext_ports"),
			Comment:  s.Option("comment"),
			Managed:  strings.HasPrefix(name, upnpRulePrefix),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Section < out[j].Section })
	return out
}

// upnpLeaseFilePath resolves the lease file from UCI with the package default
// as fallback.
func upnpLeaseFilePath(opts map[string][]string) string {
	if v := upnpOpt(opts, "upnp_lease_file"); v != "" {
		return v
	}
	return upnpLeaseFileDef
}

// parseUpnpLeaseFile parses miniupnpd's lease file. Each line has the form
// PROTO:ext_port:int_addr:int_port:timestamp:desc where timestamp is unix time
// in seconds (0 = no expiry). The description is everything after the fifth
// colon, so it may itself contain colons. Malformed lines are skipped
// (fail-soft). Pure on its arguments so tests run off-router.
func parseUpnpLeaseFile(content string, now time.Time) []UpnpLease {
	var out []UpnpLease
	nowUnix := now.Unix()
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 6)
		if len(parts) != 6 {
			continue
		}
		proto := strings.ToUpper(parts[0])
		if proto != "TCP" && proto != "UDP" && proto != "UDPLITE" {
			continue
		}
		ext, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		intPort, err := strconv.Atoi(parts[3])
		if err != nil {
			continue
		}
		ts, _ := strconv.ParseInt(parts[4], 10, 64)
		lease := int64(0)
		if ts > 0 {
			lease = ts - nowUnix
			if lease < 0 {
				lease = 0
			}
		}
		out = append(out, UpnpLease{
			Proto:     proto,
			ExtPort:   ext,
			IntAddr:   parts[2],
			IntPort:   intPort,
			LeaseTime: lease,
			Desc:      strings.TrimSpace(parts[5]),
		})
	}
	return out
}

// ProbeUpnpStatus reads only installed/enabled/running, cheap on purpose (the
// Services page renders several cards at once).
func ProbeUpnpStatus() *UpnpStatus {
	s := &UpnpStatus{Installed: upnpInstalled(), Applicable: hasUplink()}
	if !s.Installed {
		return s
	}
	s.Enabled = uciGet("upnpd.config.enabled") == "1"
	s.Running = executor.ServiceRunning("miniupnpd")
	return s
}

// ProbeUpnp reads the full UPnP state. Every read is fail-soft: a missing
// lease file or uci package is an omitted field, never an error.
func ProbeUpnp() *UpnpProbe {
	p := &UpnpProbe{
		Installed:  upnpInstalled(),
		Applicable: hasUplink(),
		Leases:     []UpnpLease{},
		Rules:      []UpnpRule{},
	}
	if !p.Installed {
		return p
	}
	sections := upnpSections()
	opts := upnpConfigOpts(sections)
	p.Enabled = upnpOpt(opts, "enabled") == "1"
	p.SecureMode = upnpOpt(opts, "secure_mode") != "0"
	p.NatPmp = upnpOpt(opts, "enable_natpmp") != "0"
	p.Upnp = upnpOpt(opts, "enable_upnp") != "0"
	p.LeaseFile = upnpLeaseFilePath(opts)
	p.Running = executor.ServiceRunning("miniupnpd")
	p.Rules = upnpRulesFromSections(sections)

	if data, err := os.ReadFile(p.LeaseFile); err == nil {
		p.Leases = parseUpnpLeaseFile(string(data), time.Now())
	}
	return p
}

// validUpnpActions are the service actions the page exposes. enable/disable
// toggle the enabled master switch in UCI; the rest map to the init script.
var validUpnpActions = map[string]bool{
	"start": true, "stop": true, "reload": true, "restart": true,
	"enable": true, "disable": true,
}

// UpnpAction runs one service action through the executor and verifies the
// resulting state, reversing the action as rollback when the healthcheck
// fails.
func UpnpAction(action string) (*UpnpProbe, bool, error) {
	if !validUpnpActions[action] {
		return ProbeUpnp(), false, fmt.Errorf("unsupported action %q", action)
	}
	if !upnpInstalled() {
		return ProbeUpnp(), false, fmt.Errorf("upnpd is not installed")
	}
	before := ProbeUpnpStatus()
	var ops []executor.Op
	switch action {
	case "enable":
		snap, _ := executor.Snapshot("upnpd")
		// secure_mode=1 restricts each mapping to the requesting LAN host.
		// It is forced on here so enabling UPnP never exposes the whole LAN.
		ops = []executor.Op{
			{Kind: "uci_set", Args: []string{"upnpd.config.enabled", "1"}},
			{Kind: "uci_set", Args: []string{"upnpd.config.secure_mode", "1"}},
			{Kind: "uci_commit", Args: []string{"upnpd"}},
			{Kind: "initd", Args: []string{"miniupnpd", "enable"}},
			{Kind: "initd", Args: []string{"miniupnpd", "start"}},
		}
		if err := executor.Apply(ops, nil); err != nil {
			if snap != "" {
				_ = executor.Restore("upnpd", snap)
			}
			return ProbeUpnp(), true, err
		}
		return upnpActionVerify("enable", before)
	case "disable":
		snap, _ := executor.Snapshot("upnpd")
		ops = []executor.Op{
			{Kind: "uci_set", Args: []string{"upnpd.config.enabled", "0"}},
			{Kind: "uci_commit", Args: []string{"upnpd"}},
			{Kind: "initd", Args: []string{"miniupnpd", "stop"}},
			{Kind: "initd", Args: []string{"miniupnpd", "disable"}},
		}
		if err := executor.Apply(ops, nil); err != nil {
			if snap != "" {
				_ = executor.Restore("upnpd", snap)
			}
			return ProbeUpnp(), true, err
		}
		return upnpActionVerify("disable", before)
	default:
		ops = []executor.Op{{Kind: "initd", Args: []string{"miniupnpd", action}}}
		if err := executor.Apply(ops, nil); err != nil {
			return ProbeUpnp(), false, err
		}
		return upnpActionVerify(action, before)
	}
}

// upnpActionVerify healthchecks a service action by polling the light status.
// reload/restart keep the previous running state; start/enable must come up,
// stop/disable must go down.
func upnpActionVerify(action string, before *UpnpStatus) (*UpnpProbe, bool, error) {
	want := before.Running
	switch action {
	case "start", "enable":
		want = true
	case "stop", "disable":
		want = false
	}
	for i := 0; i < 30; i++ {
		st := ProbeUpnpStatus()
		if action == "enable" {
			if st.Enabled && st.Running {
				return ProbeUpnp(), false, nil
			}
		} else if action == "disable" {
			if !st.Enabled && !st.Running {
				return ProbeUpnp(), false, nil
			}
		} else if st.Running == want {
			return ProbeUpnp(), false, nil
		}
		time.Sleep(time.Second)
	}
	probe := ProbeUpnp()
	reverse := map[string]string{"start": "stop", "stop": "start", "enable": "disable", "disable": "enable"}
	if r, ok := reverse[action]; ok {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"miniupnpd", r}})
		if action == "enable" || action == "disable" {
			val := "0"
			if action == "disable" {
				val = "1"
			}
			_ = executor.Run(executor.Op{Kind: "uci_set", Args: []string{"upnpd.config.enabled", val}})
			_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"upnpd"}})
		}
	}
	return probe, true, fmt.Errorf("healthcheck failed after %q, rolled back", action)
}

// UpnpRuleConfig is the write model for a perm_rule (ACL entry).
type UpnpRuleConfig struct {
	Action   string `json:"action"` // "allow" or "deny"
	IntAddr  string `json:"int_addr"`
	IntPorts string `json:"int_ports"`
	ExtPorts string `json:"ext_ports"`
	Comment  string `json:"comment"`
}

// reUpnpPortRange matches a port range as miniupnpd accepts them: a single
// port, "a-b" or "a:b".
var reUpnpPortRange = regexp.MustCompile(`^\d{1,5}([-:]\d{1,5})?$`)

// reUpnpBadChars mirrors the executor's guard for UCI values.
var reUpnpBadChars = regexp.MustCompile("[;\n\r`]")

func validUpnpIntAddr(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

func validUpnpComment(s string) bool {
	return len(s) <= 128 && !reUpnpBadChars.MatchString(s)
}

// upnpRuleOps builds the UCI ops that add one perm_rule. Rules use a named
// section (netgrip_upnp_<n>) so re-applies are idempotent and the rule is
// removable, matching the ADR-003 preference for named sections.
func upnpRuleOps(section string, cfg UpnpRuleConfig) ([]executor.Op, error) {
	if cfg.Action != "allow" && cfg.Action != "deny" {
		return nil, fmt.Errorf("invalid action %q: use allow or deny", cfg.Action)
	}
	if !validUpnpIntAddr(cfg.IntAddr) {
		return nil, fmt.Errorf("invalid int_addr %q: use an IP or CIDR", cfg.IntAddr)
	}
	for _, ports := range []string{cfg.IntPorts, cfg.ExtPorts} {
		if ports != "" && !reUpnpPortRange.MatchString(ports) {
			return nil, fmt.Errorf("invalid port range %q", ports)
		}
	}
	if cfg.Comment != "" && !validUpnpComment(cfg.Comment) {
		return nil, fmt.Errorf("invalid comment")
	}
	base := "upnpd." + section
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{base, "perm_rule"}},
		{Kind: "uci_set", Args: []string{base + ".action", cfg.Action}},
		{Kind: "uci_set", Args: []string{base + ".int_addr", cfg.IntAddr}},
	}
	if cfg.IntPorts != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".int_ports", cfg.IntPorts}})
	}
	if cfg.ExtPorts != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".ext_ports", cfg.ExtPorts}})
	}
	if cfg.Comment != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".comment", cfg.Comment}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"upnpd"}},
		executor.Op{Kind: "initd", Args: []string{"miniupnpd", "reload"}},
	)
	return ops, nil
}

// upnpNextRuleSection returns the next free netgrip_upnp_<n> section name.
func upnpNextRuleSection(sections map[string]uciSection) string {
	max := 0
	for name := range sections {
		if !strings.HasPrefix(name, upnpRulePrefix) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(name, upnpRulePrefix)); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%d", upnpRulePrefix, max+1)
}

// SetUpnpRule adds (add=true) or deletes (add=false) one ACL entry and
// reloads miniupnpd. Deletes only target the panel's own netgrip_upnp_*
// sections; stock rules are read-only.
func SetUpnpRule(add bool, cfg UpnpRuleConfig, section string) (*UpnpProbe, bool, error) {
	if !upnpInstalled() {
		return ProbeUpnp(), false, fmt.Errorf("upnpd is not installed")
	}
	if !add {
		if !strings.HasPrefix(section, upnpRulePrefix) {
			return ProbeUpnp(), false, fmt.Errorf("not a netgrip rule")
		}
		snap, _ := executor.Snapshot("upnpd")
		ops := []executor.Op{
			{Kind: "uci_delete", Args: []string{"upnpd." + section}},
			{Kind: "uci_commit", Args: []string{"upnpd"}},
			{Kind: "initd", Args: []string{"miniupnpd", "reload"}},
		}
		if err := executor.Apply(ops, nil); err != nil {
			if snap != "" {
				_ = executor.Restore("upnpd", snap)
			}
			return ProbeUpnp(), true, err
		}
		for _, r := range ProbeUpnp().Rules {
			if r.Section == section {
				if snap != "" {
					_ = executor.Restore("upnpd", snap)
				}
				return ProbeUpnp(), true, fmt.Errorf("rule still present after delete, rolled back")
			}
		}
		return ProbeUpnp(), false, nil
	}

	sections := upnpSections()
	section = upnpNextRuleSection(sections)
	ops, err := upnpRuleOps(section, cfg)
	if err != nil {
		return ProbeUpnp(), false, err
	}
	snap, _ := executor.Snapshot("upnpd")
	if err := executor.Apply(ops, nil); err != nil {
		if snap != "" {
			_ = executor.Restore("upnpd", snap)
		}
		return ProbeUpnp(), true, err
	}
	for _, r := range ProbeUpnp().Rules {
		if r.Section == section {
			return ProbeUpnp(), false, nil
		}
	}
	if snap != "" {
		_ = executor.Restore("upnpd", snap)
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"miniupnpd", "reload"}})
	return ProbeUpnp(), true, fmt.Errorf("healthcheck failed after rule change, rolled back")
}

// InstallUpnp installs the miniupnpd package after explicit confirmation
// from the UI (disk/RAM impact is stated there). Never called implicitly.
func InstallUpnp(confirm bool) (*UpnpProbe, error) {
	if !confirm {
		return ProbeUpnp(), fmt.Errorf("installation needs explicit confirmation")
	}
	if upnpInstalled() {
		return ProbeUpnp(), nil
	}
	if err := executor.Run(executor.Op{Kind: "pkg_add", Args: []string{"miniupnpd"}}); err != nil {
		return ProbeUpnp(), fmt.Errorf("install miniupnpd: %w", err)
	}
	return ProbeUpnp(), nil
}

// UninstallUpnp removes the miniupnpd package after explicit confirmation.
// The service is stopped and its rc.d autostart removed before the package,
// so no half-removed state is left behind. The UCI config (/etc/config/upnpd)
// stays on disk, as opkg keeps conffiles: the UI says so in the confirmation.
func UninstallUpnp(confirm bool) (*UpnpProbe, error) {
	if !confirm {
		return ProbeUpnp(), fmt.Errorf("uninstall needs explicit confirmation")
	}
	if !upnpInstalled() {
		return ProbeUpnp(), nil
	}
	var ops []executor.Op
	if executor.ServiceRunning("miniupnpd") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"miniupnpd", "stop"}})
	}
	if executor.ServiceEnabled("miniupnpd") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"miniupnpd", "disable"}})
	}
	ops = append(ops, executor.Op{Kind: "pkg_del", Args: []string{"miniupnpd"}})
	if err := executor.Apply(ops, nil); err != nil {
		return ProbeUpnp(), fmt.Errorf("uninstall miniupnpd: %w", err)
	}
	return ProbeUpnp(), nil
}
