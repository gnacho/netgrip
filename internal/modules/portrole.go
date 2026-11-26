package modules

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// Roles de puerto (#662): decir que una boca ethernet es WAN o LAN sin tocar
// cables, y darle su propia MAC. El reparto de puertos de OpenWrt vive en dos
// sitios (la lista de puertos del bridge y el device de la interfaz WAN), asi
// que la UI no puede mostrar un "rol" guardado en ningun campo: se deduce de
// la configuracion en marcha y se cambia moviendo el puerto de un sitio al
// otro.

type PortRoleEntry struct {
	Name string `json:"name"`
	// Role es derivado: lan cuando el puerto esta en el bridge, wan cuando es
	// el device de la interfaz WAN activa fuera del bridge, unused el resto.
	Role      string `json:"role"`
	LinkUp    bool   `json:"link_up"`
	SpeedMbps int    `json:"speed_mbps"`
	// Mac es la direccion en uso, FactoryMac la del hardware y MacOverride la
	// que escribe la config ("" = fabrica).
	Mac         string `json:"mac"`
	FactoryMac  string `json:"factory_mac"`
	MacOverride string `json:"mac_override"`
	// VlanSuffix es el ".N" que lleva el device WAN actual, para poder
	// conservarlo al mover el puerto (PPPoE sobre VLAN del ISP).
	VlanSuffix string `json:"vlan_suffix,omitempty"`
}

type PortRolesProbe struct {
	Applicable bool   `json:"applicable"`
	Reason     string `json:"reason,omitempty"`
	Mode       string `json:"mode"`
	WanIface   string `json:"wan_iface,omitempty"`
	WanDevice  string `json:"wan_device,omitempty"`
	// CanAssignWAN: en un AP no hay WAN que asignar; la MAC si se puede tocar.
	CanAssignWAN bool            `json:"can_assign_wan"`
	Ports        []PortRoleEntry `json:"ports"`
}

// BasePortName quita el sufijo de VLAN de un device: "eth1.20" -> "eth1".
// Solo quita el ultimo punto cuando lo que sigue son digitos, para no partir
// nombres con puntos que no son VLAN.
func basePortName(device string) string {
	if i := strings.LastIndexByte(device, '.'); i > 0 && i < len(device)-1 {
		if _, err := strconv.Atoi(device[i+1:]); err == nil {
			return device[:i]
		}
	}
	return device
}

func vlanSuffix(device string) string {
	if base := basePortName(device); base != device {
		return device[len(base):]
	}
	return ""
}

// ProbePortRoles lists every physical port with its derived role and MACs.
func ProbePortRoles() *PortRolesProbe {
	mode := ProbeMode()
	p := &PortRolesProbe{
		Mode:         mode.Mode,
		CanAssignWAN: mode.Mode == "router",
		Ports:        []PortRoleEntry{},
	}
	parts := EthPorts()
	if len(parts) == 0 {
		p.Reason = "no physical ethernet ports on this device"
		return p
	}
	// En un switch gestionado los puertos son tela del chip (DSA), no bocas
	// que se puedan repartir entre LAN y WAN: ni rol ni MAC se tocan aquí.
	if mode.Role == "switch" {
		p.Reason = "managed switch: its ports belong to the switch fabric"
		return p
	}
	p.Applicable = true

	sections := networkSections()
	bridge := LANBridge()
	bridgePorts := configuredBridgePorts(sections, bridge)
	for name := range bridgePortsOf(bridge) {
		bridgePorts[name] = true
	}

	p.WanIface = ubus.ActiveWANInterfaceName()
	p.WanDevice = ubus.ActiveWANDevice()
	wanBase := basePortName(p.WanDevice)

	for _, port := range parts {
		inBridge := bridgePorts[port.Name]
		role := derivePortRole(port.Name, inBridge, mode.Mode, wanBase)
		isWAN := role == "wan"
		e := PortRoleEntry{
			Name:      port.Name,
			Role:      role,
			LinkUp:    port.Up,
			SpeedMbps: port.SpeedMbps,
		}
		e.Mac = readStringFile("/sys/class/net/" + port.Name + "/address")
		e.FactoryMac = permanentMAC(port.Name)
		if e.FactoryMac == "" {
			e.FactoryMac = e.Mac
		}
		if sec, ok := deviceSectionByName(sections, port.Name); ok {
			e.MacOverride = sections[sec].Option("macaddr")
		}
		if isWAN {
			e.VlanSuffix = vlanSuffix(p.WanDevice)
		}
		p.Ports = append(p.Ports, e)
	}
	sort.Slice(p.Ports, func(i, j int) bool { return p.Ports[i].Name < p.Ports[j].Name })
	return p
}

// derivePortRole is the rule the whole card rests on, kept pure so it can be
// checked against the real configs of the fleet: a port inside the LAN bridge
// is LAN even if a stale WAN device still names it (router turned AP), and a
// port outside the bridge is WAN only when it backs the active uplink of a
// device that actually routes.
func derivePortRole(port string, inBridge bool, deviceMode, wanBase string) string {
	if deviceMode == "router" && port == wanBase && !inBridge {
		return "wan"
	}
	if inBridge {
		return "lan"
	}
	return "unused"
}

func readStringFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// permanentMAC asks the driver for the hardware address. Not every DSA port
// answers (ethtool -P prints nothing on some switches), so "" means unknown
// and the caller falls back to the address in use.
func permanentMAC(name string) string {
	out, err := exec.Command("ethtool", "-P", name).Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return ""
	}
	if hw, err := net.ParseMAC(fields[2]); err != nil || len(hw) != 6 {
		return ""
	}
	return strings.ToLower(fields[2])
}

func networkSections() map[string]uciSection {
	show, err := uciShowNetwork()
	if err != nil {
		return map[string]uciSection{}
	}
	return parseUCIShow(show, "network")
}

// configuredBridgePorts reads the ports list of the LAN bridge from the
// config, so a probe still knows what the bridge owns when the running one
// is down.
func configuredBridgePorts(sections map[string]uciSection, bridge string) map[string]bool {
	ports := map[string]bool{}
	sec, ok := deviceSectionByName(sections, bridge)
	if !ok {
		return ports
	}
	for _, v := range sections[sec].Options["ports"] {
		ports[v] = true
	}
	return ports
}

func deviceSectionByName(sections map[string]uciSection, name string) (string, bool) {
	for sec, s := range sections {
		if s.Type == "device" && s.Option("name") == name {
			return sec, true
		}
	}
	return "", false
}

// portRoleInput is everything the pure ops builder needs from the device, so
// the decision can be tested without a router.
type portRoleInput struct {
	Bridge    string
	WanIface  string
	WanDevice string
	Mode      string
	// Candidates are the physical ports the chassis shows.
	Candidates []string
}

func (in portRoleInput) candidate(port string) bool {
	for _, c := range in.Candidates {
		if c == port {
			return true
		}
	}
	return basePortName(in.WanDevice) == port
}

// chassisPort is the strict check: only ports the chassis actually lists.
// candidate() also accepts the current WAN device, which is useful to validate
// a target but wrong for deciding whether an old WAN netdev belongs back in
// the bridge (a dedicated 2.5G WAN is not a switch port of the LAN chip).
func (in portRoleInput) chassisPort(port string) bool {
	for _, c := range in.Candidates {
		if c == port {
			return true
		}
	}
	return false
}

// buildPortRoleOps decides the UCI writes for moving a port between LAN and
// WAN. Pure: same inputs, same ops.
func buildPortRoleOps(sections map[string]uciSection, in portRoleInput, port, role string) ([]executor.Op, error) {
	if role != "lan" && role != "wan" {
		return nil, fmt.Errorf("role must be lan or wan")
	}
	if !in.candidate(port) {
		return nil, fmt.Errorf("%s is not a physical ethernet port on this device", port)
	}
	bridgeSec, ok := deviceSectionByName(sections, in.Bridge)
	if !ok {
		return nil, fmt.Errorf("this router has no LAN bridge to move ports in")
	}
	// VLAN filtering moves the port split into bridge-vlan sections; editing
	// the plain bridge ports list there does nothing and the user thinks the
	// change was ignored.
	for _, s := range sections {
		if s.Type != "bridge-vlan" || s.Option("device") != in.Bridge {
			continue
		}
		for _, p := range s.Options["ports"] {
			if strings.SplitN(p, ":", 2)[0] == port {
				return nil, fmt.Errorf("this router splits ports with VLAN filtering: change %s in the VLANs page, where its membership lives", port)
			}
		}
	}
	if user, found := portUserInterface(sections, port, in.WanIface); found && role == "wan" {
		return nil, fmt.Errorf("port %s is already used by interface %s", port, user)
	}

	bridgePorts := configuredBridgePorts(sections, in.Bridge)
	var ops []executor.Op

	switch role {
	case "wan":
		if in.Mode != "router" {
			return nil, fmt.Errorf("this device is not a router: there is no WAN to assign")
		}
		if in.WanIface == "" {
			return nil, fmt.Errorf("no WAN interface is configured")
		}
		if basePortName(in.WanDevice) == port {
			return nil, fmt.Errorf("%s is already the WAN port", port)
		}
		newDevice := port + vlanSuffix(in.WanDevice)
		if bridgePorts[port] {
			ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{"network." + bridgeSec + ".ports", port}})
		}
		if usesIfname(sections, in.WanIface) {
			ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network." + in.WanIface + ".ifname", newDevice}})
		} else {
			ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network." + in.WanIface + ".device", newDevice}})
		}
		// Swap, not accumulation: the port that was the WAN goes back to LAN
		// when it is a switch port of the same bridge. A dedicated WAN netdev
		// (eth1 on a board with a separate 2.5G uplink) is left unused rather
		// than silently bridged.
		oldBase := basePortName(in.WanDevice)
		if oldBase != "" && oldBase != port && in.chassisPort(oldBase) && !bridgePorts[oldBase] && oldBase != in.Bridge {
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + bridgeSec + ".ports", oldBase}})
		}

	case "lan":
		if basePortName(in.WanDevice) == port {
			return nil, fmt.Errorf("%s is the WAN port: the router would be left without an uplink. Assign another port to WAN first, or switch to AP mode in Modo", port)
		}
		if bridgePorts[port] {
			return nil, fmt.Errorf("%s is already a LAN port", port)
		}
		if in.Bridge == "" {
			return nil, fmt.Errorf("this router has no LAN bridge")
		}
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + bridgeSec + ".ports", port}})
	}

	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)
	return ops, nil
}

