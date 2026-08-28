package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

const (
	wgIface        = "wg0"
	wgDefaultPort  = "51820"
	wgDefaultAddr  = "10.66.0.1/24"
	wgFirewallRule = "allow_netgrip_wg"
)

// WGPeer is one WireGuard peer as stored in UCI. The preshared key itself
// is never exposed through the API, only its presence.
type WGPeer struct {
	Section    string   `json:"section"`
	Name       string   `json:"name"`
	PublicKey  string   `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips"`
	Admin      bool     `json:"admin"`
	HasPSK     bool     `json:"has_psk"`
}

// WGProbe is the read-only WireGuard state.
type WGProbe struct {
	Installed  bool     `json:"installed"`
	Active     bool     `json:"active"`
	Running    bool     `json:"running"`
	ZoneMember bool     `json:"zone_member"`
	Port       string   `json:"port"`
	Address    string   `json:"address"`
	PublicKey  string   `json:"public_key"`
	Peers      []WGPeer `json:"peers"`
}

var reWGPubkey = regexp.MustCompile(`^[A-Za-z0-9+/]{43}=$`)

func wgInstalled() bool {
	_, err := exec.LookPath("wg")
	return err == nil
}

func wgShowIface() bool {
	out, err := exec.Command("wg", "show", wgIface).CombinedOutput()
	return err == nil && strings.Contains(string(out), "interface: "+wgIface)
}

func wgPublicKey(priv string) string {
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(priv + "\n")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ProbeWG reads the current WireGuard state from UCI and the kernel.
func ProbeWG() *WGProbe {
	p := &WGProbe{Peers: []WGPeer{}}
	p.Installed = wgInstalled()
	if !uciSectionExists("network." + wgIface) {
		return p
	}
	p.Active = true
	p.Port = uciGet("network." + wgIface + ".listen_port")
	p.Address = uciGet("network." + wgIface + ".addresses")
	if priv := uciGet("network." + wgIface + ".private_key"); priv != "" {
		p.PublicKey = wgPublicKey(priv)
	}
	p.Running = p.Installed && wgShowIface()
	p.ZoneMember = wgZoneSection() != ""
	p.Peers = wgPeers()
	return p
}

// wgZoneSection returns the firewall zone section whose network list
// contains wg0 ("" when absent).
func wgZoneSection() string {
	out, err := exec.Command("sh", "-c", "uci show firewall | grep '=zone' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return ""
	}
	for _, section := range strings.Fields(string(out)) {
		networks := uciGet("firewall." + section + ".network")
		for _, n := range strings.Fields(networks) {
			if n == wgIface {
				return section
			}
		}
	}
	return ""
}

// wgLanZoneSection returns the firewall section of the zone named 'lan'.
func wgLanZoneSection() string {
	out, err := exec.Command("sh", "-c", "uci show firewall | grep '=zone' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return ""
	}
	for _, section := range strings.Fields(string(out)) {
		if uciGet("firewall."+section+".name") == "lan" {
			return section
		}
	}
	return ""
}

func wgPeers() []WGPeer {
	out, err := exec.Command("sh", "-c", "uci show network | grep '=wireguard_"+wgIface+"' | cut -d. -f2 | cut -d= -f1").Output()
	if err != nil {
		return []WGPeer{}
	}
	peers := []WGPeer{}
	for _, section := range strings.Fields(string(out)) {
		base := "network." + section
		ipList, _ := exec.Command("sh", "-c", "uci get "+base+".allowed_ips 2>/dev/null").Output()
		allowed := strings.Fields(strings.TrimSpace(string(ipList)))
		if allowed == nil {
			allowed = []string{}
		}
		peers = append(peers, WGPeer{
			Section:    section,
			Name:       uciGet(base + ".description"),
			PublicKey:  uciGet(base + ".public_key"),
			AllowedIPs: allowed,
			Admin:      uciGet(base+".netgrip_admin") == "1",
			HasPSK:     uciGet(base+".preshared_key") != "",
		})
	}
	return peers
}

func wgApply(ops []executor.Op, snapNetwork, snapFirewall string, check func() bool) (bool, error) {
	if err := executor.Apply(ops, nil); err != nil {
		rollbackWG(snapNetwork, snapFirewall)
		return true, err
	}
	if !check() {
		rollbackWG(snapNetwork, snapFirewall)
		return true, fmt.Errorf("healthcheck failed after apply, rolled back")
	}
	return false, nil
}

func rollbackWG(snapNetwork, snapFirewall string) {
	_ = executor.Restore("network", snapNetwork)
	if snapFirewall != "" {
		_ = executor.Restore("firewall", snapFirewall)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
}

// SetWG enables or disables the WireGuard server.
func SetWG(enable bool) (*WGProbe, bool, error) {
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot network: %w", err)
	}
	snapFirewall, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot firewall: %w", err)
	}

	if enable {
		return enableWG(snapNetwork, snapFirewall)
	}
	return disableWG(snapNetwork, snapFirewall)
}

