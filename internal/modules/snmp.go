package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// SNMP (#442): manage the OpenWrt snmpd daemon (net-snmp) from the Advanced
// section. Config lives in UCI (/etc/config/snmpd) and the init script
// regenerates /var/run/snmpd.conf from it on every start, using the classic
// com2sec/group/access/view chain. There is NO 'config community' support in
// the OpenWrt init script (verified on 25.12): communities are com2sec
// sections keyed by their secname.
//
// Ownership rule, same spirit as the AdGuard credentials work: a config with
// section types outside the panel's and the stock sets reports
// Managed=false and is never touched. The untouched stock config (which
// ships 'public'/'private' com2sec chains) is replaced wholesale on the
// first panel apply, so enabling SNMP from the panel never leaves the
// default community open.
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

// panelSectionTypes are the section types a panel-managed config contains
// after an apply: agent + system + the com2sec/group/access/view chain for
// the ro (and optional rw) communities.
var panelSectionTypes = map[string]bool{
	"agent": true, "system": true, "view": true,
	"com2sec": true, "com2sec6": true, "group": true, "access": true,
}

// stockSectionTypes are the section types the OpenWrt package ships with
// beyond the panel's own (agentx, exec, engineid, the 'snmpd' general
// block). An untouched stock config reads as managed so the panel can
// replace it on the first apply.
var stockSectionTypes = map[string]bool{
	"agentx": true, "engineid": true, "exec": true, "snmpd": true,
	// Transitional: early #442 previews wrote 'config community' sections,
	// which the OpenWrt init script ignores. They are dead weight, replaced
	// like the rest of the stock config on apply.
	"community": true,
}

// Section names the panel manages. Named (not anonymous) so re-applying is
// idempotent.
const (
	snmpAgentSection  = "netgrip_agent"
	snmpSystemSection = "netgrip_system"
	snmpViewSection   = "netgrip_all"
	snmpROCom2sec     = "netgrip_ro"
	snmpROCom2sec6    = "netgrip_ro6"
	snmpROGroup       = "netgrip_ro_group"
	snmpROAccess      = "netgrip_ro_access"
	snmpRWCom2sec     = "netgrip_rw"
	snmpRWCom2sec6    = "netgrip_rw6"
	snmpRWGroup       = "netgrip_rw_group"
	snmpRWAccess      = "netgrip_rw_access"
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
	for _, s := range sections {
		switch s.Type {
		case "agent":
			// After a panel apply this is the named netgrip_agent; stock
			// ships an anonymous one. Either way: listen address.
			if p.Listen == "" {
				p.Listen = s.Option("agentaddress")
			}
		case "system":
			// sysLocation/sysContact live here (stock and panel alike).
			p.Location = firstNonEmpty(p.Location, s.Option("sysLocation"))
			p.Contact = firstNonEmpty(p.Contact, s.Option("sysContact"))
		case "com2sec":
			// secname 'ro'/'rw' identifies the chain; the community string
			// is the option. The stock 'public' com2sec uses secname 'ro',
			// so a stock config prefills the ro community with what worked.
			switch s.Option("secname") {
			case "ro":
				p.CommunityRO = firstNonEmpty(p.CommunityRO, s.Option("community"))
			case "rw":
				p.CommunityRW = firstNonEmpty(p.CommunityRW, s.Option("community"))
			}
		}
	}
	return p
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// snmpdSections reads /etc/config/snmpd via uci show. Returns nil when the
// config does not exist at all. Keys are section names: named sections by
// their name, anonymous ones by their cfg id.
func snmpdSections() map[string]uciSection {
	out, err := exec.Command("uci", "-q", "show", "snmpd").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	return parseUCIShow(string(out), "snmpd")
}

// snmpdConfigManaged is true when every section type is the panel's own or
// part of the stock config. Anything else means a human manages this config.
func snmpdConfigManaged(sections map[string]uciSection) bool {
	for _, s := range sections {
		if !panelSectionTypes[s.Type] && !stockSectionTypes[s.Type] {
			return false
		}
	}
	return true
}

// SetSNMP applies the request. The package must already be installed (the
// optional-packages catalog handles that). The snmpd config is snapshotted
// and restored on failure; the daemon restarts only after the commit lands,
// and an agent that does not come up rolls the config back.
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

	// The managed config replaces everything: stock chains (com2sec public/
	// private, agentx, views) go away, so no leftover default community.
	ops := []executor.Op{}
	for name := range sections {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"snmpd." + name}})
	}

	// uci set cannot create a missing section: declare it with its type
	// first, idempotent within the same batch after the deletes above.
	listen := strings.TrimSpace(req.Listen)
	if listen == "" {
		listen = "UDP:161"
	}
	ops = append(ops,
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpAgentSection, "agent"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpAgentSection + ".agentaddress", listen}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpSystemSection, "system"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpSystemSection + ".sysLocation", req.Location}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpSystemSection + ".sysContact", req.Contact}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpViewSection, "view"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpViewSection + ".viewname", "all"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpViewSection + ".type", "included"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + snmpViewSection + ".oid", ".1"}},
	)

	ops = snmpCommunityChainOps(ops, "ro", snmpROCom2sec, snmpROCom2sec6, snmpROGroup, snmpROAccess, req.CommunityRO, "none")
	if strings.TrimSpace(req.CommunityRW) == "" {
		// No rw chain at all: the named sections were already deleted above.
	} else {
		ops = snmpCommunityChainOps(ops, "rw", snmpRWCom2sec, snmpRWCom2sec6, snmpRWGroup, snmpRWAccess, req.CommunityRW, "all")
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

// snmpCommunityChainOps appends the com2sec (+v6)/group/access chain for one
// community. writeAccess is the write view for the access line: "none" for
// ro, "all" for rw.
func snmpCommunityChainOps(ops []executor.Op, secname, com2sec, com2sec6, group, access, community, writeAccess string) []executor.Op {
	groupName := "readonly"
	if secname == "rw" {
		groupName = "readwrite"
	}
	return append(ops,
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec, "com2sec"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec + ".secname", secname}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec + ".source", "default"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec + ".community", community}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec6, "com2sec6"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec6 + ".secname", secname}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec6 + ".source", "default"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + com2sec6 + ".community", community}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + group, "group"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + group + ".group", groupName}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + group + ".version", "v2c"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + group + ".secname", secname}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access, "access"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".group", groupName}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".context", "none"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".version", "any"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".level", "noauth"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".prefix", "exact"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".read", "all"}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".write", writeAccess}},
		executor.Op{Kind: "uci_set", Args: []string{"snmpd." + access + ".notify", "none"}},
	)
}
