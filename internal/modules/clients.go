package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// Client is one network client in the clients table.
type Client struct {
	Name        string   `json:"name"`
	IP          string   `json:"ip,omitempty"`
	MAC         string   `json:"mac"`
	Type        string   `json:"type"`                  // wifi24 | wifi5 | cable
	DeviceType  string   `json:"device_type,omitempty"` // user-assigned: pc | phone | ...
	Iface       string   `json:"iface,omitempty"`
	Signal      int      `json:"signal,omitempty"`
	RxBytes     int64    `json:"rx_bytes"` // client upload (AP rx)
	TxBytes     int64    `json:"tx_bytes"` // client download (AP tx)
	Self        bool     `json:"self"`
	Reserved    bool     `json:"reserved"`
	Reservable  bool     `json:"reservable"`
	Blocked     bool     `json:"blocked"`
	BlockedOn   []string `json:"blocked_on,omitempty"` // bands with a deny entry (partial blocks)
	Blockable   bool     `json:"blockable"`
	LeaseExpiry int64    `json:"lease_expiry,omitempty"`
	LeaseSource string   `json:"lease_source,omitempty"` // local | gateway
	IPSource    string   `json:"ip_source,omitempty"`    // arp: IP resolved from the neighbor table, no DHCP lease (#212)
}

var reMac = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)

// ListClients builds the clients table: wireless stations (with byte
// counters for rates/totals), wired devices from the bridge FDB, DHCP
// leases for names/IPs, reservation and block states, and the caller's
// own device flagged as self.
func ListClients(requesterIP string) []Client {
	leases, leaseSource, _ := leasesForClients()
	byMac := map[string]ubus.Lease{}
	for _, l := range leases {
		byMac[strings.ToLower(l.MAC)] = l
	}
	// Static wired clients have no lease; their current IP still shows up
	// in the neighbor table (#212). On APs the authoritative ARP table is
	// the gateway's, same as leases.
	arp := localArp()
	if leaseSource == "gateway" {
		if out, err := gatewaySSH("cat /proc/net/arp"); err == nil {
			arp = mergeArp(arp, parseProcArp(out))
		}
	}
	reserved := reservedMACs()
	denied, availBands := blockedBands()
	isAP := ProbeMode().Mode == "ap"

	var clients []Client

	radios, _ := ubus.GetWirelessStatus()
	for _, radio := range radios {
		typ := "wifi24"
		if radio.Band == "5g" {
			typ = "wifi5"
		}
		for _, iface := range radio.Interfaces {
			for _, wc := range iface.Clients {
				mac := strings.ToLower(wc.MAC)
				c := Client{
					MAC:       mac,
					Type:      typ,
					Iface:     iface.Ifname,
					Signal:    wc.Signal,
					RxBytes:   wc.RxBytes,
					TxBytes:   wc.TxBytes,
					Blockable: true,
				}
				if on := denied[mac]; len(on) > 0 {
					c.BlockedOn = bandsList(on)
					c.Blocked = blockedEverywhere(on, availBands)
				}
				fillIdentity(&c, mac, requesterIP, leaseSource, byMac, arp)
				c.Reserved = reserved[mac]
				c.Reservable = c.IP != ""
				clients = append(clients, c)
			}
		}
	}

	// Wired clients from the FDB (no byte counters available). On an AP the
	// bridge learns the whole upstream LAN through the uplink port, so only
	// show wired clients when the router is the gateway (router mode); a dumb
	// AP's clients are its wireless associates.
	if !isAP {
		wifiPorts := map[string]bool{}
		for _, radio := range radios {
			for _, iface := range radio.Interfaces {
				wifiPorts[iface.Ifname] = true
			}
		}
		for port, macs := range bridgeFdb() {
			if wifiPorts[port] {
				continue
			}
			for _, mac := range macs {
				mac = strings.ToLower(mac)
				c := Client{MAC: mac, Type: "cable", Iface: port, Blocked: len(denied[mac]) > 0}
				fillIdentity(&c, mac, requesterIP, leaseSource, byMac, arp)
				c.Reserved = reserved[mac]
				c.Reservable = c.IP != ""
				c.Blockable = executor.ServiceEnabled("firewall")
				clients = append(clients, c)
			}
		}
	}

	// Apply user-assigned metadata (custom name + device type) overrides.
	meta := clientMeta()
	for i, c := range clients {
		if m, ok := meta[strings.ToLower(c.MAC)]; ok {
			if m.Name != "" {
				clients[i].Name = m.Name
			}
			clients[i].DeviceType = m.DeviceType
		}
	}

	// Self first, then by name.
	for i, c := range clients {
		if c.Self && i != 0 {
			clients = append([]Client{c}, append(clients[:i], clients[i+1:]...)...)
			break
		}
	}
	if clients == nil {
		return []Client{}
	}
	return clients
}

