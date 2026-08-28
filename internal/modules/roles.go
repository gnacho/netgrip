package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type RoleProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	VID         int    `json:"vid"`
	Isolated    bool   `json:"isolated"`
}

func ListRoleProfiles() []RoleProfile {
	return []RoleProfile{
		{ID: "trusted", Name: "Trusted", Description: "Full LAN access (VLAN 1)", VID: 1, Isolated: false},
		{ID: "iot", Name: "IoT", Description: "Isolated IoT network (VLAN 10)", VID: 10, Isolated: true},
		{ID: "guest", Name: "Guest", Description: "Internet-only guest (VLAN 20)", VID: 20, Isolated: true},
		{ID: "camera", Name: "Camera", Description: "Camera surveillance (VLAN 30)", VID: 30, Isolated: true},
	}
}

type RoleApply struct {
	RoleID string `json:"role_id"`
	Port   string `json:"port"`
}

func ApplyRoleProfile(req RoleApply) error {
	if req.RoleID == "" || req.Port == "" {
		return fmt.Errorf("role_id and port required")
	}

	var role *RoleProfile
	roles := ListRoleProfiles()
	for _, r := range roles {
		if r.ID == req.RoleID {
			r2 := r
			role = &r2
			break
		}
	}
	if role == nil {
		return fmt.Errorf("unknown role: %s", req.RoleID)
	}

	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return err
	}
	snapFirewall, _ := executor.Snapshot("firewall")

	// Ensure the VLAN exists
	section := findVLANSection(role.VID)
	if section == "" {
		cmd := fmt.Sprintf("config bridge-vlan\n\toption device 'br-lan'\n\toption vlan '%d'\n", role.VID)
		importCmd := fmt.Sprintf("uci import network <<'EOF'\n%sEOF\n", cmd)
		_ = executor.Run(executor.Op{Kind: "uci_set", Args: []string{"network.bridge_vlan_" + fmt.Sprintf("%d", role.VID), "bridge-vlan"}})
		_ = importCmd
		// Simpler approach: add via uci commands
		addVLANSection(role.VID)
	}

	// Add port to the VLAN as untagged
	vidStr := fmt.Sprintf("%d", role.VID)
	sec := findVLANSection(role.VID)
	if sec != "" {
		// Remove port from other VLANs first
		removePortFromAllVLANs(req.Port)
		executor.Run(executor.Op{Kind: "uci_add_list", Args: []string{"network." + sec + ".ports", req.Port}})
	}

	ops := []executor.Op{
		{Kind: "uci_commit", Args: []string{"network"}},
		{Kind: "initd", Args: []string{"network", "reload"}},
	}

	// If isolated, add firewall rule to block forwarding from this VLAN to LAN
	if role.Isolated && executor.ServiceEnabled("firewall") {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"firewall.netgrip_isolate_" + vidStr, "rule"}},
			executor.Op{Kind: "uci_set", Args: []string{"firewall.netgrip_isolate_" + vidStr + ".name", "NetGrip-Isolate-VLAN" + vidStr}},
			executor.Op{Kind: "uci_set", Args: []string{"firewall.netgrip_isolate_" + vidStr + ".src", "lan"}},
			executor.Op{Kind: "uci_set", Args: []string{"firewall.netgrip_isolate_" + vidStr + ".dest", "lan"}},
			executor.Op{Kind: "uci_set", Args: []string{"firewall.netgrip_isolate_" + vidStr + ".proto", "all"}},
			executor.Op{Kind: "uci_set", Args: []string{"firewall.netgrip_isolate_" + vidStr + ".target", "REJECT"}},
			executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
			executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
		)
	}

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snapNetwork)
		if snapFirewall != "" {
			_ = executor.Restore("firewall", snapFirewall)
		}
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return err
	}
	return nil
}

func addVLANSection(vid int) {
	vidStr := fmt.Sprintf("%d", vid)
	cmd := fmt.Sprintf("config bridge-vlan 'vlan_%s'\n\toption device 'br-lan'\n\toption vlan '%s'\n", vidStr, vidStr)
	importCmd := exec.Command("sh", "-c", "echo '"+cmd+"' | uci import -m network")
	_ = importCmd.Run()
}

func removePortFromAllVLANs(port string) {
	// This is a best-effort removal; the full UCI show + parse approach
	// would be more robust but this covers the common case
	out, err := exec.Command("uci", "show", "network").Output()
	if err != nil {
		return
	}
	for _, line := range splitLines(string(out)) {
		if contains(line, ".ports=") && (contains(line, "="+port+"'") || contains(line, "="+port+":t'")) {
			// Extract section and remove port from list
			parts := splitN(line, "=", 3)
			if len(parts) >= 2 {
				key := parts[0]
				val := trimQuote(parts[len(parts)-1])
				if val == port || val == port+":t" {
					executor.Run(executor.Op{Kind: "uci_del_list", Args: []string{key, val}})
				}
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range split(s, "\n") {
		l = trimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func split(s, sep string) []string {
	return strings.Split(s, sep)
}

func splitN(s, sep string, n int) []string {
	return strings.SplitN(s, sep, n)
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

func trimQuote(s string) string {
	return strings.Trim(s, "'\"")
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
