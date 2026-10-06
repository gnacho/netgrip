package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// SNMP (#442): manage the OpenWrt snmpd daemon (net-snmp) from the Advanced
// section. Config lives in UCI (/etc/config/snmpd); the init script
// regenerates snmpd.conf from there on restart.
//
// Ownership rule, same spirit as the AdGuard credentials work: a config with
// section types we do not manage (smux, traps, users...) reports
// Managed=false and is never touched. The stock community sections that ship
// with the package are removed on the first panel-managed apply, so enabling
// SNMP from the panel never leaves the default 'public' community open.
type SNMPProbe struct {
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	Enabled     bool   `json:"enabled"`
	Managed     bool   `json:"managed"`
	Location    string `json:"location"`
	Contact     string `json:"contact"`
	Listen      string `json:"listen"`
	CommunityRO string `json:"community_ro,omitempty"`
	CommunityRW string `json:"community_rw,omitempty"`
}

type SNMPRequest struct {
	Enabled     bool   `json:"enabled"`
	Location    string `json:"location"`
	Contact     string `json:"contact"`
	Listen      string `json:"listen"`
	CommunityRO string `json:"community_ro"`
	CommunityRW string `json:"community_rw"`
}

// snmpdManagedSections are the only section types the panel writes.
var snmpdManagedSections = map[string]bool{
	"agent":     true,
	"community": true,
}

// Section names for the two communities the panel manages. Named (not
// anonymous) sections so re-applying never duplicates them.
const (
	snmpROSection = "netgrip_ro"
	snmpRWSection = "netgrip_rw"
)

// ProbeSNMP reports the daemon and config state. A missing package or config
// reads as a clean "not installed" state.
func ProbeSNMP() *SNMPProbe {
	p := &SNMPProbe{
		Installed: pkgInstalled("snmpd"),
		Running:   executor.ServiceRunning("snmpd"),
		Enabled:   executor.ServiceEnabled("snmpd"),
	}
	if !p.Installed {
		return p
	}
	sections := snmpdSections()
	if sections == nil {
		return p
	}
	p.Managed = snmpdConfigManaged(sections)
	if agent, ok := sections["agent"]; ok {
		p.Location = agent.Option("sysLocation")
		p.Contact = agent.Option("sysContact")
		p.Listen = agent.Option("agentaddress")
	}
	for name, s := range sections {
		if s.Type != "community" {
			continue
		}
		switch name {
		case snmpROSection:
			p.CommunityRO = s.Option("name")
		case snmpRWSection:
			p.CommunityRW = s.Option("name")
		}
	}
	return p
}

// snmpdSections reads /etc/config/snmpd via uci show. Returns nil when the
// config does not exist at all. Keys are section names: named sections by
// their name, anonymous ones by their cfg id (cfgXXXX...).
func snmpdSections() map[string]uciSection {
	out, err := exec.Command("uci", "-q", "show", "snmpd").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	return parseUCIShow(string(out), "snmpd")
}

// snmpdConfigManaged is true when every section type is one the panel
// manages. Anything else means someone else owns the config.
func snmpdConfigManaged(sections map[string]uciSection) bool {
	for _, s := range sections {
		if !snmpdManagedSections[s.Type] {
			return false
		}
	}
	return true
}

// stockCommunities lists the community sections that did not come from the
// panel (the stock 'public' one and any anonymous leftovers). The panel
// deletes them on apply so a managed config has exactly the communities the
// user asked for.
func stockCommunities(sections map[string]uciSection) []string {
	var out []string
	for name, s := range sections {
		if s.Type != "community" {
			continue
		}
		if name != snmpROSection && name != snmpRWSection {
			out = append(out, name)
		}
	}
	return out
}

// SetSNMP applies the request. The package must already be installed (the
// optional-packages catalog handles that). The snmpd config is snapshotted
// and restored on failure; the daemon restarts only after the commit lands,
// and a daemon that fails to come up with the new config rolls back.
func SetSNMP(req SNMPRequest) (*SNMPProbe, bool, error) {
	if !pkgInstalled("snmpd") {
		return ProbeSNMP(), false, fmt.Errorf("snmpd is not installed")
	}
	sections := snmpdSections()
	if sections != nil && !snmpdConfigManaged(sections) {
		return ProbeSNMP(), false, fmt.Errorf("snmpd configuration is managed outside NetGrip; refusing to modify it")
	}
	if req.Enabled && strings.TrimSpace(req.CommunityRO) == "" {
		return ProbeSNMP(), false, fmt.Errorf("a read-only community is required to enable SNMP")
	}

	// A router that never had a snmpd config has nothing to snapshot; on
	// rollback an empty import restores exactly that (no package).
	snap := ""
	if sections != nil {
		var err error
		snap, err = executor.Snapshot("snmpd")
		if err != nil {
			return nil, false, fmt.Errorf("snapshot snmpd: %w", err)
		}
	}
	restore := func() {
		_ = executor.Restore("snmpd", snap)
	}

	ops := []executor.Op{
		// uci set cannot create a missing section: declare it with its type
		// first, idempotent when it already exists.
		{Kind: "uci_set", Args: []string{"snmpd.agent", "agent"}},
		{Kind: "uci_set", Args: []string{"snmpd.agent.sysLocation", req.Location}},
		{Kind: "uci_set", Args: []string{"snmpd.agent.sysContact", req.Contact}},
	}
	listen := strings.TrimSpace(req.Listen)
	if listen == "" {
		listen = "UDP:161"
	}
	ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"snmpd.agent.agentaddress", listen}})

	for _, stock := range stockCommunities(sections) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"snmpd." + stock}})
	}

	ops = append(ops,
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpROSection, "community"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpROSection + ".name", req.CommunityRO}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpROSection + ".source", "default"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpROSection + ".mode", "ro"}},
	)
	if strings.TrimSpace(req.CommunityRW) == "" {
		// Delete is not idempotent in uci, but the executor treats "Entry
		// not found" as success, so a missing section is fine.
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"snmpd." + snmpRWSection}})
	} else {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpRWSection, "community"}},
			executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpRWSection + ".name", req.CommunityRW}},
			executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpRWSection + ".source", "default"}},
			executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpRWSection + ".mode", "rw"}},
		)
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"snmpd"}})

	if err := executor.Apply(ops, nil); err != nil {
		restore()
		return ProbeSNMP(), true, err
	}

	svcOp := "stop"
	if req.Enabled {
		svcOp = "restart"
	}
	if err := executor.Run(executor.Op{Kind: "initd", Args: []string{"snmpd", svcOp}}); err != nil {
		restore()
		return ProbeSNMP(), true, err
	}
	if req.Enabled {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"snmpd", "enable"}})
		if !executor.ServiceRunning("snmpd") {
			restore()
			return ProbeSNMP(), true, fmt.Errorf("snmpd did not come up after restart; configuration rolled back")
		}
	} else {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"snmpd", "disable"}})
	}
	return ProbeSNMP(), false, nil
}