// gatewayAddr resolves the default gateway address (the DHCP server for
// the LAN on dumb APs).
func gatewayAddr() string {
	out, err := exec.Command("sh", "-c", "ip route show default | awk '{print $3}' | head -1").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gatewaySSH runs a command on the gateway with the dedicated netgrip_ro key.
// dropbear's ssh client needs -y to accept the host key and does not support
// -o BatchMode/ConnectTimeout. This is used both for read-only lease lookups
// and for writing DHCP reservations on the gateway when the local device is an AP.
func gatewaySSH(command string) (string, error) {
	gw := gatewayAddr()
	if gw == "" {
		return "", fmt.Errorf("no default gateway")
	}
	// #161: routers set up before the rename keep the key under the old
	// owpanel_ro filename; use it when the netgrip_ro key is not there.
	key := "/root/.ssh/netgrip_ro"
	if _, err := os.Stat(key); err != nil {
		if _, errLegacy := os.Stat("/root/.ssh/owpanel_ro"); errLegacy == nil {
			key = "/root/.ssh/owpanel_ro"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-y", "-i", key, "root@"+gw, command).Output()
	if err != nil {
		return "", fmt.Errorf("gateway ssh: %w", err)
	}
	return string(out), nil
}

// leasesForClients reads DHCP leases locally; on dumb APs without local
// dnsmasq it falls back to the gateway's lease file over SSH.
func leasesForClients() ([]ubus.Lease, string, error) {
	leases, _ := ubus.ReadLeases("/tmp/dhcp.leases")
	if len(leases) > 0 {
		return leases, "local", nil
	}
	out, err := gatewaySSH("cat /tmp/dhcp.leases")
	if err != nil {
		return []ubus.Lease{}, "", fmt.Errorf("gateway leases: %w", err)
	}
	return ubus.ParseLeases(out), "gateway", nil
}

// localArp reads this router's neighbor table.
func localArp() map[string]string {
	raw, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return map[string]string{}
	}
	return parseProcArp(string(raw))
}

// parseProcArp parses /proc/net/arp content into a MAC -> IP map. Only
// resolved entries count: incomplete rows (flags 0x0) and failed lookups
// (all-zero MAC) are skipped, MACs come out lowercase.
func parseProcArp(content string) map[string]string {
	arp := map[string]string{}
	for i, line := range strings.Split(content, "\n") {
		if i == 0 || line == "" {
			continue // header
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		ip, flags, mac := fields[0], fields[2], strings.ToLower(fields[3])
		if flags == "0x0" || !reMac.MatchString(mac) || mac == "00:00:00:00:00:00" {
			continue
		}
		arp[mac] = ip
	}
	return arp
}

// mergeArp fills gaps in base with entries from extra (base wins on
// conflicts), leaving both inputs untouched.
func mergeArp(base, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for m, ip := range base {
		merged[m] = ip
	}
	for m, ip := range extra {
		if _, ok := merged[m]; !ok {
			merged[m] = ip
		}
	}
	return merged
}

// fillIdentity fills name, IP, lease data and the self flag for one
// client from the lease map, falling back to the ARP table for the IP
// when the device has no DHCP lease (#212).
func fillIdentity(c *Client, mac, requesterIP, leaseSource string, byMac map[string]ubus.Lease, arp map[string]string) {
	if l, ok := byMac[mac]; ok {
		c.Name = l.Hostname
		c.IP = l.IP
		c.Self = requesterIP != "" && l.IP == requesterIP
		c.LeaseExpiry = l.Expires.Unix()
		c.LeaseSource = leaseSource
	}
	if c.IP == "" {
		if ip, ok := arp[mac]; ok {
			c.IP = ip
			c.IPSource = "arp"
			c.Self = requesterIP != "" && ip == requesterIP
		}
	}
	if c.Name == "" || c.Name == "*" {
		c.Name = mac
	}
}

// reservedMACs lists MACs with a DHCP reservation, locally or, on dumb
// APs, from the gateway's dhcp config (read-only).
func reservedMACs() map[string]bool {
	reserved := map[string]bool{}
	local, err := exec.Command("sh", "-c", "uci show dhcp | grep '=host' | cut -d. -f2 | cut -d= -f1").Output()
	if err == nil && len(strings.Fields(string(local))) > 0 {
		for _, section := range strings.Fields(string(local)) {
			mac := strings.ToLower(uciGet("dhcp." + section + ".mac"))
			if mac != "" {
				reserved[mac] = true
			}
		}
		return reserved
	}
	// Fallback: parse the gateway's /etc/config/dhcp host blocks.
	out, err := gatewaySSH("cat /etc/config/dhcp")
	if err != nil {
		return reserved
	}
	inHost := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "config host") {
			inHost = true
			continue
		}
		if strings.HasPrefix(line, "config ") {
			inHost = false
		}
		if inHost && (strings.HasPrefix(line, "option mac") || strings.HasPrefix(line, "list mac")) {
			// GL firmware stores mac as a LIST (multiple MACs per host entry)
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				mac := strings.ToLower(strings.Trim(parts[2], "'\""))
				if reMac.MatchString(mac) {
					reserved[mac] = true
				}
			}
		}
	}
	return reserved
}

