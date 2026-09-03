package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

const (
	ovpnSection = "netgrip_server"
	ovpnPkiDir  = "/etc/easy-rsa/pki"
	ovpnPort    = "1194"
	ovpnSubnet  = "10.8.0.0 255.255.255.0"
	ovpnFwRule  = "allow_netgrip_ovpn"
)

// OVPNClient is one issued client certificate.
type OVPNClient struct {
	Name string `json:"name"`
}

// OVPNProbe is the read-only OpenVPN state.
type OVPNProbe struct {
	Installed bool         `json:"installed"`
	HasPKI    bool         `json:"has_pki"`
	Active    bool         `json:"active"`
	Running   bool         `json:"running"`
	Port      string       `json:"port"`
	Subnet    string       `json:"subnet"`
	Clients   []OVPNClient `json:"clients"`
}

func ovpnInstalled() bool {
	_, err := exec.LookPath("openvpn")
	return err == nil
}

// easyrsaExtraPaths are the standard easyrsa locations beyond PATH: the
// openvpn-easy-rsa package installs the binary under /usr/lib/easy-rsa with
// a /usr/bin symlink, but installs without the symlink must still work.
var easyrsaExtraPaths = []string{"/usr/lib/easy-rsa/easyrsa", "/usr/bin/easyrsa", "/usr/sbin/easyrsa"}

// easyrsaBin resolves the easyrsa executable: PATH first, then the standard
// locations ("" when not found).
func easyrsaBin() string {
	if p, err := exec.LookPath("easyrsa"); err == nil {
		return p
	}
	return resolveEasyrsaIn(easyrsaExtraPaths)
}

// resolveEasyrsaIn returns the first existing executable file among the
// candidates ("" when none qualifies).
func resolveEasyrsaIn(candidates []string) string {
	for _, c := range candidates {
		info, err := os.Stat(c)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		return c
	}
	return ""
}

// ovpnMissingPkgs returns the packages the enable flow must install. GL.iNet
// firmware ships openvpn itself, so the two are decided independently; its
// feeds carry no easy-rsa at all, which the caller turns into an actionable
// error.
var (
	ovpnInstalledF = ovpnInstalled
	easyrsaBinF    = easyrsaBin
)

func ovpnMissingPkgs() []string {
	var pkgs []string
	if !ovpnInstalledF() {
		pkgs = append(pkgs, "openvpn-openssl")
	}
	if easyrsaBinF() == "" {
		pkgs = append(pkgs, "openvpn-easy-rsa")
	}
	return pkgs
}

const errEasyrsaMissing = "easyrsa is required to build the certificates: install the openvpn-easy-rsa package (GL.iNet firmwares ship neither the tool nor the package in their feeds; enable the official OpenWrt feeds or copy /usr/lib/easy-rsa and /etc/easy-rsa from a same-release router) and retry"

func ovpnHasPKI() bool {
	for _, f := range []string{"ca.crt", "issued/server.crt", "private/server.key", "crl.pem"} {
		if _, err := os.Stat(filepath.Join(ovpnPkiDir, f)); err != nil {
			return false
		}
	}
	return true
}

func ovpnRunning() bool {
	out, err := exec.Command("pgrep", "-f", "openvpn.*"+ovpnSection).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// ProbeOVPN reads the OpenVPN state.
func ProbeOVPN() *OVPNProbe {
	p := &OVPNProbe{Installed: ovpnInstalled(), HasPKI: ovpnHasPKI(), Clients: []OVPNClient{}}
	base := "openvpn." + ovpnSection
	if uciSectionExists(base) {
		p.Port = uciGet(base + ".port")
		p.Subnet = uciGet(base + ".server")
		p.Active = uciGet(base+".enabled") == "1"
		p.Running = ovpnRunning()
	}
	if entries, err := os.ReadDir(filepath.Join(ovpnPkiDir, "issued")); err == nil {
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".crt")
			if name != "server" && !e.IsDir() {
				p.Clients = append(p.Clients, OVPNClient{Name: name})
			}
		}
	}
	return p
}

