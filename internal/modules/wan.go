package modules

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

// WANConfig is the editable WAN configuration: whichever interface
// ubus.ActiveWANInterfaceName resolves as the active uplink, and its device.
type WANConfig struct {
	// Iface names the interface these settings belong to. With more than
	// one uplink, which one that is changes with the active connection, and
	// a form that silently follows it is a form that edits the wrong link.
	// Read-only: the server decides, the panel shows it.
	Iface    string `json:"iface,omitempty"`
	Proto    string `json:"proto"` // dhcp | static | pppoe
	Device   string `json:"device,omitempty"`
	IPAddr   string `json:"ipaddr,omitempty"`
	Netmask  string `json:"netmask,omitempty"`
	Gateway  string `json:"gateway,omitempty"`
	DNS      string `json:"dns,omitempty"`
	MTU      string `json:"mtu,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	VlanID   string `json:"vlanid,omitempty"`
}

func wanUCI(key string) string {
	out, err := exec.Command("uci", "-q", "get", "network."+ubus.ActiveWANInterfaceName()+"."+key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadWANConfig reads the current network.wan configuration (password is NOT
// returned; a non-empty Password sent back means 'keep the stored one').
func ReadWANConfig() WANConfig {
	return WANConfig{
		Iface:    ubus.ActiveWANInterfaceName(),
		Proto:    wanUCI("proto"),
		Device:   wanUCI("device"),
		IPAddr:   wanUCI("ipaddr"),
		Netmask:  wanUCI("netmask"),
		Gateway:  wanUCI("gateway"),
		DNS:      wanUCI("dns"),
		MTU:      wanUCI("mtu"),
		Username: wanUCI("username"),
		VlanID:   wanUCI("vlanid"),
	}
}

// ApplyWANConfig applies WAN settings with a snapshot/rollback so a bad
// config does not leave the router without connectivity.
func ApplyWANConfig(cfg WANConfig) (WANConfig, error) {
	if cfg.Proto != "dhcp" && cfg.Proto != "static" && cfg.Proto != "pppoe" {
		return ReadWANConfig(), fmt.Errorf("invalid proto %q", cfg.Proto)
	}
	snap, err := executor.Snapshot("network")
	if err != nil {
		return ReadWANConfig(), err
	}
	rollback := func() {
		_ = executor.Restore("network", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	}
	iface := ubus.ActiveWANInterfaceName()
	cfg.Iface = iface
	ops := []executor.Op{{Kind: "uci_set", Args: []string{"network." + iface + ".proto", cfg.Proto}}}
	setIf := func(key, val string) {
		if val == "" {
			return
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network." + iface + "." + key, val}})
	}
	setIf("device", cfg.Device)
	setIf("ipaddr", cfg.IPAddr)
	setIf("netmask", cfg.Netmask)
	setIf("gateway", cfg.Gateway)
	setIf("dns", cfg.DNS)
	setIf("mtu", cfg.MTU)
	setIf("username", cfg.Username)
	setIf("vlanid", cfg.VlanID)
	if cfg.Password != "" {
		setIf("password", cfg.Password)
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"network"}},
		executor.Op{Kind: "initd", Args: []string{"network", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ReadWANConfig(), err
	}
	return ReadWANConfig(), nil
}

// ---------------------------------------------------------------------------
// The address the internet sees
// ---------------------------------------------------------------------------

// PublicIP is what an outside service reports this router as coming from.
// It is not always the address on the uplink: a provider may hand out one
// of its own and put the customer behind its NAT, which is why a phone on
// the same connection shows a different number to any "what is my IP" site.
type PublicIP struct {
	IP string `json:"ip"`
	// Iface: the uplink it was measured through, so the answer is not
	// mistaken for the other connection's after a failover.
	Iface string `json:"iface"`
	// Source: who was asked, named rather than hidden.
	Source    string `json:"source"`
	CheckedAt int64  `json:"checked_at"`
}

// publicIPServices are asked in order until one answers with an address.
// Both return the address as plain text and nothing else.
var publicIPServices = []string{
	"https://api.ipify.org",
	"https://checkip.amazonaws.com",
}

// CheckPublicIP asks an outside service what address this router appears to
// come from.
//
// This is the only place netgrip talks to anything beyond the user's own
// network, so it happens on request and never on a poll: the panel wires it
// to a button, not to a refresh. The request follows the active uplink like
// any other traffic from the router, which is what makes the answer belong
// to the connection currently in use.
func CheckPublicIP() (*PublicIP, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	var lastErr error
	for _, service := range publicIPServices {
		req, err := http.NewRequest(http.MethodGet, service, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s: %s", service, resp.Status)
			continue
		}
		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) == nil {
			lastErr = fmt.Errorf("%s answered something that is not an address", service)
			continue
		}
		host := service
		if u, err := url.Parse(service); err == nil {
			host = u.Host
		}
		return &PublicIP{
			IP:        ip,
			Iface:     ubus.ActiveWANInterfaceName(),
			Source:    host,
			CheckedAt: time.Now().Unix(),
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no address service answered")
	}
	return nil, lastErr
}