// blockedBands maps each denied MAC to the set of bands whose wifi-iface
// carries a macfilter=deny entry for it (#160). Available bands come from
// every wifi-iface, so a deny on all bands (what the in-app block action
// writes) can be told apart from a single-band deny (e.g. steering bans).
func blockedBands() (denied map[string]map[string]bool, avail map[string]bool) {
	denied = map[string]map[string]bool{}
	avail = map[string]bool{}
	out, err := exec.Command("sh", "-c", "uci show wireless | grep '=wifi-iface' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return denied, avail
	}
	for _, section := range strings.Fields(string(out)) {
		radio := uciGet("wireless." + section + ".device")
		if radio == "" {
			continue
		}
		band := uciGet("wireless." + radio + ".band")
		if band == "" {
			continue
		}
		avail[band] = true
		if uciGet("wireless."+section+".macfilter") != "deny" {
			continue
		}
		for _, mac := range strings.Fields(strings.ToLower(uciGet("wireless." + section + ".maclist"))) {
			if !reMac.MatchString(mac) {
				continue
			}
			if denied[mac] == nil {
				denied[mac] = map[string]bool{}
			}
			denied[mac][band] = true
		}
	}
	return denied, avail
}

// bandsList flattens a band set into a stable order.
func bandsList(set map[string]bool) []string {
	list := make([]string, 0, len(set))
	for b := range set {
		list = append(list, b)
	}
	sort.Strings(list)
	return list
}

// blockedEverywhere reports whether the deny covers every available band.
func blockedEverywhere(set, avail map[string]bool) bool {
	if len(avail) == 0 || len(set) < len(avail) {
		return false
	}
	for b := range avail {
		if !set[b] {
			return false
		}
	}
	return true
}

func hostSections() []string {
	out, err := exec.Command("sh", "-c", "uci show dhcp | grep '=host' | cut -d. -f2 | cut -d= -f1 | grep '^netgrip_host_'").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(out))
}

