package modules

import (
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type SwitchPort struct {
	Name        string `json:"name"`
	AdminUp     bool   `json:"admin_up"`
	OperUp      bool   `json:"oper_up"`
	SpeedMbps   int    `json:"speed_mbps"`
	Duplex      string `json:"duplex"`
	PoeEnabled  bool   `json:"poe_enabled"`
	PoeSupported bool  `json:"poe_supported"`
	Description string `json:"description"`
}

type SwitchProbe struct {
	Applicable bool         `json:"applicable"`
	Ports      []SwitchPort `json:"ports"`
}

func ProbeSwitchPorts() *SwitchProbe {
	ports := bridgePortList()
	if len(ports) == 0 {
		return &SwitchProbe{Applicable: false}
	}

	var result []SwitchPort
	for _, name := range ports {
		p := SwitchPort{
			Name: name,
			Description: uciGet("netgrip.ports." + sanitizeUCIKey(name) + ".description"),
		}

		if data, err := os.ReadFile("/sys/class/net/" + name + "/operstate"); err == nil {
			state := strings.TrimSpace(string(data))
			p.OperUp = state == "up"
		}

		if data, err := os.ReadFile("/sys/class/net/" + name + "/carrier"); err == nil {
			p.AdminUp = strings.TrimSpace(string(data)) == "1"
		}

		if data, err := os.ReadFile("/sys/class/net/" + name + "/speed"); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v > 0 {
				p.SpeedMbps = v
			}
		}

		if data, err := os.ReadFile("/sys/class/net/" + name + "/duplex"); err == nil {
			p.Duplex = strings.TrimSpace(string(data))
		}

		poePath := "/sys/class/net/" + name + "/device/of_node/poe"
		if _, err := os.Stat(poePath); err == nil {
			p.PoeSupported = true
			if data, err := os.ReadFile(poePath + "/status"); err == nil {
				p.PoeEnabled = strings.TrimSpace(string(data)) != "disabled"
			}
		}

		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return &SwitchProbe{Applicable: true, Ports: result}
}

func sanitizeUCIKey(s string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(s)
}

type SwitchPortEdit struct {
	Name        string  `json:"name"`
	AdminUp     *bool   `json:"admin_up,omitempty"`
	SpeedMbps   *int    `json:"speed_mbps,omitempty"`
	PoeEnabled  *bool   `json:"poe_enabled,omitempty"`
	Description *string `json:"description,omitempty"`
}

func SetSwitchPort(edit SwitchPortEdit) (*SwitchProbe, bool, error) {
	if edit.Name == "" {
		return nil, false, nil
	}
	ports := bridgePortList()
	found := false
	for _, p := range ports {
		if p == edit.Name {
			found = true
			break
		}
	}
	if !found {
		return ProbeSwitchPorts(), false, nil
	}

	if edit.AdminUp != nil {
		action := "down"
		if *edit.AdminUp {
			action = "up"
		}
		if err := executor.Run(executor.Op{Kind: "ip_link", Args: []string{edit.Name, action}}); err != nil {
			return ProbeSwitchPorts(), false, err
		}
	}

	if edit.SpeedMbps != nil {
		speed := strconv.Itoa(*edit.SpeedMbps)
		duplex := "full"
		cmd := exec.Command("ethtool", "-s", edit.Name, "speed", speed, "duplex", duplex, "autoneg", "off")
		_ = cmd.Run()
	}

	if edit.PoeEnabled != nil {
		val := "disable"
		if *edit.PoeEnabled {
			val = "enable"
		}
		poePath := "/sys/class/net/" + edit.Name + "/device/of_node/poe/status"
		os.WriteFile(poePath, []byte(val), 0644)
	}

	if edit.Description != nil {
		key := "netgrip.ports." + sanitizeUCIKey(edit.Name) + ".description"
		if !uciSectionExists("netgrip.ports") {
			cmd := exec.Command("uci", "import", "netgrip")
			cmd.Stdin = strings.NewReader("config ports 'ports'\n")
			_ = cmd.Run()
		}
		secName := sanitizeUCIKey(edit.Name)
		if !uciSectionExists("netgrip.ports." + secName) {
			cmd := exec.Command("uci", "import", "netgrip")
			cmd.Stdin = strings.NewReader("config port '" + secName + "'\n")
			_ = cmd.Run()
		}
		executor.Apply([]executor.Op{
			{Kind: "uci_set", Args: []string{key, *edit.Description}},
			{Kind: "uci_commit", Args: []string{"netgrip"}},
		}, nil)
	}

	return ProbeSwitchPorts(), false, nil
}
