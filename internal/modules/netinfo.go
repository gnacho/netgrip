package modules

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// IfaceCounters are the raw /proc/net/dev counters of one interface.
type IfaceCounters struct {
	Name    string `json:"name"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}

var reNetDevIface = regexp.MustCompile(`^(br-lan|br0|lan\d+|wan|eth\d+|swp\d+|phy\d+-ap\d+|wlan\d+(-\d+)?)$`)

// NetDevCounters parses /proc/net/dev for the interesting interfaces.
func NetDevCounters() []IfaceCounters {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return []IfaceCounters{}
	}
	var counters []IfaceCounters
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if !reNetDevIface.MatchString(name) {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseInt(fields[0], 10, 64)
		tx, _ := strconv.ParseInt(fields[8], 10, 64)
		counters = append(counters, IfaceCounters{Name: name, RxBytes: rx, TxBytes: tx})
	}
	if counters == nil {
		return []IfaceCounters{}
	}
	return counters
}

// EthPort is one physical ethernet port with its link state and the
// MAC addresses the switch has learned on it.
type EthPort struct {
	Name      string   `json:"name"`
	Up        bool     `json:"up"`
	SpeedMbps int      `json:"speed_mbps"`
	MACs      []string `json:"macs"`
}

var reEthPortName = regexp.MustCompile(`^(lan\d+|wan|eth\d+|swp\d+)$`)

// EthPorts lists the physical ports with link state, speed and FDB MACs.
func EthPorts() []EthPort {
	out, err := exec.Command("ip", "-o", "link").Output()
	if err != nil {
		return []EthPort{}
	}
	fdb := bridgeFdb()
	var ports []EthPort
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := strings.TrimSuffix(fields[1], ":")
		if !reEthPortName.MatchString(name) {
			continue
		}
		port := EthPort{Name: name, MACs: []string{}}
		port.Up = strings.Contains(line, "LOWER_UP")
		if speed, err := os.ReadFile("/sys/class/net/" + name + "/speed"); err == nil {
			if mbps, err := strconv.Atoi(strings.TrimSpace(string(speed))); err == nil && mbps > 0 {
				port.SpeedMbps = mbps
			}
		}
		if macs, ok := fdb[name]; ok {
			port.MACs = macs
		}
		ports = append(ports, port)
	}
	if ports == nil {
		return []EthPort{}
	}
	return ports
}

// bridgeFdb returns the learned MACs grouped by port name, via
// `brctl showmacs` against br-lan first and br0 as fallback (GLuON uses
// br0). The port numbers in showmacs are the low byte of the kernel
// port_id, resolved via /sys/class/net/<bridge>/brif/<dev>/port_id
// (verified on an ipq807x DSA switch: brctl show order is NOT the port
// order).
func bridgeFdb() map[string][]string {
	for _, bridge := range []string{"br-lan", "br0"} {
		if fdb := fdbFromBrctl(bridge); fdb != nil {
			return fdb
		}
	}
	return map[string][]string{}
}

func fdbFromBrctl(bridge string) map[string][]string {
	// port number (low byte of port_id) -> interface name
	brifDir := "/sys/class/net/" + bridge + "/brif"
	entries, err := os.ReadDir(brifDir)
	if err != nil {
		return nil
	}
	portNames := map[int]string{}
	for _, e := range entries {
		data, err := os.ReadFile(brifDir + "/" + e.Name() + "/port_id")
		if err != nil {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(string(data)), 0, 64)
		if err != nil {
			continue
		}
		portNames[int(id&0xff)] = e.Name()
	}

	out, err := exec.Command("brctl", "showmacs", bridge).Output()
	if err != nil {
		return nil
	}
	fdb := map[string][]string{}
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] == "yes" {
			continue
		}
		portNo, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		if name, ok := portNames[portNo]; ok {
			fdb[name] = append(fdb[name], fields[1])
		}
	}
	return fdb
}
