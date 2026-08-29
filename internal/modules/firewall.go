package modules

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type FWZone struct {
	Name    string   `json:"name"`
	Input   string   `json:"input"`
	Output  string   `json:"output"`
	Forward string   `json:"forward"`
	Network []string `json:"network"`
	Masq    bool     `json:"masq"`
}

type FWRule struct {
	Name     string `json:"name"`
	Section  string `json:"section"`
	Src      string `json:"src"`
	Dest     string `json:"dest"`
	Proto    string `json:"proto"`
	DestPort string `json:"dest_port"`
	Target   string `json:"target"`
}

type FirewallProbe struct {
	Applicable bool     `json:"applicable"`
	Zones      []FWZone `json:"zones"`
	Rules      []FWRule `json:"rules"`
}

func ProbeFirewall() *FirewallProbe {
	if !executor.ServiceEnabled("firewall") {
		return &FirewallProbe{Applicable: false}
	}
	p := &FirewallProbe{Applicable: true}
	p.Zones = parseFWZones()
	p.Rules = parseFWRules()
	return p
}

func parseFWZones() []FWZone {
	out, err := exec.Command("uci", "show", "firewall").Output()
	if err != nil {
		return nil
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
	zones := map[string]*rawZone{}
	for _, line := range strings.Split(string(out), "\n") {
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

		if strings.HasSuffix(key, ".type") && val == "zone" {
			sec := strings.TrimPrefix(strings.TrimSuffix(key, ".type"), "firewall.")
			zones[sec] = &rawZone{section: sec}
		}
		for sec, z := range zones {
			prefix := "firewall." + sec + "."
			switch {
			case key == prefix+"name":
				z.name = val
			case key == prefix+"input":
				z.input = val
			case key == prefix+"output":
				z.output = val
			case key == prefix+"forward":
				z.forward = val
			case key == prefix+"masq" && val == "1":
				z.masq = true
			case strings.HasPrefix(key, prefix+"network"):
				if strings.Contains(key, "=") {
					z.network = append(z.network, val)
				}
			}
		}
	}
	var result []FWZone
	for _, z := range zones {
		if z.name == "" {
			continue
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

func parseFWRules() []FWRule {
	out, err := exec.Command("uci", "show", "firewall").Output()
	if err != nil {
		return nil
	}
	type rawRule struct {
		section  string
		name     string
		src      string
		dest     string
		proto    string
		destPort string
		target   string
	}
	rules := map[string]*rawRule{}
	for _, line := range strings.Split(string(out), "\n") {
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

		if strings.HasSuffix(key, ".type") && val == "rule" {
			sec := strings.TrimPrefix(strings.TrimSuffix(key, ".type"), "firewall.")
			rules[sec] = &rawRule{section: sec}
		}
		for sec, r := range rules {
			prefix := "firewall." + sec + "."
			switch {
			case key == prefix+"name":
				r.name = val
			case key == prefix+"src":
				r.src = val
			case key == prefix+"dest":
				r.dest = val
			case key == prefix+"proto":
				r.proto = val
			case key == prefix+"dest_port":
				r.destPort = val
			case key == prefix+"target":
				r.target = val
			}
		}
	}
	var result []FWRule
	for _, r := range rules {
		result = append(result, FWRule{
			Name:     r.name,
			Section:  r.section,
			Src:      r.src,
			Dest:     r.dest,
			Proto:    r.proto,
			DestPort: r.destPort,
			Target:   r.target,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

type FirewallRuleAdd struct {
	Name     string `json:"name"`
	Src      string `json:"src"`
	Dest     string `json:"dest"`
	Proto    string `json:"proto"`
	DestPort string `json:"dest_port"`
	Target   string `json:"target"`
}

func AddFirewallRule(rule FirewallRuleAdd) (*FirewallProbe, bool, error) {
	if !executor.ServiceEnabled("firewall") {
		return nil, false, fmt.Errorf("firewall not enabled")
	}
	if rule.Name == "" || rule.Proto == "" || rule.DestPort == "" {
		return nil, false, fmt.Errorf("name, proto and dest_port required")
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot firewall: %w", err)
	}

	cmd := exec.Command("uci", "add", "firewall", "rule")
	secOut, err := cmd.Output()
	if err != nil {
		return ProbeFirewall(), false, fmt.Errorf("uci add rule: %w", err)
	}
	section := strings.TrimSpace(string(secOut))

	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"firewall." + section + ".name", rule.Name}},
		{Kind: "uci_set", Args: []string{"firewall." + section + ".src", rule.Src}},
		{Kind: "uci_set", Args: []string{"firewall." + section + ".proto", rule.Proto}},
		{Kind: "uci_set", Args: []string{"firewall." + section + ".dest_port", rule.DestPort}},
		{Kind: "uci_set", Args: []string{"firewall." + section + ".target", rule.Target}},
	}
	if rule.Dest != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"firewall." + section + ".dest", rule.Dest}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "restart"}},
	)

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("firewall", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "restart"}})
		return ProbeFirewall(), true, err
	}
	return ProbeFirewall(), false, nil
}

func DeleteFirewallRule(section string) (*FirewallProbe, bool, error) {
	if section == "" {
		return nil, false, fmt.Errorf("section required")
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot firewall: %w", err)
	}
	ops := []executor.Op{
		{Kind: "uci_delete", Args: []string{"firewall." + section}},
		{Kind: "uci_commit", Args: []string{"firewall"}},
		{Kind: "initd", Args: []string{"firewall", "restart"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("firewall", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "restart"}})
		return ProbeFirewall(), true, err
	}
	return ProbeFirewall(), false, nil
}