func enableWG(snapNetwork, snapFirewall string) (*WGProbe, bool, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	freshInstall := !wgInstalled()
	if freshInstall {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"wireguard-tools", "kmod-wireguard"}})
	}
	priv := uciGet("network." + wgIface + ".private_key")
	if priv == "" {
		out, err := exec.Command("wg", "genkey").Output()
		if err != nil {
			// wg binary missing before pkg_add: install first, then retry.
			if err := executor.Run(executor.Op{Kind: "pkg_add", Args: []string{"wireguard-tools", "kmod-wireguard"}}); err != nil {
				return ProbeWG(), false, fmt.Errorf("install wireguard packages: %w", err)
			}
			ops = nil
			out, err = exec.Command("wg", "genkey").Output()
			if err != nil {
				return ProbeWG(), false, fmt.Errorf("wg genkey: %w", err)
			}
		}
		priv = strings.TrimSpace(string(out))
	}
	set("network."+wgIface, "interface")
	set("network."+wgIface+".proto", "wireguard")
	set("network."+wgIface+".private_key", priv)
	if uciGet("network."+wgIface+".listen_port") == "" {
		set("network."+wgIface+".listen_port", wgDefaultPort)
	}
	if uciGet("network."+wgIface+".addresses") == "" {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + wgIface + ".addresses", wgDefaultAddr}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})

	// Firewall: only when fw4 is actually running (dumb APs disable it;
	// there `firewall reload` fails hard, verified on a Redmi AX6).
	// Two distinct things per the official server guide:
	//   1. input rule accepting UDP <port> from wan (clients reaching us)
	//   2. wg0 in the lan zone network list (tunnel traffic treated as LAN,
	//      which gives clients lan/wan access via the zone forwardings)
	if executor.ServiceEnabled("firewall") {
		src := "lan"
		if uciSectionExists("network.wan") {
			src = "wan"
		}
		set("firewall."+wgFirewallRule, "rule")
		set("firewall."+wgFirewallRule+".name", "Allow-netgrip-WireGuard")
		set("firewall."+wgFirewallRule+".src", src)
		set("firewall."+wgFirewallRule+".dest_port", uciGet("network."+wgIface+".listen_port"))
		set("firewall."+wgFirewallRule+".proto", "udp")
		set("firewall."+wgFirewallRule+".target", "ACCEPT")
		if zone := wgLanZoneSection(); zone != "" {
			member := false
			for _, n := range strings.Fields(uciGet("firewall." + zone + ".network")) {
				if n == wgIface {
					member = true
				}
			}
			if !member {
				ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"firewall." + zone + ".network", wgIface}})
			}
		}
		ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"firewall"}})
	}
	if freshInstall {
		// netifd only scans /lib/netifd/proto at startup: a proto handler
		// installed after boot is invisible until a full network restart
		// (reload and ifup are not enough; verified on 25.12.5).
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"network", "restart"}})
	} else {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
		// network reload does not always ifup a brand new interface; do it
		// explicitly so the healthcheck sees the kernel device.
		ops = append(ops, executor.Op{Kind: "ifup", Args: []string{wgIface}})
	}
	if executor.ServiceEnabled("firewall") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}

	rolledBack, err := wgApply(ops, snapNetwork, snapFirewall, func() bool {
		// After a network restart the interface takes a moment to come up.
		for range 10 {
			if wgShowIface() {
				if executor.ServiceEnabled("firewall") && wgLanZoneSection() != "" && wgZoneSection() == "" {
					return false
				}
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	})
	return ProbeWG(), rolledBack, err
}

func disableWG(snapNetwork, snapFirewall string) (*WGProbe, bool, error) {
	var ops []executor.Op
	// ifdown BEFORE removing the config: deleting the section and reloading
	// leaves the kernel device behind (verified); ifdown only exists while
	// netifd still has the interface config.
	if wgShowIface() {
		ops = append(ops, executor.Op{Kind: "ifdown", Args: []string{wgIface}})
	}
	for _, peer := range wgPeers() {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + peer.Section}})
	}
	if uciSectionExists("network." + wgIface) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + wgIface}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})
	firewallDirty := false
	if zone := wgZoneSection(); zone != "" {
		ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{"firewall." + zone + ".network", wgIface}})
		firewallDirty = true
	}
	if uciSectionExists("firewall." + wgFirewallRule) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"firewall." + wgFirewallRule}})
		firewallDirty = true
	}
	if firewallDirty {
		ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"firewall"}})
	}
	ops = append(ops, executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	if executor.ServiceEnabled("firewall") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	rolledBack, err := wgApply(ops, snapNetwork, snapFirewall, func() bool { return !wgShowIface() })
	return ProbeWG(), rolledBack, err
}

