package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// WANConfig is the editable WAN (network.wan + its device) configuration.
type WANConfig struct {
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
	out, err := exec.Command("uci", "-q", "get", "network.wan."+key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadWANConfig reads the current network.wan configuration (password is NOT
// returned; a non-empty Password sent back means 'keep the stored one').
func ReadWANConfig() WANConfig {
	return WANConfig{
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
	ops := []executor.Op{{Kind: "uci_set", Args: []string{"network.wan.proto", cfg.Proto}}}
	setIf := func(key, val string) {
		if val == "" {
			return
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"network.wan." + key, val}})
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
