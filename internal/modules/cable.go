package modules

import (
	"os/exec"
	"strings"
)

type CableTestResult struct {
	Port       string `json:"port"`
	Supported  bool   `json:"supported"`
	PairStatus string `json:"pair_status,omitempty"`
	Length     string `json:"length,omitempty"`
	Error      string `json:"error,omitempty"`
}

type CableTestProbe struct {
	Applicable  bool              `json:"applicable"`
	MissingTool string            `json:"missing_tool,omitempty"`
	Ports       []CableTestResult `json:"ports"`
}

func ProbeCableTest() CableTestProbe {
	if _, err := exec.LookPath("ethtool"); err != nil {
		return CableTestProbe{Applicable: false, MissingTool: "ethtool-full"}
	}
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return CableTestProbe{Applicable: false}
	}

	// The cable test needs the full ethtool build with netlink support; the
	// tiny variant prints a netlink error even though the binary exists.
	for port := range portMap {
		if strings.HasPrefix(port, "phy") || strings.HasPrefix(port, "wlan") {
			continue
		}
		out, _ := exec.Command("ethtool", "--cable-test", port).CombinedOutput()
		if strings.Contains(strings.ToLower(string(out)), "netlink support") {
			return CableTestProbe{Applicable: false, MissingTool: "ethtool-full"}
		}
		break
	}

	var results []CableTestResult
	for port := range portMap {
		result := testCable(port)
		results = append(results, result)
	}

	return CableTestProbe{
		Applicable: true,
		Ports:      results,
	}
}

func testCable(port string) CableTestResult {
	result := CableTestResult{Port: port}

	cmd := exec.Command("ethtool", "--cable-test", port)
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		if strings.Contains(output, "not supported") ||
			strings.Contains(output, "Not supported") ||
			strings.Contains(output, "operation not supported") {
			result.Supported = false
			result.Error = "not supported"
			return result
		}
		result.Supported = false
		result.Error = strings.TrimSpace(output)
		if result.Error == "" {
			result.Error = err.Error()
		}
		return result
	}

	result.Supported = true
	result.PairStatus = parseCableStatus(output)
	result.Length = parseCableLength(output)

	return result
}

func parseCableStatus(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Pair") || strings.Contains(line, "pair") {
			if strings.Contains(line, "OK") || strings.Contains(line, "ok") {
				return "ok"
			}
			if strings.Contains(line, "Open") || strings.Contains(line, "open") {
				return "open"
			}
			if strings.Contains(line, "Short") || strings.Contains(line, "short") {
				return "short"
			}
			return line
		}
	}
	return "unknown"
}

func parseCableLength(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Length") || strings.Contains(line, "length") ||
			strings.Contains(line, "Distance") || strings.Contains(line, "distance") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if i := strings.Index(val, ","); i >= 0 {
					val = strings.TrimSpace(val[:i])
				}
				return val
			}
		}
	}
	return ""
}
