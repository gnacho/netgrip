package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const macAclPath = "/etc/netgrip/mac-acl.json"

type MACACLPort struct {
	Port string   `json:"port"`
	Mode string   `json:"mode"`
	MACs []string `json:"macs"`
}

type MACACLProbe struct {
	Applicable bool         `json:"applicable"`
	Ports      []MACACLPort `json:"ports"`
}

type MACACLSetRequest struct {
	Port string   `json:"port"`
	Mode string   `json:"mode"`
	MACs []string `json:"macs"`
}

func ProbeMACACL() MACACLProbe {
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return MACACLProbe{Applicable: false}
	}

	acls := loadMACACLs()

	var ports []MACACLPort
	for port := range portMap {
		acl := findMACACL(acls, port)
		if acl == nil {
			acl = &MACACLPort{Port: port, Mode: "off", MACs: []string{}}
		}
		ports = append(ports, *acl)
	}

	return MACACLProbe{
		Applicable: true,
		Ports:      ports,
	}
}

func SetMACACL(req MACACLSetRequest) error {
	if req.Port == "" {
		return fmt.Errorf("port required")
	}
	if req.Mode != "off" && req.Mode != "allow" && req.Mode != "deny" {
		return fmt.Errorf("mode must be off, allow or deny")
	}

	acls := loadMACACLs()
	acls = removeMACACL(acls, req.Port)
	if req.Mode != "off" && len(req.MACs) > 0 {
		acls = append(acls, MACACLPort{Port: req.Port, Mode: req.Mode, MACs: req.MACs})
	}

	if err := saveMACACLs(acls); err != nil {
		return err
	}

	return applyMACACLRules(acls)
}

func loadMACACLs() []MACACLPort {
	data, err := os.ReadFile(macAclPath)
	if err != nil {
		return nil
	}
	var acls []MACACLPort
	if err := json.Unmarshal(data, &acls); err != nil {
		return nil
	}
	return acls
}

func saveMACACLs(acls []MACACLPort) error {
	if err := os.MkdirAll("/etc/netgrip", 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(acls, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(macAclPath, data, 0600)
}

func findMACACL(acls []MACACLPort, port string) *MACACLPort {
	for i := range acls {
		if acls[i].Port == port {
			return &acls[i]
		}
	}
	return nil
}

func removeMACACL(acls []MACACLPort, port string) []MACACLPort {
	var filtered []MACACLPort
	for _, a := range acls {
		if a.Port != port {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func applyMACACLRules(acls []MACACLPort) error {
	exec.Command("nft", "delete", "table", "bridge", "netgrip_acl").Run()

	var active []MACACLPort
	for _, a := range acls {
		if a.Mode != "off" && len(a.MACs) > 0 {
			active = append(active, a)
		}
	}

	if len(active) == 0 {
		return nil
	}

	var rules strings.Builder
	rules.WriteString("table bridge netgrip_acl {\n")

	for _, a := range active {
		chainName := "acl_" + strings.ReplaceAll(a.Port, ".", "_")
		rules.WriteString(fmt.Sprintf("  chain %s {\n", chainName))

		for _, mac := range a.MACs {
			mac = strings.TrimSpace(strings.ToLower(mac))
			if mac == "" {
				continue
			}
			if a.Mode == "allow" {
				rules.WriteString(fmt.Sprintf("    ether saddr %s accept\n", mac))
			} else {
				rules.WriteString(fmt.Sprintf("    ether saddr %s drop\n", mac))
			}
		}

		if a.Mode == "allow" {
			rules.WriteString("    drop\n")
		}

		rules.WriteString("  }\n")
	}

	rules.WriteString("}\n")

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(rules.String())
	return cmd.Run()
}
