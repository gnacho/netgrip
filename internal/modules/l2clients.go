package modules

import (
	"os"
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
// the netlink types so the assembly logic is unit-testable. Port is the
// bridge port the entry belongs to: the dump is issued per port, so the port
// is known by construction instead of resolved from an interface index.
type l2Neigh struct {
	mac   string
	port  string
	vlan  int
	flags int
	state int
	used  uint32
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

// probeL2ClientsImpl reads the bridge FDB over netlink AF_BRIDGE.
//
// The dump is issued PER PORT, not against the bridge: a dump filtered by
// the bridge's own index returns only the bridge's local entries (the
// multicast addresses of the device itself) and zero learned ones, so a view
// built that way is permanently empty. Field-verified on a live router (rt3,
// 4 ethernet ports and 2 radios configured): filter by bridge -> 8 entries,
// 0 learned; per port -> 60 learned entries spread over its ports. Asking
// per port has a second advantage: the port of every entry is known by
// construction, with no index-to-name resolution in the loop.
func probeL2ClientsImpl() *L2Probe {
	bridge := LANBridge()
	notApplicable := &L2Probe{Applicable: false, Bridge: bridge, Ports: []string{}, Clients: []L2Client{}}
	if bridge == "" {
		return notApplicable
	}
	ports := make([]string, 0, 4)
	for p := range bridgePortsOf(bridge) {
		ports = append(ports, p)
	}
	sort.Strings(ports)
	notApplicable.Ports = ports
	if len(ports) == 0 {
		return notApplicable
	}

	var entries []l2Neigh
	readable := 0
	for _, name := range ports {
		link, err := netlink.LinkByName(name)
		if err != nil {
			continue
		}
		neighs, err := netlink.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
		if err != nil {
			continue
		}
		readable++
		own := portOwnMAC(name)
		for _, n := range neighs {
			mac := strings.ToLower(n.HardwareAddr.String())
			// A port also publishes its own address in the FDB (hostapd adds
			// the BSSID MAC as a static entry, the WAN port does the same):
			// that is the device talking about itself, never a client.
			if mac == "" || mac == own {
				continue
			}
			entries = append(entries, l2Neigh{
				mac:   mac,
				port:  name,
				vlan:  n.Vlan,
				flags: n.Flags,
				state: n.State,
				used:  n.Used,
			})
		}
	}
	if readable == 0 {
		return notApplicable
	}
	return &L2Probe{Applicable: true, Bridge: bridge, Ports: ports, Clients: assembleL2Clients(entries)}
}

// portOwnMAC is the hardware address of a bridge port, lowercased, or "" when
// it cannot be read.
func portOwnMAC(name string) string {
	data, err := os.ReadFile("/sys/class/net/" + name + "/address")
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(data)))
}

// assembleL2Clients groups FDB entries by (MAC, port), accumulating the
// VLANs each device carries on that port, then sorts the result by port name
// and MAC for a stable order. A MAC learned on two different ports stays as
// two rows: that is what loop detection looks for (same rationale as
// parseBrctlShowmacs). A single (MAC, port) on several VLANs collapses into
// one row instead of one row per VLAN (#520).
func assembleL2Clients(entries []l2Neigh) []L2Client {
	// Key is MAC + "\x00" + port; accumulate VLANs and dedupe them when
	// folding.
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
		port := e.port
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
					Static: isStaticFdbState(e.state),
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
		g.client.Vlans = normalizeVlans(g.client.Vlans)
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

// isStaticFdbState reports whether an FDB entry was configured rather than
// learned. The bridge dump marks a static entry as NUD_NOARP, and hostapd's
// own-BSSID entries (and the WAN port's) come back as NUD_PERMANENT; a
// learned entry is REACHABLE or STALE.
func isStaticFdbState(state int) bool {
	return state == unix.NUD_NOARP || state == unix.NUD_PERMANENT
}

// normalizeVlans drops the unfiltered marker (VLAN 0) when the entry is also
// seen on real VLANs: a trunked device carrying VLANs 10 and 20 must read
// "10, 20", not "sin VLAN, 10, 20". A lone VLAN 0 is kept, and the UI shows
// it as "no VLAN filtering".
func normalizeVlans(vlans []int) []int {
	if len(vlans) < 2 {
		return vlans
	}
	out := make([]int, 0, len(vlans))
	for _, v := range vlans {
		if v != 0 {
			out = append(out, v)
		}
	}
	return out
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
