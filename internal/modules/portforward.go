package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
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
}

// FwdProbe is the read-only port forwarding state.
type FwdProbe struct {
	HasWan   bool      `json:"has_wan"`
	Firewall bool      `json:"firewall"`
	Rules    []FwdRule `json:"rules"`
}

var (
	reIPv4       = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	reFwdSection = regexp.MustCompile(`^` + fwdPrefix + `[a-z0-9_]+$`)
)

// ProbeFwd reads the port forwarding state.
func ProbeFwd() *FwdProbe {
	p := &FwdProbe{
		HasWan:   uciSectionExists("network.wan"),
		Firewall: executor.ServiceEnabled("firewall"),
		Rules:    []FwdRule{},
	}
	out, err := exec.Command("sh", "-c", "uci show firewall | grep '=redirect' | cut -d. -f2 | cut -d= -f1 | grep '^"+fwdPrefix+"'").Output()
	if err != nil {
		return p
	}
	for _, section := range strings.Fields(string(out)) {
		base := "firewall." + section
		p.Rules = append(p.Rules, FwdRule{
			Section:  section,
			Name:     uciGet(base + ".name"),
			SrcDport: uciGet(base + ".src_dport"),
			DestIP:   uciGet(base + ".dest_ip"),
			DestPort: uciGet(base + ".dest_port"),
			Proto:    uciGet(base + ".proto"),
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

	section := fwdPrefix + srcDport + "_" + strings.ReplaceAll(destIP, ".", "_")
	base := "firewall." + section
	proto = strings.ReplaceAll(proto, "tcpudp", "tcp udp")
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{base, "redirect"}},
		{Kind: "uci_set", Args: []string{base + ".name", "netgrip-fwd-" + srcDport}},
		{Kind: "uci_set", Args: []string{base + ".src", "wan"}},
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

// RemoveFwdRule deletes a redirect rule by section.
func RemoveFwdRule(section string) (*FwdProbe, bool, error) {
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
