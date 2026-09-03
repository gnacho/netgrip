package modules

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// A public hostname (DDNS or a static name). Deliberately permissive:
// the value only ends up as the remote line of generated client configs,
// where a wrong value fails to connect instead of breaking the router.
var validPublicHostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]{0,251}[a-zA-Z0-9])?$`)

const vpnPublicHostUCI = "netgrip.vpn.public_host"

// GetVPNPublicHost returns the stable public hostname configured for VPN
// client configs (UCI netgrip.vpn.public_host); empty when unset.
func GetVPNPublicHost() string {
	return uciGet(vpnPublicHostUCI)
}

// ensureVpnSection creates the named netgrip.vpn section on first use.
// The executor only accepts option writes (config.section.option=value),
// so section creation goes through the uci CLI directly, mirroring how
// access.go bootstraps netgrip.main.
func ensureVpnSection() error {
	if uciSectionExists("netgrip.vpn") {
		return nil
	}
	cmd := exec.Command("uci", "set", "netgrip.vpn=vpn")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create netgrip.vpn section: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// SetVPNPublicHost persists (or clears, when empty) the hostname used as
// the remote endpoint of generated client configs.
func SetVPNPublicHost(host string) error {
	host = strings.TrimSpace(host)
	if host != "" && !validPublicHostRe.MatchString(host) {
		return fmt.Errorf("invalid host")
	}
	var ops []executor.Op
	if host == "" {
		if uciGet(vpnPublicHostUCI) != "" {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{vpnPublicHostUCI}})
		}
	} else {
		if err := ensureVpnSection(); err != nil {
			return err
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{vpnPublicHostUCI, host}})
	}
	if len(ops) == 0 {
		return nil
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"netgrip"}})
	return executor.Apply(ops, nil)
}

// ddnsDomains lists the domains managed by the DDNS updaters configured
// on this router (/etc/config/ddns), so the UI can offer them as
// suggestions for the public host.
func ddnsDomains() []string {
	out := make([]string, 0)
	for _, e := range ProbeDDNS().Entries {
		if e.Domain != "" {
			out = append(out, e.Domain)
		}
	}
	return out
}

// ovpnRemoteEndpoint resolves the remote for generated client configs:
// an explicit request wins, then the configured public host (stable
// across WAN changes), then the current WAN address, then the lan
// address for on-network testing.
func ovpnRemoteEndpoint(explicit, publicHost, wan, lan string) string {
	switch {
	case explicit != "":
		return explicit
	case publicHost != "":
		return publicHost
	case wan != "":
		return wan
	default:
		return lan
	}
}
