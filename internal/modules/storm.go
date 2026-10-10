package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const stormConfPath = "/etc/netgrip/storm.conf"

type StormPort struct {
	Port               string `json:"port"`
	LinkSpeedMbps      int    `json:"link_speed_mbps"`
	BroadcastKbps      int    `json:"broadcast_kbps"`
	MulticastKbps      int    `json:"multicast_kbps"`
	UnknownUnicastKbps int    `json:"unknown_unicast_kbps"`
	Active             bool   `json:"active"`
	// Percent es el limite configurado (storm.conf), 0 = sin control. El
	// kbps efectivo se lee de tc, pero no se puede traducir a porcentaje
	// sin velocidad de enlace (una boca caida reporta -1), asi que el
	// probe expone ambos.
	Percent int `json:"percent"`
}

type StormProbe struct {
	Applicable bool        `json:"applicable"`
	Ports      []StormPort `json:"ports"`
}

type StormSetRequest struct {
	// Port aplica a una boca; Ports a una lista; All a todas las del
	// puente. En los tres casos con una sola llamada (#487).
	Port    string   `json:"port"`
	Ports   []string `json:"ports,omitempty"`
	All     bool     `json:"all,omitempty"`
	Percent int      `json:"percent"`
}

func ProbeStormControl() StormProbe {
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return StormProbe{Applicable: false}
	}

	configs := loadStormConfigs()

	var ports []StormPort
	for port := range portMap {
		sp := readStormPort(port)
		sp.Percent = configs[port]
		ports = append(ports, sp)
	}

	return StormProbe{
		Applicable: true,
		Ports:      ports,
	}
}

func readStormPort(port string) StormPort {
	sp := StormPort{Port: port}

	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", port)
	if data, err := os.ReadFile(speedPath); err == nil {
		s := strings.TrimSpace(string(data))
		if v, err := strconv.Atoi(s); err == nil {
			sp.LinkSpeedMbps = v
		}
	}

	out, err := exec.Command("tc", "filter", "show", "dev", port, "ingress").CombinedOutput()
	if err != nil {
		return sp
	}
	output := string(out)

	if strings.Contains(output, "match ff:ff:ff:ff:ff:ff") {
		sp.Active = true
		sp.BroadcastKbps = parseTcRate(output, "broadcast")
	}
	if strings.Contains(output, "match 01:00:5e:00:00:00") ||
		strings.Contains(output, "match 33:33:00:00:00:00") {
		sp.Active = true
		sp.MulticastKbps = parseTcRate(output, "multicast")
	}

	return sp
}

func parseTcRate(output string, kind string) int {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "rate") && i > 0 {
			parts := strings.Fields(line)
			for j, p := range parts {
				if p == "rate" && j+1 < len(parts) {
					val := parts[j+1]
					val = strings.TrimSuffix(val, "Kbit")
					val = strings.TrimSuffix(val, "kbit")
					if v, err := strconv.Atoi(val); err == nil {
						return v
					}
				}
			}
		}
	}
	return 0
}

func SetStormControl(req StormSetRequest) error {
	if req.Percent < 0 || req.Percent > 100 {
		return fmt.Errorf("invalid percent")
	}

	targets, err := stormTargets(req)
	if err != nil {
		return err
	}

	for _, port := range targets {
		if err := setStormPort(port, req.Percent); err != nil {
			return err
		}
	}
	return nil
}

// stormTargets resuelve la lista de bocas destino: todas las del puente
// (All), una lista explicita (Ports) o una sola (Port), validando que
// existan en el puente.
func stormTargets(req StormSetRequest) ([]string, error) {
	portMap := bridgePorts()
	if req.All {
		ports := make([]string, 0, len(portMap))
		for p := range portMap {
			ports = append(ports, p)
		}
		sort.Strings(ports)
		return ports, nil
	}
	if len(req.Ports) > 0 {
		for _, p := range req.Ports {
			if _, ok := portMap[p]; !ok {
				return nil, fmt.Errorf("port %s not in bridge", p)
			}
		}
		return req.Ports, nil
	}
	if req.Port == "" {
		return nil, fmt.Errorf("invalid port")
	}
	if _, ok := portMap[req.Port]; !ok {
		return nil, fmt.Errorf("port not in bridge")
	}
	return []string{req.Port}, nil
}

func setStormPort(port string, percent int) error {
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", port)
	speedMbps := 1000
	if data, err := os.ReadFile(speedPath); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v > 0 {
			speedMbps = v
		}
	}

	rateKbps := speedMbps * 1000 * percent / 100

	exec.Command("tc", "qdisc", "del", "dev", port, "ingress").Run()

	if percent == 0 {
		removeStormConfig(port)
		return nil
	}

	if err := exec.Command("tc", "qdisc", "add", "dev", port, "ingress").Run(); err != nil {
		return fmt.Errorf("add ingress qdisc on %s: %v", port, err)
	}

	bcRate := fmt.Sprintf("%dkbit", rateKbps)

	exec.Command("tc", "filter", "add", "dev", port, "parent", "ffff:",
		"protocol", "all", "prio", "1",
		"basic", "match", "meta(dst eq ff:ff:ff:ff:ff:ff)",
		"police", "rate", bcRate, "burst", "32k",
		"exceed", "drop").Run()

	exec.Command("tc", "filter", "add", "dev", port, "parent", "ffff:",
		"protocol", "all", "prio", "2",
		"basic", "match", "meta(dst eq 01:00:5e:00:00:00/01:00:00:00:00:00)",
		"police", "rate", bcRate, "burst", "32k",
		"exceed", "drop").Run()

	saveStormConfig(port, percent)

	return nil
}

func loadStormConfigs() map[string]int {
	configs := make(map[string]int)
	f, err := os.Open(stormConfPath)
	if err != nil {
		return configs
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			port := strings.TrimSpace(parts[0])
			pct, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil && pct > 0 {
				configs[port] = pct
			}
		}
	}
	return configs
}

func saveStormConfig(port string, percent int) {
	configs := loadStormConfigs()
	configs[port] = percent
	writeStormConfigs(configs)
}

func removeStormConfig(port string) {
	configs := loadStormConfigs()
	delete(configs, port)
	writeStormConfigs(configs)
}

func writeStormConfigs(configs map[string]int) {
	os.MkdirAll(filepath.Dir(stormConfPath), 0755)
	var lines []string
	for port, pct := range configs {
		lines = append(lines, fmt.Sprintf("%s=%d", port, pct))
	}
	os.WriteFile(stormConfPath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}
