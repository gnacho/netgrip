package modules

import (
	"fmt"
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

// SetVPNPublicHost persists (or clears, when empty) the hostname used as
// the remote endpoint of generated client configs.
func SetVPNPublicHost(host string) error {
	host = strings.TrimSpace(host)
	if host != "" && !validPublicHostRe.MatchString(host) {
		return fmt.Errorf("invalid host")
	}
	ops := []executor.Op{{Kind: "uci_set", Args: []string{"netgrip.vpn=vpn"}}}
	if host == "" {
		if uciGet(vpnPublicHostUCI) != "" {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{vpnPublicHostUCI}})
		}
	} else {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{vpnPublicHostUCI + "=" + host}})
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