func easyrsa(args ...string) error {
	bin := easyrsaBin()
	if bin == "" {
		return fmt.Errorf("easyrsa %s: %s", strings.Join(args, " "), errEasyrsaMissing)
	}
	full := append([]string{"--batch"}, args...)
	cmd := exec.Command(bin, full...)
	cmd.Dir = "/etc/easy-rsa"
	cmd.Env = append(os.Environ(), "EASYRSA_PKI="+ovpnPkiDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("easyrsa %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureOVPNPKI() error {
	if ovpnHasPKI() {
		return nil
	}
	steps := [][]string{
		{"init-pki"},
		{"build-ca", "nopass"},
		{"gen-req", "server", "nopass"},
		{"sign-req", "server", "server"},
		{"gen-crl"},
	}
	for _, step := range steps {
		if err := easyrsa(step...); err != nil {
			return err
		}
	}
	return nil
}

// SetOVPN enables or disables the OpenVPN server.
func SetOVPN(enable bool) (*OVPNProbe, bool, error) {
	snapOvpn := snapshotIfExists("openvpn")
	snapNetwork, err := executor.Snapshot("network")
	if err != nil {
		return nil, false, err
	}
	snapFirewall, err := executor.Snapshot("firewall")
	if err != nil {
		return nil, false, err
	}
	rollback := func() {
		if snapOvpn != "" {
			_ = executor.Restore("openvpn", snapOvpn)
		} else {
			_ = executor.Run(executor.Op{Kind: "uci_delete", Args: []string{"openvpn." + ovpnSection}})
			_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"openvpn"}})
		}
		_ = executor.Restore("network", snapNetwork)
		_ = executor.Restore("firewall", snapFirewall)
		ovpnStopOurs()
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	}

	if enable {
		return enableOVPN(rollback)
	}
	return disableOVPN(rollback, snapOvpn)
}

func snapshotIfExists(config string) string {
	if _, err := os.Stat("/etc/config/" + config); err != nil {
		return ""
	}
	snap, err := executor.Snapshot(config)
	if err != nil {
		return ""
	}
	return snap
}

func enableOVPN(rollback func()) (*OVPNProbe, bool, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	if pkgs := ovpnMissingPkgs(); len(pkgs) > 0 {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: pkgs})
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		// GL.iNet feeds have no easy-rsa: wrap the raw apk failure so the
		// user gets the actual remedy instead of "unable to select packages".
		if easyrsaBin() == "" {
			err = fmt.Errorf("%s (%w)", errEasyrsaMissing, err)
		}
		return ProbeOVPN(), true, err
	}
	if easyrsaBin() == "" {
		rollback()
		return ProbeOVPN(), true, fmt.Errorf("pki: %s", errEasyrsaMissing)
	}
	if err := ensureOVPNPKI(); err != nil {
		rollback()
		return ProbeOVPN(), true, fmt.Errorf("pki: %w", err)
	}

	ops = nil
	base := "openvpn." + ovpnSection
	set(base, "openvpn")
	set(base+".enabled", "1")
	set(base+".dev", "tun")
	set(base+".proto", "udp")
	set(base+".port", ovpnPort)
	set(base+".topology", "subnet")
	set(base+".server", ovpnSubnet)
	set(base+".ca", ovpnPkiDir+"/ca.crt")
	set(base+".cert", ovpnPkiDir+"/issued/server.crt")
	set(base+".key", ovpnPkiDir+"/private/server.key")
	// dh none: TLS-ECDHE gives PFS without DH params, and gen-dh takes
	// minutes on router CPUs.
	set(base+".dh", "none")
	set(base+".crl_verify", ovpnPkiDir+"/crl.pem")
	set(base+".keepalive", "10 120")
	set(base+".persist_key", "1")
	set(base+".persist_tun", "1")
	set(base+".verb", "3")
	if route := lanRoute(); route != "" {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{base + ".push", "route " + route}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"openvpn"}})

	// Firewall input rule + lan zone membership via a vpn0 interface over
	// tun0, only when fw4 runs (mirrors the WireGuard module).
	if executor.ServiceEnabled("firewall") {
		set("network.vpn0", "interface")
		set("network.vpn0.proto", "none")
		set("network.vpn0.device", "tun0")
		ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})
		src := "lan"
		if uciSectionExists("network.wan") {
			src = "wan"
		}
		set("firewall."+ovpnFwRule, "rule")
		set("firewall."+ovpnFwRule+".name", "Allow-netgrip-OpenVPN")
		set("firewall."+ovpnFwRule+".src", src)
		set("firewall."+ovpnFwRule+".dest_port", ovpnPort)
		set("firewall."+ovpnFwRule+".proto", "udp")
		set("firewall."+ovpnFwRule+".target", "ACCEPT")
		if zone := wgLanZoneSection(); zone != "" {
			member := false
			for _, n := range strings.Fields(uciGet("firewall." + zone + ".network")) {
				if n == "vpn0" {
					member = true
				}
			}
			if !member {
				ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"firewall." + zone + ".network", "vpn0"}})
			}
		}
		ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"firewall"}})
	}
	ops = append(ops,
		executor.Op{Kind: "initd", Args: []string{"openvpn", "enable"}},
		executor.Op{Kind: "initd", Args: []string{"openvpn", "start"}},
	)
	if executor.ServiceEnabled("firewall") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeOVPN(), true, err
	}
	for range 10 {
		if ovpnRunning() {
			return ProbeOVPN(), false, nil
		}
		time.Sleep(time.Second)
	}
	rollback()
	return ProbeOVPN(), true, fmt.Errorf("openvpn did not start, rolled back")
}