// gatewaySSHExec runs a command on the gateway and returns an error if it fails.
// Used for write operations (DHCP reservations) when the local device is an AP.
func gatewaySSHExec(command string) error {
	gw := gatewayAddr()
	if gw == "" {
		return fmt.Errorf("no default gateway")
	}
	key := "/root/.ssh/netgrip_ro"
	if _, err := os.Stat(key); err != nil {
		if _, errLegacy := os.Stat("/root/.ssh/owpanel_ro"); errLegacy == nil {
			key = "/root/.ssh/owpanel_ro"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-y", "-i", key, "root@"+gw, command).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gateway ssh %q: %w (%s)", command, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SetClientReservation adds or removes a DHCP reservation for a MAC.
// On the gateway itself it uses local dnsmasq/UCI. On APs it delegates the
// same UCI operations to the gateway over SSH (#192).
func SetClientReservation(mac, ip string, reserved bool) (*[]Client, bool, error) {
	mac = strings.ToLower(mac)
	if !reMac.MatchString(mac) || (reserved && !reIPv4.MatchString(ip)) {
		return nil, false, fmt.Errorf("invalid mac/ip")
	}
	localDnsmasq := executor.ServiceRunning("dnsmasq")
	if !localDnsmasq {
		// AP path: write the reservation on the gateway.
		return setGatewayClientReservation(mac, ip, reserved)
	}

	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}
	section := "netgrip_host_" + strings.ReplaceAll(mac, ":", "")
	var ops []executor.Op
	if reserved {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"dhcp." + section, "host"}},
			executor.Op{Kind: "uci_set", Args: []string{"dhcp." + section + ".mac", mac}},
			executor.Op{Kind: "uci_set", Args: []string{"dhcp." + section + ".ip", ip}},
		)
	} else {
		if uciSectionExists("dhcp." + section) {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"dhcp." + section}})
		}
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dhcp"}},
		executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return nil, true, err
	}
	return nil, false, nil
}

// setGatewayClientReservation applies a DHCP reservation on the gateway via SSH.
// It intentionally does not snapshot/rollback over SSH; failures are surfaced to the UI.
func setGatewayClientReservation(mac, ip string, reserved bool) (*[]Client, bool, error) {
	if err := gatewaySSHExec("/etc/init.d/dnsmasq running"); err != nil {
		return nil, false, fmt.Errorf("DHCP reservations on the gateway need a running dnsmasq")
	}
	section := "netgrip_host_" + strings.ReplaceAll(mac, ":", "")
	var commands []string
	if reserved {
		commands = append(commands,
			"uci set dhcp."+section+"=host",
			"uci set dhcp."+section+".mac="+mac,
			"uci set dhcp."+section+".ip="+ip,
		)
	} else {
		commands = append(commands, "uci delete dhcp."+section+" 2>/dev/null || true")
	}
	commands = append(commands,
		"uci commit dhcp",
		"/etc/init.d/dnsmasq reload",
	)
	if err := gatewaySSHExec(strings.Join(commands, " && ")); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

// SetClientBlocked blocks or unblocks a client. WiFi clients go into the
// macfilter deny list of every wireless iface (with a radio reload);
// wired clients need a firewall REJECT rule (gateway/firewall only).
// band selects the scope for WiFi: "" (all bands), "2g", "5g" or "6g";
// unblocking with band "" removes every deny entry (#163).
func SetClientBlocked(mac, typ, band string, blocked bool) (*[]Client, bool, error) {
	mac = strings.ToLower(mac)
	if !reMac.MatchString(mac) {
		return nil, false, fmt.Errorf("invalid mac")
	}
	if typ == "cable" {
		return setCableBlocked(mac, blocked)
	}

	snap, err := executor.Snapshot("wireless")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("wireless", snap)
		_ = executor.Run(executor.Op{Kind: "wifi_reconf", Args: []string{}})
	}

	sections := wifiIfaceSections()
	sections, err = sectionsForBand(sections, band, ifaceBand)
	if err != nil {
		return nil, false, err
	}
	var ops []executor.Op
	for _, section := range sections {
		base := "wireless." + section
		if blocked {
			if uciGet(base+".macfilter") != "deny" {
				ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".macfilter", "deny"}})
			}
			present := false
			for _, m := range strings.Fields(strings.ToLower(uciGet(base + ".maclist"))) {
				if m == mac {
					present = true
				}
			}
			if !present {
				ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{base + ".maclist", mac}})
			}
		} else {
			for _, m := range strings.Fields(strings.ToLower(uciGet(base + ".maclist"))) {
				if m == mac {
					ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{base + ".maclist", mac}})
				}
			}
		}
	}
	if len(ops) == 0 {
		return nil, false, nil
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	// Use wifi reconf to apply MAC ACL changes without tearing down the
	// BSS and disconnecting every other station on the radio (#183).
	radios := radiosForSections(sections)
	for _, radio := range radios {
		ops = append(ops, executor.Op{Kind: "wifi_reconf", Args: []string{radio}})
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return nil, true, err
	}
	return nil, false, nil
}

