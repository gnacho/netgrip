package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type PortTemplateVLAN struct {
	VID    int  `json:"vid"`
	Tagged bool `json:"tagged"`
}

type PortTemplate struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	VLANs       []PortTemplateVLAN `json:"vlans"`
	AdminUp     bool               `json:"admin_up"`
	SpeedMbps   int                `json:"speed_mbps"`
}

func ListPortTemplates() []PortTemplate {
	out, err := exec.Command("uci", "show", "netgrip").Output()
	if err != nil {
		return []PortTemplate{}
	}
	return listPortTemplatesFrom(string(out))
}

// listPortTemplatesFrom parses `uci show netgrip` output for port_template
// sections ("netgrip.<section>=port_template"). Never returns nil.
func listPortTemplatesFrom(out string) []PortTemplate {
	templates := map[string]*PortTemplate{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "netgrip.") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := strings.Trim(parts[1], "'")

		rel := strings.TrimPrefix(key, "netgrip.")
		if !strings.Contains(rel, ".") {
			// section header: netgrip.<section>=port_template
			if val == "port_template" {
				templates[rel] = &PortTemplate{Name: rel}
			}
			continue
		}
		sec, opt, _ := strings.Cut(rel, ".")
		t, ok := templates[sec]
		if !ok {
			continue
		}
		switch opt {
		case "description":
			t.Description = val
		case "admin_up":
			t.AdminUp = val == "1"
		case "speed_mbps":
			if v, err := parseInt(val); err == nil {
				t.SpeedMbps = v
			}
		case "vlans_json":
			json.Unmarshal([]byte(val), &t.VLANs)
		}
	}
	result := make([]PortTemplate, 0, len(templates))
	for _, t := range templates {
		result = append(result, *t)
	}
	return result
}

func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

type PortTemplateSave struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	VLANs       []PortTemplateVLAN `json:"vlans"`
	AdminUp     bool               `json:"admin_up"`
	SpeedMbps   int                `json:"speed_mbps"`
}

func SavePortTemplate(tpl PortTemplateSave) error {
	if tpl.Name == "" {
		return fmt.Errorf("name required")
	}
	secName := sanitizeUCIKey(tpl.Name)
	if !uciSectionExists("netgrip.port_templates") {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config port_templates 'port_templates'\n")
		_ = cmd.Run()
	}
	if !uciSectionExists("netgrip." + secName) {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config port_template '" + secName + "'\n")
		_ = cmd.Run()
	}

	vlansJSON, _ := json.Marshal(tpl.VLANs)
	adminUp := "0"
	if tpl.AdminUp {
		adminUp = "1"
	}

	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"netgrip." + secName + ".description", tpl.Description}},
		{Kind: "uci_set", Args: []string{"netgrip." + secName + ".admin_up", adminUp}},
		{Kind: "uci_set", Args: []string{"netgrip." + secName + ".speed_mbps", fmt.Sprintf("%d", tpl.SpeedMbps)}},
		{Kind: "uci_set", Args: []string{"netgrip." + secName + ".vlans_json", string(vlansJSON)}},
		{Kind: "uci_commit", Args: []string{"netgrip"}},
	}
	return executor.Apply(ops, nil)
}

func DeletePortTemplate(name string) error {
	secName := sanitizeUCIKey(name)
	if !uciSectionExists("netgrip." + secName) {
		return fmt.Errorf("template not found: %s", name)
	}
	ops := []executor.Op{
		{Kind: "uci_delete", Args: []string{"netgrip." + secName}},
		{Kind: "uci_commit", Args: []string{"netgrip"}},
	}
	return executor.Apply(ops, nil)
}

type PortTemplateApply struct {
	Template string   `json:"template"`
	Ports    []string `json:"ports"`
}

func ApplyPortTemplate(req PortTemplateApply) error {
	templates := ListPortTemplates()
	var tpl *PortTemplate
	for _, t := range templates {
		if sanitizeUCIKey(t.Name) == sanitizeUCIKey(req.Template) {
			tpl = &t
			break
		}
	}
	if tpl == nil {
		return fmt.Errorf("template not found: %s", req.Template)
	}

	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return err
	}

	for _, port := range req.Ports {
		for _, vlan := range tpl.VLANs {
			vidStr := fmt.Sprintf("%d", vlan.VID)
			portVal := port
			if vlan.Tagged {
				portVal = port + ":t"
			}
			// Find the bridge-vlan section for this VID
			section := findVLANSection(vlan.VID)
			if section != "" {
				executor.Run(executor.Op{Kind: "uci_add_list", Args: []string{"network." + section + ".ports", portVal}})
			} else {
				// Create new bridge-vlan section
				cmd := exec.Command("uci", "add", "network", "bridge-vlan")
				out, err := cmd.Output()
				if err == nil {
					newSec := strings.TrimSpace(string(out))
					executor.Apply([]executor.Op{
						{Kind: "uci_set", Args: []string{"network." + newSec + ".device", "br-lan"}},
						{Kind: "uci_set", Args: []string{"network." + newSec + ".vlan", vidStr}},
						{Kind: "uci_add_list", Args: []string{"network." + newSec + ".ports", portVal}},
					}, nil)
				}
			}
		}

		if tpl.SpeedMbps > 0 {
			SetSwitchPort(SwitchPortEdit{Name: port, SpeedMbps: &tpl.SpeedMbps})
		}
		if tpl.AdminUp {
			executor.Run(executor.Op{Kind: "ip_link", Args: []string{port, "up"}})
		}
	}

	executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"network"}})
	if err := executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}}); err != nil {
		_ = executor.Restore("network", snapNetwork)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return err
	}
	return nil
}