func disableOVPN(rollback func(), snapOvpn string) (*OVPNProbe, bool, error) {
	if !uciSectionExists("openvpn." + ovpnSection) {
		return ProbeOVPN(), false, nil
	}
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	set("openvpn."+ovpnSection+".enabled", "0")
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"openvpn"}})
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeOVPN(), true, err
	}
	// procd manages each section as an instance: after enabled=0, a reload
	// stops only the disabled/changed instances (killing the pid instead
	// would just make procd respawn it; verified on 25.12.5).
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"openvpn", "reload"}})
	for range 5 {
		if !ovpnRunning() {
			return ProbeOVPN(), false, nil
		}
		time.Sleep(time.Second)
	}
	rollback()
	return ProbeOVPN(), true, fmt.Errorf("openvpn did not stop, rolled back")
}

func ovpnStopOurs() {
	out, err := exec.Command("pgrep", "-f", "openvpn.*"+ovpnSection).Output()
	if err != nil {
		return
	}
	for _, pidStr := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			if proc, err := os.FindProcess(pid); err == nil {
				_ = proc.Kill()
			}
		}
	}
}

// lanRoute returns "192.168.1.0 255.255.255.0" style route for the lan.
func lanRoute() string {
	out, err := exec.Command("sh", "-c", "ubus call network.interface.lan status 2>/dev/null | grep -A2 'ipv4-address' | grep address | head -1 | cut -d'\"' -f4").Output()
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(out))
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ""
	}
	maskOut, _ := exec.Command("sh", "-c", "ubus call network.interface.lan status 2>/dev/null | grep -A2 'ipv4-address' | grep mask | head -1 | awk '{print $2}'").Output()
	mask := strings.TrimSpace(string(maskOut))
	if mask == "" {
		mask = "24"
	}
	parts[3] = "0"
	return strings.Join(parts, ".") + " " + prefixToMask(mask)
}

