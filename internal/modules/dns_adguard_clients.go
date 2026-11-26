package modules

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

// Clientes reales en AdGuard (#671). Con el handoff de #360, dnsmasq reenvía
// al listener local de AdGuard y todas las consultas llegan desde 127.0.0.1:
// las estadísticas ven un único cliente, el router. La solución que usa
// GL.iNet ("AdGuard Home Handle Client Requests") es redirigir el DNS de los
// clientes directamente a AdGuard con una regla de cortafuegos, sin que
// dnsmasq se entere. Efectos que la tarjeta cuenta: las políticas por dominio
// dejan de aplicarse y los nombres locales de dnsmasq hay que resolverlos en
// AdGuard.

const (
	adguardClientsRedirect = "netgrip_adguard_clients"
	adguardClientsAllow    = "netgrip_adguard_clients_allow"
)

// adguardClientsFirewallShow reads the firewall config once.
func adguardClientsFirewallShow() string {
	out, err := uciExec("show", "firewall")
	if err != nil {
		return ""
	}
	return string(out)
}

// adGuardClientsEnabled reports whether the redirect for this AdGuard port is
// present and enabled, which is what the card shows.
func adGuardClientsEnabled(show string, port int) bool {
	sections := parseUCIShow(show, "firewall")
	s, ok := sections[adguardClientsRedirect]
	if !ok {
		return false
	}
	if s.Type != "redirect" || s.Option("enabled") == "0" || s.Option("enabled") == "false" {
		return false
	}
	return s.Option("src_dport") == "53" && s.Option("dest_port") == strconv.Itoa(port) && s.Option("target") == "DNAT"
}

// adGuardClientsOps builds the firewall writes. Pure: same inputs, same ops.
// The accept rule exists because the DNAT lands on the router itself (INPUT)
// and a hardened input policy would drop it without a word.
func adGuardClientsOps(enable bool, port int, lanIP string, hasRedirect, hasAllow bool) []executor.Op {
	var ops []executor.Op
	if !enable {
		if hasRedirect {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"firewall." + adguardClientsRedirect}})
		}
		if hasAllow {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"firewall." + adguardClientsAllow}})
		}
		if len(ops) > 0 {
			ops = append(ops,
				executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
				executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
			)
		}
		return ops
	}
	set := func(key, val string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"firewall." + adguardClientsRedirect + "." + key, val}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_set", Args: []string{"firewall." + adguardClientsRedirect, "redirect"}},
	)
	set("name", "netgrip AdGuard client DNS")
	set("src", "lan")
	set("proto", "tcp udp")
	set("src_dport", "53")
	set("dest", "lan")
	set("dest_ip", lanIP)
	set("dest_port", strconv.Itoa(port))
	set("target", "DNAT")
	set("enabled", "1")

	setAllow := func(key, val string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"firewall." + adguardClientsAllow + "." + key, val}})
	}
	ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"firewall." + adguardClientsAllow, "rule"}})
	setAllow("name", "netgrip AdGuard client DNS in")
	setAllow("src", "lan")
	setAllow("proto", "tcp udp")
	setAllow("dest_port", strconv.Itoa(port))
	setAllow("target", "ACCEPT")

	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
	)
	return ops
}

// adGuardRedirectLive checks the loaded ruleset for the DNAT. The UCI section
// can be perfect and fw4 still refuse to load it, so the promise "it works"
// needs a look at what the kernel has.
func adGuardRedirectLive(port int) bool {
	out, err := exec.Command("nft", "list", "ruleset").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "dnat") {
			continue
		}
		if strings.Contains(line, ":"+strconv.Itoa(port)) {
			return true
		}
	}
	return false
}

// AdGuardClientRequests turns the client redirect on or off. The handler arms
// the confirm window; here we snapshot, apply, verify and roll back.
func AdGuardClientRequests(enable bool) (*DNSConfig, bool, error) {
	if !dnsApplicable() {
		return ProbeDNS(), false, fmt.Errorf("DNS settings only apply on the gateway (dnsmasq)")
	}
	if !pkgInstalled("adguardhome") {
		return ProbeDNS(), false, fmt.Errorf("adguardhome is not installed")
	}
	if !adGuardProtected(dnsmasqStateNow()) {
		return ProbeDNS(), false, fmt.Errorf("DNS protection must be active first: AdGuard has to be the resolver dnsmasq forwards to")
	}
	port := adGuardResolvedPort()
	if port <= 0 || port == 53 {
		// A redirect to :53 would point dnsmasq at itself.
		return ProbeDNS(), false, fmt.Errorf("AdGuard does not listen on a separate port (found %d): a redirect would loop", port)
	}
	if enable && adGuardClientsEnabled(adguardClientsFirewallShow(), port) {
		return ProbeDNS(), false, fmt.Errorf("client requests are already handled by AdGuard")
	}
	lanIP := lanIPv4()
	if enable && lanIP == "" {
		return ProbeDNS(), false, fmt.Errorf("cannot resolve the LAN address")
	}

	show := adguardClientsFirewallShow()
	sections := parseUCIShow(show, "firewall")
	_, hasRedirect := sections[adguardClientsRedirect]
	_, hasAllow := sections[adguardClientsAllow]
	ops := adGuardClientsOps(enable, port, lanIP, hasRedirect, hasAllow)
	if len(ops) == 0 {
		return ProbeDNS(), false, fmt.Errorf("nothing to change")
	}

	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return ProbeDNS(), false, fmt.Errorf("snapshot firewall: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeDNS(), true, err
	}
	if enable && !adGuardRedirectLive(port) {
		rollback()
		return ProbeDNS(), true, fmt.Errorf("firewall reloaded but the redirect is not in the ruleset, restored the previous firewall")
	}
	if !enable && adGuardClientsEnabled(adguardClientsFirewallShow(), port) {
		rollback()
		return ProbeDNS(), true, fmt.Errorf("the redirect is still enabled, restored the previous firewall")
	}
	return ProbeDNS(), false, nil
}
