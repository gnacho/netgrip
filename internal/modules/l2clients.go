package modules

import (
	"net"
	"sort"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// L2Client is one learned MAC in the switch's bridge FDB, shown in the L2
// client inventory (#520). There is no DHCP/IP semantics here: a pure L2
// switch only knows the MAC, the port it was learned on and the VLANs it
// carries. Vlans is sorted ascending; an empty list (or [0]) means the
// entry is not filtered by VLAN.
type L2Client struct {
	MAC    string `json:"mac"`
	Vendor string `json:"vendor,omitempty"`
	Random bool   `json:"random,omitempty"`
	Port   string `json:"port"`
	Vlans  []int  `json:"vlans"`
	Age    int    `json:"age,omitempty"`
	Static bool   `json:"static,omitempty"`
}

// L2Probe is the payload for GET /api/l2clients. Applicable is false when
// there is no resolvable bridge or the FDB read failed; in that case Clients
// is empty and Ports still carries the bridge's ports so the UI can show a
// coherent "not applicable" state.
type L2Probe struct {
	Applicable bool       `json:"applicable"`
	Bridge     string     `json:"bridge,omitempty"`
	Ports      []string   `json:"ports,omitempty"`
	Clients    []L2Client `json:"clients"`
}

// l2Neigh is a bridge FDB entry already parsed from netlink, decoupled from
// the netlink types so the assembly logic is unit-testable.
type l2Neigh struct {
	mac     string
	vlan    int
	linkIdx int
	flags   int
	state   int
	used    uint32
}

// probeL2ClientsFn is the seam for tests; it defaults to the real read.
var probeL2ClientsFn = probeL2ClientsImpl

// ProbeL2Clients returns the L2 client inventory for the configured LAN
// bridge via netlink AF_BRIDGE (the FDB). Unlike the router/AP clients view
// it never invokes the `bridge` binary (absent on some targets) nor relies
// on DHCP leases or ARP.
func ProbeL2Clients() *L2Probe {
	return probeL2ClientsFn()
}

func probeL2ClientsImpl() *L2Probe {
	bridge := LANBridge()
	ports := bridgePortList()
	if bridge == "" {
		return &L2Probe{Applicable: false, Bridge: bridge, Ports: ports, Clients: []L2Client{}}
	}
	link, err := netlink.LinkByName(bridge)
	if err != nil {
		return &L2Probe{Applicable: false, Bridge: bridge, Ports: ports, Clients: []L2Client{}}
	}
	neighs, err := netlink.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
	if err != nil {
		return &L2Probe{Applicable: false, Bridge: bridge, Ports: ports, Clients: []L2Client{}}
	}
	entries := make([]l2Neigh, 0, len(neighs))
	for _, n := range neighs {
		mac := strings.ToLower(n.HardwareAddr.String())
		if mac == "" {
			continue
		}
		entries = append(entries, l2Neigh{
			mac:     mac,
			vlan:    n.Vlan,
			linkIdx: n.LinkIndex,
			flags:   n.Flags,
			state:   n.State,
			used:    n.Used,
		})
	}
	clients := assembleL2Clients(entries, portNameForIndex)
	return &L2Probe{Applicable: true, Bridge: bridge, Ports: ports, Clients: clients}
}

// portNameForIndex resolves a bridge port index to its interface name,
// preferring netlink (which knows bridge ports by index) and falling back to
// the standard net interfaces table.
func portNameForIndex(idx int) string {
	if l, err := netlink.LinkByIndex(idx); err == nil && l != nil {
		return l.Attrs().Name
	}
	if ifc, err := net.InterfaceByIndex(idx); err == nil {
		return ifc.Name
	}
	return ""
}

// assembleL2Clients groups FDB entries by (MAC, port), accumulating the
// VLANs each device carries on that port, then sorts the result by port name
// and MAC for a stable order. A MAC learned on two different ports stays as
// two rows: that is what loop detection looks for (same rationale as
// parseBrctlShowmacs). A single (MAC, port) on several VLANs collapses into
// one row instead of one row per VLAN (#520).
func assembleL2Clients(entries []l2Neigh, portOf func(int) string) []L2Client {
	// Key is MAC + "\x00" + port; accumulate VLANs in insertion order and
	// dedupe them when folding.
	type acc struct {
		client L2Client
		seen   map[int]bool
	}
	groups := map[string]*acc{}
	var order []string
	for _, e := range entries {
		// The bridge's own local entries (NTF_SELF) are not clients.
		if e.flags&unix.NTF_SELF != 0 {
			continue
		}
		port := portOf(e.linkIdx)
		if port == "" {
			continue
		}
		key := e.mac + "\x00" + port
		g, ok := groups[key]
		if !ok {
			g = &acc{
				client: L2Client{
					MAC:    e.mac,
					Port:   port,
					Static: e.state == unix.NUD_NOARP,
					Age:    ageFromTicks(e.used),
				},
				seen: map[int]bool{},
			}
			groups[key] = g
			order = append(order, key)
		}
		if !g.seen[e.vlan] {
			g.seen[e.vlan] = true
			g.client.Vlans = append(g.client.Vlans, e.vlan)
		}
	}

	clients := make([]L2Client, 0, len(order))
	for _, key := range order {
		g := groups[key]
		sort.Ints(g.client.Vlans)
		g.client.Vendor, g.client.Random = vendorFor(g.client.MAC)
		clients = append(clients, g.client)
	}
	sort.Slice(clients, func(i, j int) bool {
		if clients[i].Port != clients[j].Port {
			return clients[i].Port < clients[j].Port
		}
		return clients[i].MAC < clients[j].MAC
	})
	return clients
}

// ageFromTicks converts a netlink "used" clock-tick count into seconds. The
// kernel expresses it in clock ticks; _SC_CLK_TCK is 100 on every OpenWrt
// target NetGrip runs on, so divide by 100. Zero means "unknown age" and is
// left as zero rather than guessed from another source.
func ageFromTicks(used uint32) int {
	if used == 0 {
		return 0
	}
	return int(used / 100)
}
