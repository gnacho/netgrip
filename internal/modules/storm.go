package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const stormConfPath = "/etc/netgrip/storm.conf"

type StormPort struct {
	Port             string `json:"port"`
	LinkSpeedMbps    int    `json:"link_speed_mbps"`
	BroadcastKbps    int    `json:"broadcast_kbps"`
	MulticastKbps    int    `json:"multicast_kbps"`
	UnknownUnicastKbps int  `json:"unknown_unicast_kbps"`
	Active           bool   `json:"active"`
}

type StormProbe struct {
	Applicable bool        `json:"applicable"`
	Ports      []StormPort `json:"ports"`
}

type StormSetRequest struct {
	Port    string `json:"port"`
	Percent int    `json:"percent"`
}

func ProbeStormControl() StormProbe {
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return StormProbe{Applicable: false}
	}

	var ports []StormPort
	for port := range portMap {
		sp := readStormPort(port)
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
	if req.Port == "" || req.Percent < 0 || req.Percent > 100 {
		return fmt.Errorf("invalid port or percent")
	}

	portMap := bridgePorts()
	if _, ok := portMap[req.Port]; !ok {
		return fmt.Errorf("port not in bridge")
	}

	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", req.Port)
	speedMbps := 1000
	if data, err := os.ReadFile(speedPath); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v > 0 {
			speedMbps = v
		}
	}

	rateKbps := speedMbps * 1000 * req.Percent / 100

	exec.Command("tc", "qdisc", "del", "dev", req.Port, "ingress").Run()

	if req.Percent == 0 {
		removeStormConfig(req.Port)
		return nil
	}

	if err := exec.Command("tc", "qdisc", "add", "dev", req.Port, "ingress").Run(); err != nil {
		return fmt.Errorf("add ingress qdisc: %v", err)
	}

	bcRate := fmt.Sprintf("%dkbit", rateKbps)

	exec.Command("tc", "filter", "add", "dev", req.Port, "parent", "ffff:",
		"protocol", "all", "prio", "1",
		"basic", "match", "meta(dst eq ff:ff:ff:ff:ff:ff)",
		"police", "rate", bcRate, "burst", "32k",
		"exceed", "drop").Run()

	exec.Command("tc", "filter", "add", "dev", req.Port, "parent", "ffff:",
		"protocol", "all", "prio", "2",
		"basic", "match", "meta(dst eq 01:00:5e:00:00:00/01:00:00:00:00:00)",
		"police", "rate", bcRate, "burst", "32k",
		"exceed", "drop").Run()

	saveStormConfig(req.Port, req.Percent)

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