// usesIfname reports whether the interface still addresses its device with
// the legacy "ifname" option, so the move writes the field netifd reads.
func usesIfname(sections map[string]uciSection, iface string) bool {
	s, ok := sections[iface]
	if !ok {
		return false
	}
	return s.Option("device") == "" && s.Option("ifname") != ""
}

// portUserInterface names another interface that already claims the port, so
// moving it to WAN does not steal a device some other section is using.
func portUserInterface(sections map[string]uciSection, port, wanIface string) (string, bool) {
	for name, s := range sections {
		if s.Type != "interface" || name == wanIface {
			continue
		}
		dev := s.Option("device")
		if dev == "" {
			dev = s.Option("ifname")
		}
		if strings.TrimSpace(dev) == "" {
			continue
		}
		for _, d := range strings.Fields(dev) {
			if basePortName(d) == port {
				return name, true
			}
		}
	}
	return "", false
}

// SetPortRole moves a port between LAN and WAN with a snapshot and rollback,
// the same shape as every other connectivity-risky write.
func SetPortRole(port, role string) (*PortRolesProbe, bool, error) {
	sections := networkSections()
	in := portRoleInput{
		Bridge:     LANBridge(),
		WanIface:   ubus.ActiveWANInterfaceName(),
		WanDevice:  ubus.ActiveWANDevice(),
		Mode:       ProbeMode().Mode,
		Candidates: physicalPortNames(),
	}
	ops, err := buildPortRoleOps(sections, in, port, role)
	if err != nil {
		return ProbePortRoles(), false, err
	}
	snap, err := executor.Snapshot("network")
	if err != nil {
		return ProbePortRoles(), false, fmt.Errorf("snapshot network: %w", err)
	}
	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		InvalidateKey("mode")
		return ProbePortRoles(), true, err
	}
	InvalidateKey("mode")
	invalidateClients()
	return ProbePortRoles(), false, nil
}

func physicalPortNames() []string {
	ports := EthPorts()
	names := make([]string, 0, len(ports))
	for _, p := range ports {
		names = append(names, p.Name)
	}
	return names
}

// SetPortMAC writes or clears the MAC of a single port through its device
// section. The bridge keeps its own address; this only touches the port.
func SetPortMAC(port, mac string) (*PortRolesProbe, bool, error) {
	normalized, err := normalizePortMAC(mac)
	if err != nil {
		return ProbePortRoles(), false, err
	}
	found := false
	for _, c := range physicalPortNames() {
		if c == port {
			found = true
			break
		}
	}
	if !found {
		return ProbePortRoles(), false, fmt.Errorf("%s is not a physical ethernet port on this device", port)
	}

	sections := networkSections()
	var ops []executor.Op
	if sec, ok := deviceSectionByName(sections, port); ok {
		key := "network." + sec + ".macaddr"
		switch {
		case normalized == "" && sections[sec].Option("macaddr") != "":
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{key}})
		case normalized != "" && sections[sec].Option("macaddr") != normalized:
			ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, normalized}})
		}
	} else if normalized != "" {
		sec := namedDeviceSection(port)
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"network." + sec, "device"}},
			executor.Op{Kind: "uci_set", Args: []string{"network." + sec + ".name", port}},
			executor.Op{Kind: "uci_set", Args: []string{"network." + sec + ".macaddr", normalized}},
		)
	}
	if len(ops) == 0 {
		return ProbePortRoles(), false, fmt.Errorf("nothing to change")
	}

	snap, err := executor.Snapshot("network")
	if err != nil {
		return ProbePortRoles(), false, fmt.Errorf("snapshot network: %w", err)
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("network", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		return ProbePortRoles(), true, err
	}
	return ProbePortRoles(), false, nil
}

// normalizePortMAC cleans copy-paste artifacts (spaces, CRLFs, dashes) and
// validates the result: empty means "back to factory", anything else must be
// a unicast, non-null MAC with the canonical colon form.
func normalizePortMAC(mac string) (string, error) {
	clean := strings.TrimSpace(mac)
	if clean == "" {
		return "", nil
	}
	hw, err := net.ParseMAC(clean)
	if err != nil || len(hw) != 6 {
		return "", fmt.Errorf("invalid MAC address %q: expected aa:bb:cc:dd:ee:ff", mac)
	}
	if hw[0]&1 != 0 {
		return "", fmt.Errorf("invalid MAC address %q: must be a unicast address", mac)
	}
	for _, b := range hw {
		if b != 0 {
			return strings.ToLower(hw.String()), nil
		}
	}
	return "", fmt.Errorf("invalid MAC address %q", mac)
}

// namedDeviceSection builds a stable, valid UCI section name for a port that
// has no device section yet.
func namedDeviceSection(port string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, port)
	return "port_" + safe
}