// AddWGPeer registers a peer. Admin peers can never be removed later.
func AddWGPeer(name, pubkey string, allowedIPs []string, admin bool) (*WGProbe, bool, error) {
	probe := ProbeWG()
	if !probe.Active {
		return probe, false, fmt.Errorf("wireguard is not enabled")
	}
	if !reWGPubkey.MatchString(pubkey) {
		return probe, false, fmt.Errorf("invalid public key format")
	}
	for _, p := range probe.Peers {
		if p.PublicKey == pubkey {
			return probe, false, fmt.Errorf("peer already exists")
		}
	}
	if len(allowedIPs) == 0 {
		allowedIPs = []string{nextWGAddress(probe)}
	}
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return probe, false, err
	}
	section := "wgpeer" + strconv.Itoa(len(probe.Peers))
	base := "network." + section
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{base, "wireguard_" + wgIface}},
		{Kind: "uci_set", Args: []string{base + ".public_key", pubkey}},
		// Best practices from the official guide: route peer traffic back
		// through the tunnel, and a per-peer preshared key on top of the
		// keypair (extra layer recommended by the WireGuard docs).
		{Kind: "uci_set", Args: []string{base + ".route_allowed_ips", "1"}},
	}
	if psk, err := exec.Command("wg", "genpsk").Output(); err == nil {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".preshared_key", strings.TrimSpace(string(psk))}})
	}
	if name != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".description", name}})
	}
	if admin {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".netgrip_admin", "1"}})
	}
	for _, ip := range allowedIPs {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{base + ".allowed_ips", ip}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)
	rolledBack, err := wgApply(ops, snapNetwork, "", func() bool {
		for _, p := range wgPeers() {
			if p.PublicKey == pubkey {
				return true
			}
		}
		return false
	})
	return ProbeWG(), rolledBack, err
}

// RemoveWGPeer deletes a peer by public key. Admin peers are protected.
func RemoveWGPeer(pubkey string) (*WGProbe, bool, error) {
	probe := ProbeWG()
	var target *WGPeer
	for i, p := range probe.Peers {
		if p.PublicKey == pubkey {
			target = &probe.Peers[i]
			break
		}
	}
	if target == nil {
		return probe, false, fmt.Errorf("peer not found")
	}
	if target.Admin {
		return probe, false, fmt.Errorf("admin peer cannot be removed (anti-lockout)")
	}
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return probe, false, err
	}
	ops := []executor.Op{
		{Kind: "uci_delete", Args: []string{"network." + target.Section}},
		{Kind: "uci_commit", Args: []string{"network"}},
		{Kind: "initd", Args: []string{"network", "reload"}},
	}
	rolledBack, err := wgApply(ops, snapNetwork, "", func() bool {
		for _, p := range wgPeers() {
			if p.PublicKey == pubkey {
				return false
			}
		}
		return true
	})
	return ProbeWG(), rolledBack, err
}

// AddWGPeerGenerated creates a peer with a server-side generated keypair
// and returns the ready client config (private key included) exactly once.
// The private key is never persisted by the panel; only the public key and
// the preshared key live in UCI.
func AddWGPeerGenerated(name string, admin bool, endpoint string) (string, *WGProbe, error) {
	privOut, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", nil, fmt.Errorf("wg genkey: %w", err)
	}
	priv := strings.TrimSpace(string(privOut))
	pub := wgPublicKey(priv)
	if pub == "" {
		return "", nil, fmt.Errorf("wg pubkey failed")
	}
	probe, _, err := AddWGPeer(name, pub, nil, admin)
	if err != nil {
		return "", probe, err
	}
	// Read back the peer data the module just wrote (address + psk).
	var address, psk string
	for _, p := range probe.Peers {
		if p.PublicKey == pub {
			if len(p.AllowedIPs) > 0 {
				address = p.AllowedIPs[0]
			}
			psk = uciGet("network." + p.Section + ".preshared_key")
		}
	}
	if endpoint == "" {
		endpoint = wanIPv4()
	}
	if endpoint == "" {
		endpoint = lanIPv4()
	}
	var b strings.Builder
	b.WriteString("[Interface]\n")
	b.WriteString("PrivateKey = " + priv + "\n")
	b.WriteString("Address = " + address + "\n")
	b.WriteString("DNS = " + strings.Split(probe.Address, "/")[0] + "\n")
	b.WriteString("\n[Peer]\n")
	b.WriteString("PublicKey = " + probe.PublicKey + "\n")
	if psk != "" {
		b.WriteString("PresharedKey = " + psk + "\n")
	}
	b.WriteString("AllowedIPs = 0.0.0.0/0\n")
	if endpoint != "" {
		b.WriteString("Endpoint = " + endpoint + ":" + probe.Port + "\n")
	}
	b.WriteString("PersistentKeepalive = 25\n")
	return b.String(), probe, nil
}
func nextWGAddress(probe *WGProbe) string {
	base := strings.Split(probe.Address, ".")
	if len(base) != 4 {
		return "10.66.0.2/32"
	}
	prefix := strings.Join(base[:3], ".")
	used := map[string]bool{}
	for _, p := range probe.Peers {
		for _, ip := range p.AllowedIPs {
			used[ip] = true
		}
	}
	for i := 2; i < 255; i++ {
		candidate := fmt.Sprintf("%s.%d/32", prefix, i)
		if !used[candidate] {
			return candidate
		}
	}
	return prefix + ".254/32"
}
