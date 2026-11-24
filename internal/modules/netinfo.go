package modules

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/ubus"
)

// IfaceCounters are the raw /proc/net/dev counters of one interface.
type IfaceCounters struct {
	Name    string `json:"name"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}

// isWirelessNetdev reports whether a netdev belongs to a wireless phy.
func isWirelessNetdev(name string) bool {
	_, err := os.Stat("/sys/class/net/" + name + "/phy80211")
	return err == nil
}

// isPhysicalEthPort reports whether a netdev is a real ethernet port, asking
// the kernel instead of matching its name: port names are board-specific
// (lan1, wan2, sfp1, port3, enp2s0 on x86), so a fixed name pattern
// silently hides every port on a router that names them differently. A
// port qualifies when it is ARPHRD_ETHER (type 1), is not a virtual device
// (bridges, VLANs and tunnels live under /devices/virtual) and is not a
// wireless netdev.
func isPhysicalEthPort(name string) bool {
	data, err := os.ReadFile("/sys/class/net/" + name + "/type")
	if err != nil || strings.TrimSpace(string(data)) != "1" {
		return false
	}
	target, err := os.Readlink("/sys/class/net/" + name)
	if err != nil || strings.Contains(target, "/virtual/") {
		return false
	}
	return !isWirelessNetdev(name)
}

// NetDevCounters parses /proc/net/dev for the interfaces worth reporting:
// the physical ports, the LAN bridge and the wireless netdevs.
func NetDevCounters() []IfaceCounters {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return []IfaceCounters{}
	}
	bridge := LANBridge()
	var counters []IfaceCounters
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name != bridge && !isPhysicalEthPort(name) && !isWirelessNetdev(name) {
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

// EthDevice is one device learned on a port, with its name when it can
// be resolved (dnsmasq leases when the router runs DHCP).
type EthDevice struct {
	MAC  string `json:"mac"`
	Name string `json:"name,omitempty"`
}

// EthPort is one physical ethernet port with its link state and the
// devices the switch has learned on it.
type EthPort struct {
	Name      string      `json:"name"`
	Wan       bool        `json:"wan"`
	Up        bool        `json:"up"`
	SpeedMbps int         `json:"speed_mbps"`
	Devices   []EthDevice `json:"devices"`
}

// wanPortBase resolves the physical port name that carries the active WAN
// uplink, so EthPorts can mark it dynamically instead of assuming the port
// is named "wan" (routers with more than one port, or where the physical
// WAN port has a different name - e.g. a PPPoE uplink wired to "lan1" while
// a cellular failover interface is named "wan" with no Ethernet port at
// all - both of which occur in the wild). Strips a VLAN suffix (e.g.
// "lan1.7") to match the base port name; returns "" when the active uplink
// is not on any known physical port (a modem, a bridge, or nothing found).
func wanPortBase() string {
	device := ubus.ActiveWANDevice()
	if device == "" {
		return ""
	}
	if i := strings.LastIndexByte(device, '.'); i > 0 {
		if base := device[:i]; isPhysicalEthPort(base) {
			return base
		}
	}
	return device
}

// wanPortName is wanPortBase with the conventional fallback, for callers
// that need a port to act on even when no WAN interface is currently up -
// an AP-mode router has none, and that is exactly when the mode switch
// needs to name the port.
func wanPortName() string {
	if p := wanPortBase(); p != "" {
		return p
	}
	return "wan"
}

// linkMasters returns the netdevs that are the master of a physical port:
// on DSA switches the user ports show up as lanX@<master>, where <master>
// is the SoC uplink rather than a user port, so EthPorts skips it.
//
// The master is only taken from an '@' link whose left side is itself a
// physical port. A VLAN riding on a user port (wan.20@wan on a PPPoE
// gateway) names that port too, and treating it as an uplink removed the
// WAN port from the chassis entirely (#609).
func linkMasters(ipLink string, physical func(string) bool) map[string]bool {
	masters := map[string]bool{}
	for _, line := range strings.Split(ipLink, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		n := strings.TrimSuffix(fields[1], ":")
		i := strings.LastIndexByte(n, '@')
		if i < 0 {
			continue
		}
		if physical(n[:i]) {
			masters[n[i+1:]] = true
		}
	}
	return masters
}

// EthPorts lists the physical ports with link state, speed and the
// devices learned on each (names resolved from DHCP leases when present).
// The link that only exists to carry a DSA port to the SoC is excluded
// (see linkMasters).
func EthPorts() []EthPort {
	out, err := exec.Command("ip", "-o", "link").Output()
	if err != nil {
		return []EthPort{}
	}
	fdb := bridgeFdb()
	names := leaseNames()
	wanBase := wanPortName()

	masters := linkMasters(string(out), isPhysicalEthPort)

	var ports []EthPort
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		full := strings.TrimSuffix(fields[1], ":")
		base := full
		if i := strings.LastIndexByte(full, '@'); i >= 0 {
			base = full[:i]
		}
		if !isPhysicalEthPort(base) {
			continue
		}
		if masters[base] {
			continue // switch uplink, not a user port
		}
		port := EthPort{Name: base, Devices: []EthDevice{}}
		port.Wan = base == wanBase && WanPortActive()
		port.Up = strings.Contains(line, "LOWER_UP")
		if speed, err := os.ReadFile("/sys/class/net/" + base + "/speed"); err == nil {
			if mbps, err := strconv.Atoi(strings.TrimSpace(string(speed))); err == nil && mbps > 0 {
				port.SpeedMbps = mbps
			}
		}
		if macs, ok := fdb[base]; ok {
			for _, mac := range macs {
				port.Devices = append(port.Devices, EthDevice{MAC: mac, Name: names[mac]})
			}
		}
		ports = append(ports, port)
	}
	if ports == nil {
		return []EthPort{}
	}
	return ports
}

// leaseNames maps MAC -> hostname from the dnsmasq leases file (empty on
// routers without dnsmasq, like dumb APs).
func leaseNames() map[string]string {
	leases, err := ubusLeases()
	if err != nil {
		return map[string]string{}
	}
	names := map[string]string{}
	for _, l := range leases {
		if l.Hostname != "" && l.Hostname != "*" {
			names[strings.ToLower(l.MAC)] = l.Hostname
		}
	}
	return names
}

func ubusLeases() ([]ubus.Lease, error) {
	return ubus.ReadLeases("/tmp/dhcp.leases")
}

// bridgeFdb returns the learned MACs grouped by port name, via
// `brctl showmacs` against the configured LAN bridge, falling back to the
// conventional names (br-lan, then br0 as used by GLuON) when the config
// resolves to something that isn't up. The port numbers in showmacs are
// the low byte of the kernel
// port_id, resolved via /sys/class/net/<bridge>/brif/<dev>/port_id
// (verified on an ipq807x DSA switch: brctl show order is NOT the port
// order).
func bridgeFdb() map[string][]string {
	for _, bridge := range []string{LANBridge(), "br-lan", "br0"} {
		if bridge == "" {
			continue
		}
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
	return parseBrctlShowmacs(string(out), portNames)
}

// parseBrctlShowmacs groups the learned MACs by port name. Under VLAN
// filtering the bridge keeps one FDB entry per (port, VLAN, MAC) and brctl
// prints no VLAN column, so a trunked device shows up once per VLAN it
// carries - a switch on seven VLANs appeared seven times in the client
// list. Each port keeps one entry per MAC; a MAC learned on two different
// ports is still reported on both, which is what loop detection looks for.
func parseBrctlShowmacs(out string, portNames map[int]string) map[string][]string {
	fdb := map[string][]string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] == "yes" {
			continue
		}
		portNo, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		name, ok := portNames[portNo]
		if !ok {
			continue
		}
		key := name + "\x00" + fields[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		fdb[name] = append(fdb[name], fields[1])
	}
	return fdb
}
