package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

	return nil
}