func setCableBlocked(mac string, blocked bool) (*[]Client, bool, error) {
	if !executor.ServiceEnabled("firewall") {
		return nil, false, fmt.Errorf("blocking wired clients needs the firewall (gateway)")
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	section := "netgrip_block_" + strings.ReplaceAll(mac, ":", "")
	base := "firewall." + section
	var ops []executor.Op
	if blocked {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{base, "rule"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".name", "netgrip-block-" + mac}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".src", "lan"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".src_mac", mac}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".target", "REJECT"}},
		)
	} else if uciSectionExists(base) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{base}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return nil, true, err
	}
	return nil, false, nil
}

func wifiIfaceSections() []string {
	out, err := exec.Command("sh", "-c", "uci show wireless | grep '=wifi-iface' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(out))
}

// ifaceBand resolves the band ("2g"/"5g"/...) of a wifi-iface section via
// its parent radio device.
func ifaceBand(section string) string {
	radio := uciGet("wireless." + section + ".device")
	if radio == "" {
		return ""
	}
	return uciGet("wireless." + radio + ".band")
}

// sectionsForBand filters wifi-iface sections down to one band. An empty
// band returns every section; an unknown band (not served by any radio)
// is an error so callers cannot silently no-op (#163).
func sectionsForBand(sections []string, band string, bandOf func(string) string) ([]string, error) {
	if band == "" {
		return sections, nil
	}
	var out []string
	for _, section := range sections {
		if bandOf(section) == band {
			out = append(out, section)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no wireless interface on band %s", band)
	}
	return out, nil
}

// BlockedClient is a MAC with an active block, whether or not the
// device is currently associated.
type BlockedClient struct {
	MAC               string   `json:"mac"`
	Type              string   `json:"type"`            // wifi | cable
	Bands             []string `json:"bands,omitempty"` // wifi only: 2g / 5g / 6g
	BlockedEverywhere bool     `json:"blocked_everywhere"`
}

// BlockedClients lists every MAC that is currently denied somewhere:
// wireless clients via macfilter=deny in any wifi-iface (#160), wired
// clients via a firewall REJECT rule (#96). Unlike ListClients, this
// does NOT require the device to be associated right now, so the
// modal can unblock a client that got kicked off the radios.
func BlockedClients() []BlockedClient {
	denied, avail := blockedBands()
	out := make([]BlockedClient, 0, len(denied))
	for mac, set := range denied {
		bands := bandsList(set)
		out = append(out, BlockedClient{
			MAC:               mac,
			Type:              "wifi",
			Bands:             bands,
			BlockedEverywhere: blockedEverywhere(set, avail),
		})
	}
	// Wired: firewall rules written by setCableBlocked.
	cmdOut, _ := exec.Command("sh", "-c", "uci show firewall | grep 'src_mac='").Output()
	for _, line := range strings.Split(string(cmdOut), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "=netgrip-block-") {
			continue
		}
		// firewall.netgrip_block_<mac>.src_mac='<mac>'
		eq := strings.Index(line, ".src_mac=")
		if eq < 0 {
			continue
		}
		start := strings.LastIndex(line[:eq], ".")
		if start < 0 {
			continue
		}
		mac := strings.ToLower(strings.Trim(line[eq+len(".src_mac="):], "'\""))
		if !reMac.MatchString(mac) {
			continue
		}
		already := false
		for i := range out {
			if out[i].MAC == mac {
				out[i].Type = "cable"
				already = true
				break
			}
		}
		if !already {
			out = append(out, BlockedClient{MAC: mac, Type: "cable"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MAC < out[j].MAC })
	return out
}

// AvailableBands lists the wireless bands this router serves, for the
// clients payload (band selector in the block dialog).
func AvailableBands() []string {
	_, avail := blockedBands()
	return bandsList(avail)
}

func radiosForSections(sections []string) []string {
	seen := map[string]bool{}
	var radios []string
	for _, section := range sections {
		dev := uciGet("wireless." + section + ".device")
		if dev != "" && !seen[dev] {
			seen[dev] = true
			radios = append(radios, dev)
		}
	}
	return radios
}