func prefixToMask(prefix string) string {
	n, err := strconv.Atoi(prefix)
	if err != nil || n < 0 || n > 32 {
		return "255.255.255.0"
	}
	var b [4]int
	for i := range b {
		if n >= 8 {
			b[i] = 255
			n -= 8
		} else if n > 0 {
			b[i] = 256 - (1 << (8 - n))
			n = 0
		}
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

// AddOVPNClient issues a client certificate and returns the ready .ovpn.
func AddOVPNClient(name, remote string) (string, *OVPNProbe, error) {
	probe := ProbeOVPN()
	if !probe.Active {
		return "", probe, fmt.Errorf("openvpn is not enabled")
	}
	if !validNameRe.MatchString(name) {
		return "", probe, fmt.Errorf("invalid client name")
	}
	if _, err := os.Stat(filepath.Join(ovpnPkiDir, "issued", name+".crt")); err == nil {
		return "", probe, fmt.Errorf("client already exists")
	}
	if err := easyrsa("gen-req", name, "nopass"); err != nil {
		return "", probe, err
	}
	if err := easyrsa("sign-req", "client", name); err != nil {
		return "", probe, err
	}
	if remote == "" {
		remote = wanIPv4()
	}
	if remote == "" {
		remote = lanIPv4()
	}
	config, err := buildClientOVPN(name, remote)
	if err != nil {
		return "", probe, err
	}
	return config, ProbeOVPN(), nil
}

// RemoveOVPNClient revokes a client certificate and regenerates the CRL.
func RemoveOVPNClient(name string) (*OVPNProbe, error) {
	probe := ProbeOVPN()
	if !validNameRe.MatchString(name) {
		return probe, fmt.Errorf("invalid client name")
	}
	if _, err := os.Stat(filepath.Join(ovpnPkiDir, "issued", name+".crt")); err != nil {
		return probe, fmt.Errorf("client not found")
	}
	if err := easyrsa("revoke", name); err != nil {
		return probe, err
	}
	if err := easyrsa("gen-crl"); err != nil {
		return probe, err
	}
	// OpenVPN reads the CRL on startup. Killing our instance makes procd
	// respawn it with the fresh CRL (verified: procd has respawn).
	ovpnStopOurs()
	return ProbeOVPN(), nil
}

func buildClientOVPN(name, remote string) (string, error) {
	read := func(parts ...string) (string, error) {
		data, err := os.ReadFile(filepath.Join(append([]string{ovpnPkiDir}, parts...)...))
		return string(data), err
	}
	ca, err := read("ca.crt")
	if err != nil {
		return "", err
	}
	cert, err := read("issued", name+".crt")
	if err != nil {
		return "", err
	}
	key, err := read("private", name+".key")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("client\ndev tun\nproto udp\nremote " + remote + " " + ovpnPort + "\n")
	b.WriteString("resolv-retry infinite\nnobind\npersist-key\npersist-tun\nverb 3\n")
	b.WriteString("<ca>\n" + ca + "</ca>\n")
	b.WriteString("<cert>\n" + cert + "</cert>\n")
	b.WriteString("<key>\n" + key + "</key>\n")
	return b.String(), nil
}

func wanIPv4() string {
	return ubusIPv4("wan")
}

func lanIPv4() string {
	if ip := ubusIPv4("lan"); ip != "" {
		return ip
	}
	// Dumb APs may get the lan address via DHCP: ubus shows it nowhere
	// static. Fall back to the first global IPv4 on the device.
	out, err := exec.Command("sh", "-c", "ip -4 -o addr show scope global | awk '{print $4}' | cut -d/ -f1 | head -1").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ubusIPv4(iface string) string {
	out, err := exec.Command("ubus", "call", "network.interface."+iface, "status").Output()
	if err != nil {
		return ""
	}
	return parseUbusIPv4(string(out))
}

// parseUbusIPv4 extracts the first IPv4 address from the top-level
// ipv4-address array of a `ubus call network.interface.<x> status`
// payload. Parsing the JSON replaces the old grep pipeline, which
// matched the "ipv4-address" key line itself and always returned
// empty, silently falling back to the first global address on the
// device (a guest bridge on multi-network gateways).
func parseUbusIPv4(out string) string {
	var st struct {
		IPv4Address []struct {
			Address string `json:"address"`
		} `json:"ipv4-address"`
	}
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&st); err != nil {
		return ""
	}
	for _, a := range st.IPv4Address {
		if a.Address != "" {
			return a.Address
		}
	}
	return ""
}
