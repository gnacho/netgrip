package modules

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type PoEPort struct {
	Name        string  `json:"name"`
	Enabled     bool    `json:"enabled"`
	PowerW      float64 `json:"power_w"`
	Class       string  `json:"class"`
	Status      string  `json:"status"`
	ScheduleOn  string  `json:"schedule_on"`
	ScheduleOff string  `json:"schedule_off"`
}

type PoEProbe struct {
	Applicable   bool      `json:"applicable"`
	TotalBudgetW float64   `json:"total_budget_w"`
	UsedW        float64   `json:"used_w"`
	Ports        []PoEPort `json:"ports"`
}

func ProbePoE() *PoEProbe {
	p := &PoEProbe{Ports: []PoEPort{}}

	budgetPath := "/sys/class/hwmon/poe/budget_milliwatts"
	if data, err := os.ReadFile(budgetPath); err == nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
			p.TotalBudgetW = v / 1000
			p.Applicable = true
		}
	}

	if !p.Applicable {
		out, err := exec.Command("poe-util", "status").Output()
		if err == nil {
			parsePoeUtilStatus(p, string(out))
		}
	}

	if !p.Applicable {
		ports := bridgePortList()
		for _, name := range ports {
			poePath := "/sys/class/net/" + name + "/device/of_node/poe"
			if _, err := os.Stat(poePath); err == nil {
				p.Applicable = true
				pp := PoEPort{Name: name}
				if data, err := os.ReadFile(poePath + "/status"); err == nil {
					s := strings.TrimSpace(string(data))
					pp.Status = s
					pp.Enabled = s != "disabled"
				}
				if data, err := os.ReadFile(poePath + "/power_milliwatts"); err == nil {
					if v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
						pp.PowerW = v / 1000
						p.UsedW += pp.PowerW
					}
				}
				pp.ScheduleOn = uciGet("netgrip.poe." + sanitizeUCIKey(name) + ".schedule_on")
				pp.ScheduleOff = uciGet("netgrip.poe." + sanitizeUCIKey(name) + ".schedule_off")
				p.Ports = append(p.Ports, pp)
			}
		}
	}

	return p
}

func parsePoeUtilStatus(p *PoEProbe, output string) {
	p.Applicable = true
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		if strings.Contains(line, "Total Power Budget") {
			for _, f := range parts {
				if v, err := strconv.ParseFloat(f, 64); err == nil && v > 10 {
					p.TotalBudgetW = v
					break
				}
			}
			continue
		}
		pp := PoEPort{Name: parts[0]}
		pp.Status = parts[1]
		pp.Enabled = pp.Status != "Disabled" && pp.Status != "off"
		for _, f := range parts {
			if v, err := strconv.ParseFloat(strings.TrimSuffix(f, "W"), 64); err == nil && v > 0 && v < 100 {
				pp.PowerW = v
				p.UsedW += v
				break
			}
		}
		pp.ScheduleOn = uciGet("netgrip.poe." + sanitizeUCIKey(pp.Name) + ".schedule_on")
		pp.ScheduleOff = uciGet("netgrip.poe." + sanitizeUCIKey(pp.Name) + ".schedule_off")
		p.Ports = append(p.Ports, pp)
	}
}

type PoESchedule struct {
	Port    string `json:"port"`
	OnTime  string `json:"on_time"`
	OffTime string `json:"off_time"`
}

func SetPoESchedule(sched PoESchedule) (*PoEProbe, error) {
	if sched.Port == "" {
		return nil, nil
	}
	key := "netgrip.poe." + sanitizeUCIKey(sched.Port)
	if !uciSectionExists("netgrip.poe") {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config poe 'poe'\n")
		_ = cmd.Run()
	}
	secName := sanitizeUCIKey(sched.Port)
	if !uciSectionExists("netgrip.poe." + secName) {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config poeport '" + secName + "'\n")
		_ = cmd.Run()
	}

	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{key + ".schedule_on", sched.OnTime}},
		{Kind: "uci_set", Args: []string{key + ".schedule_off", sched.OffTime}},
		{Kind: "uci_commit", Args: []string{"netgrip"}},
	}
	_ = executor.Apply(ops, nil)

	applyPoESchedule(sched)

	return ProbePoE(), nil
}

func applyPoESchedule(sched PoESchedule) {
	if sched.OnTime != "" {
		cronLine := sched.OnTime + " /bin/sh -c 'echo enable > /sys/class/net/" + sched.Port + "/device/of_node/poe/status 2>/dev/null || poe-util enable " + sched.Port + " 2>/dev/null'"
		writeCronEntry("netgrip-poe-on-"+sanitizeUCIKey(sched.Port), cronLine)
	}
	if sched.OffTime != "" {
		cronLine := sched.OffTime + " /bin/sh -c 'echo disable > /sys/class/net/" + sched.Port + "/device/of_node/poe/status 2>/dev/null || poe-util disable " + sched.Port + " 2>/dev/null'"
		writeCronEntry("netgrip-poe-off-"+sanitizeUCIKey(sched.Port), cronLine)
	}
	exec.Command("/etc/init.d/cron", "restart").Run()
}

func writeCronEntry(name, line string) {
	dir := "/etc/crontabs"
	os.MkdirAll(dir, 0755)
	path := dir + "/root"
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")
	var filtered []string
	prefix := "# netgrip-" + name
	for _, l := range lines {
		if !strings.HasPrefix(l, prefix) && l != "" {
			filtered = append(filtered, l)
		}
	}
	filtered = append(filtered, prefix)
	filtered = append(filtered, line)
	os.WriteFile(path, []byte(strings.Join(filtered, "\n")+"\n"), 0600)
}
