package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/owpanel/internal/executor"
)

// RemoteAccess is the read-only state of the three remote-access firewalling
// knobs: allow ping from WAN, remote HTTPS and remote SSH.
type RemoteAccess struct {
	Applicable  bool `json:"applicable"`
	PingWAN     bool `json:"ping_wan"`
	RemoteHTTPS bool `json:"remote_https"`
	RemoteSSH   bool `json:"remote_ssh"`
}

const (
	remotePingIdx  = "allow_ping" // uses the stock Allow-Ping rule
	remoteHTTPSIdx = "owpanel_remote_https"
	remoteSSHIdx   = "owpanel_remote_ssh"
)

// ProbeRemoteAccess reads whether the router is the gateway, plus the current
// state of the three toggles.
func ProbeRemoteAccess() *RemoteAccess {
	p := &RemoteAccess{
		Applicable:  fwdApplicableCheck(),
		PingWAN:     panicOnPingRule(),
		RemoteHTTPS: remoteRuleTarget(remoteHTTPSIdx) == "ACCEPT",
		RemoteSSH:   remoteRuleTarget(remoteSSHIdx) == "ACCEPT",
	}
	return p
}

// fwdApplicableCheck reports whether remote-access rules make sense here.
func fwdApplicableCheck() bool {
	q := ProbeFwd()
	return q.HasWan && q.Firewall
}

// panicOnPingRule reports the effective state of WAN ping: the stock
// OpenWrt config ships an Allow-Ping rule (ACCEPT) unless it was swapped.
func panicOnPingRule() bool {
	for _, r := range listFirewallRules() {
		if r.Name == "Allow-Ping" {
			return r.Target == "ACCEPT" || r.Target == ""
		}
	}
	// Not present: either never added (deny) or a custom rule name.
	return remoteRuleTarget(remotePingIdx) == "ACCEPT"
}

type fwRule struct {
	Index  string
	Name   string
	Target string
}

func listFirewallRules() []fwRule {
	return allFirewallRules()
}

// SetRemoteAccess applies a remote-access toggle with snapshot, reload and
// rollback. Only one toggle is applied per call (the non-nil flag wins).
func SetRemoteAccess(pingWAN, remoteHTTPS, remoteSSH *bool) (*RemoteAccess, bool, error) {
	probe := ProbeRemoteAccess()
	if !probe.Applicable {
		return probe, false, fmt.Errorf("remote access only applies on the gateway (needs WAN and firewall)")
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return probe, false, fmt.Errorf("snapshot firewall: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}

	var ops []executor.Op
	if pingWAN != nil {
		ops = append(ops, pingWANOps(*pingWAN)...)
	}
	if remoteHTTPS != nil {
		ops = append(ops, remoteRuleOps("https", *remoteHTTPS)...)
	}
	if remoteSSH != nil {
		ops = append(ops, remoteRuleOps("ssh", *remoteSSH)...)
	}
	if len(ops) == 0 {
		return probe, false, fmt.Errorf("no toggle requested")
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeRemoteAccess(), true, err
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})

	after := ProbeRemoteAccess()
	if !remoteAccessHealthy(after, pingWAN, remoteHTTPS, remoteSSH) {
		rollback()
		return ProbeRemoteAccess(), true, fmt.Errorf("remote access healthcheck failed, rolled back")
	}
	return after, false, nil
}

func remoteAccessHealthy(after *RemoteAccess, pingWAN, remoteHTTPS, remoteSSH *bool) bool {
	if pingWAN != nil && after.PingWAN != *pingWAN {
		return false
	}
	if remoteHTTPS != nil && after.RemoteHTTPS != *remoteHTTPS {
		return false
	}
	if remoteSSH != nil && after.RemoteSSH != *remoteSSH {
		return false
	}
	return true
}

func pingWANOps(accept bool) []executor.Op {
	// Toggle the stock Allow-Ping rule's target; create it if absent.
	idx := findRuleIndex("Allow-Ping")
	if accept {
		if idx == "" {
			return []executor.Op{
				{Kind: "uci_set", Args: []string{"firewall." + remotePingIdx, "rule"}},
				{Kind: "uci_set", Args: []string{"firewall." + remotePingIdx + ".name", "Allow-Ping"}},
				{Kind: "uci_set", Args: []string{"firewall." + remotePingIdx + ".src", "wan"}},
				{Kind: "uci_set", Args: []string{"firewall." + remotePingIdx + ".proto", "icmp"}},
				{Kind: "uci_set", Args: []string{"firewall." + remotePingIdx + ".icmp_type", "echo-request"}},
				{Kind: "uci_set", Args: []string{"firewall." + remotePingIdx + ".target", "ACCEPT"}},
				{Kind: "uci_commit", Args: []string{"firewall"}},
			}
		}
		return []executor.Op{
			{Kind: "uci_set", Args: []string{"firewall." + idx + ".target", "ACCEPT"}},
			{Kind: "uci_commit", Args: []string{"firewall"}},
		}
	}
	if idx == "" {
		// Nothing to deny: ping is already off (rule absent).
		return nil
	}
	return []executor.Op{
		{Kind: "uci_set", Args: []string{"firewall." + idx + ".target", "DROP"}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
	}
}

func remoteRuleOps(kind string, accept bool) []executor.Op {
	idx := remoteHTTPSIdx
	proto := "tcp"
	port := "443"
	if kind == "ssh" {
		idx = remoteSSHIdx
		port = "22"
	}
	target := "DROP"
	if accept {
		target = "ACCEPT"
	}
	return []executor.Op{
		{Kind: "uci_set", Args: []string{"firewall." + idx, "rule"}},
		{Kind: "uci_set", Args: []string{"firewall." + idx + ".name", "owpanel-remote-" + kind}},
		{Kind: "uci_set", Args: []string{"firewall." + idx + ".src", "wan"}},
		{Kind: "uci_set", Args: []string{"firewall." + idx + ".proto", proto}},
		{Kind: "uci_set", Args: []string{"firewall." + idx + ".dest_port", port}},
		{Kind: "uci_set", Args: []string{"firewall." + idx + ".target", target}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
	}
}

func remoteRuleTarget(idx string) string {
	target := uciGet("firewall." + idx + ".target")
	return target
}

func findRuleIndex(name string) string {
	for _, r := range allFirewallRules() {
		if r.Name == name {
			return r.Index
		}
	}
	return ""
}

func allFirewallRules() []fwRule {
	raw, err := exec.Command("sh", "-c", "uci show firewall | grep '=rule' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return nil
	}
	var rules []fwRule
	for _, index := range strings.Fields(string(raw)) {
		rules = append(rules, fwRule{
			Index:  index,
			Name:   uciGet("firewall." + index + ".name"),
			Target: uciGet("firewall." + index + ".target"),
		})
	}
	return rules
}
