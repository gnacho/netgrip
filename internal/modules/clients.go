package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gnacho/owpanel/internal/executor"
	"github.com/gnacho/owpanel/internal/ubus"
)

// Client is one network client in the clients table.
type Client struct {
	Name       string `json:"name"`
	IP         string `json:"ip,omitempty"`
	MAC        string `json:"mac"`
	Type       string `json:"type"` // wifi24 | wifi5 | cable
	Iface      string `json:"iface,omitempty"`
	Signal     int    `json:"signal,omitempty"`
	RxBytes    int64  `json:"rx_bytes"` // client upload (AP rx)
	TxBytes    int64  `json:"tx_bytes"` // client download (AP tx)
	Self       bool   `json:"self"`
	Reserved   bool   `json:"reserved"`
	Reservable bool   `json:"reservable"`
	Blocked    bool   `json:"blocked"`
	Blockable  bool   `json:"blockable"`
}

var reMac = regexp.MustCompile(`^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$`)

// ListClients builds the clients table: wireless stations (with byte
// counters for rates/totals), wired devices from the bridge FDB, DHCP
// leases for names/IPs, reservation and block states, and the caller's
// own device flagged as self.
func ListClients(requesterIP string) []Client {
	leases, _ := ubus.ReadLeases("/tmp/dhcp.leases")
	byMac := map[string]ubus.Lease{}
	for _, l := range leases {
		byMac[strings.ToLower(l.MAC)] = l
	}
	reserved := reservedMACs()
	blocked := blockedMACs()
	dnsmasqUp := executor.ServiceRunning("dnsmasq")

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
					MAC:      mac,
					Type:     typ,
					Iface:    iface.Ifname,
					Signal:   wc.Signal,
					RxBytes:  wc.RxBytes,
					TxBytes:  wc.TxBytes,
					Blocked:  blocked[mac],
					Blockable: true,
				}
				if l, ok := byMac[mac]; ok {
					c.Name = l.Hostname
					c.IP = l.IP
					c.Self = requesterIP != "" && l.IP == requesterIP
				}
				if c.Name == "" || c.Name == "*" {
					c.Name = mac
				}
				c.Reserved = reserved[mac]
				c.Reservable = dnsmasqUp && c.IP != ""
				clients = append(clients, c)
			}
		}
	}

	// Wired clients from the FDB (no byte counters available).
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
			c := Client{MAC: mac, Type: "cable", Iface: port, Blocked: blocked[mac]}
			if l, ok := byMac[mac]; ok {
				c.Name = l.Hostname
				c.IP = l.IP
				c.Self = requesterIP != "" && l.IP == requesterIP
			}
			if c.Name == "" || c.Name == "*" {
				c.Name = mac
			}
			c.Reserved = reserved[mac]
			c.Reservable = dnsmasqUp && c.IP != ""
			c.Blockable = executor.ServiceEnabled("firewall")
			clients = append(clients, c)
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

func reservedMACs() map[string]bool {
	out, err := exec.Command("sh", "-c", "uci show dhcp | grep '=host' | cut -d. -f2 | cut -d= -f1").Output()
	reserved := map[string]bool{}
	if err != nil {
		return reserved
	}
	for _, section := range strings.Fields(string(out)) {
		mac := strings.ToLower(uciGet("dhcp." + section + ".mac"))
		if mac != "" {
			reserved[mac] = true
		}
	}
	return reserved
}

func blockedMACs() map[string]bool {
	blocked := map[string]bool{}
	out, err := exec.Command("sh", "-c", "uci show wireless | grep '=wifi-iface' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return blocked
	}
	for _, section := range strings.Fields(string(out)) {
		if uciGet("wireless."+section+".macfilter") != "deny" {
			continue
		}
		for _, mac := range strings.Fields(strings.ToLower(uciGet("wireless." + section + ".maclist"))) {
			if reMac.MatchString(mac) {
				blocked[mac] = true
			}
		}
	}
	return blocked
}

func hostSections() []string {
	out, err := exec.Command("sh", "-c", "uci show dhcp | grep '=host' | cut -d. -f2 | cut -d= -f1 | grep '^owpanel_host_'").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(out))
}

// SetClientReservation adds or removes a DHCP reservation for a MAC
// (gateway only: needs dnsmasq). Snapshot + reload + rollback.
func SetClientReservation(mac, ip string, reserved bool) (*[]Client, bool, error) {
	mac = strings.ToLower(mac)
	if !executor.ServiceRunning("dnsmasq") {
		return nil, false, fmt.Errorf("DHCP reservations only apply on the gateway (dnsmasq)")
	}
	if !reMac.MatchString(mac) || (reserved && !reIPv4.MatchString(ip)) {
		return nil, false, fmt.Errorf("invalid mac/ip")
	}
	snap, err := executor.Snapshot("dhcp")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		_ = executor.Restore("dhcp", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "reload"}})
	}
	section := "owpanel_host_" + strings.ReplaceAll(mac, ":", "")
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

// SetClientBlocked blocks or unblocks a client. WiFi clients go into the
// macfilter deny list of every wireless iface (with a radio reload);
// wired clients need a firewall REJECT rule (gateway/firewall only).
func SetClientBlocked(mac, typ string, blocked bool) (*[]Client, bool, error) {
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
		_ = executor.Run(executor.Op{Kind: "wifi_reload", Args: []string{}})
	}

	sections := wifiIfaceSections()
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
	radios := radiosForSections(sections)
	for _, radio := range radios {
		ops = append(ops, executor.Op{Kind: "wifi_reload", Args: []string{radio}})
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
	section := "owpanel_block_" + strings.ReplaceAll(mac, ":", "")
	base := "firewall." + section
	var ops []executor.Op
	if blocked {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{base, "rule"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".name", "owpanel-block-" + mac}},
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
