package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/owpanel/internal/executor"
)

const (
	wgIface        = "wg0"
	wgDefaultPort  = "51820"
	wgDefaultAddr  = "10.66.0.1/24"
	wgFirewallRule = "allow_owpanel_wg"
)

// WGPeer is one WireGuard peer as stored in UCI.
type WGPeer struct {
	Section    string   `json:"section"`
	Name       string   `json:"name"`
	PublicKey  string   `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips"`
	Admin      bool     `json:"admin"`
}

// WGProbe is the read-only WireGuard state.
type WGProbe struct {
	Installed bool     `json:"installed"`
	Active    bool     `json:"active"`
	Running   bool     `json:"running"`
	Port      string   `json:"port"`
	Address   string   `json:"address"`
	PublicKey string   `json:"public_key"`
	Peers     []WGPeer `json:"peers"`
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
	p.Peers = wgPeers()
	return p
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
			Admin:      uciGet(base+".owpanel_admin") == "1",
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

	// Firewall input rule: only when the firewall is actually running.
	// Dumb APs disable fw4 entirely; there the rule is pointless and
	// `firewall reload` fails hard (verified on a Redmi AX6 dumb AP).
	if executor.ServiceEnabled("firewall") {
		src := "lan"
		if uciSectionExists("network.wan") {
			src = "wan"
		}
		set("firewall."+wgFirewallRule, "rule")
		set("firewall."+wgFirewallRule+".name", "Allow-owpanel-WireGuard")
		set("firewall."+wgFirewallRule+".src", src)
		set("firewall."+wgFirewallRule+".dest_port", uciGet("network."+wgIface+".listen_port"))
		set("firewall."+wgFirewallRule+".proto", "udp")
		set("firewall."+wgFirewallRule+".target", "ACCEPT")
		ops = append(ops,
			executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		)
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
	if uciSectionExists("firewall." + wgFirewallRule) {
		ops = append(ops,
			executor.Op{Kind: "uci_delete", Args: []string{"firewall." + wgFirewallRule}},
			executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		)
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
	}
	if name != "" {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".description", name}})
	}
	if admin {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".owpanel_admin", "1"}})
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

// nextWGAddress picks the first free /32 inside the server /24.
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
