package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

// Forma real de la config de firewall de un router de la flota (26-Nov):
// zonas con input ACCEPT en lan, reglas y redirects con nombre.
const fwClientsFixture = `firewall.@zone[0]=zone
firewall.@zone[0].name='lan'
firewall.@zone[0].network='lan'
firewall.@zone[0].input='ACCEPT'
firewall.@zone[1]=zone
firewall.@zone[1].name='wan'
firewall.@zone[1].input='DROP'
firewall.allow_netgrip_wg=rule
firewall.allow_netgrip_wg.name='Allow-netgrip-WireGuard'
firewall.allow_netgrip_wg.src='wan'
firewall.allow_netgrip_wg.proto='udp'
firewall.allow_netgrip_wg.target='ACCEPT'
`

const fwClientsEnabledFixture = fwClientsFixture + `firewall.netgrip_adguard_clients=redirect
firewall.netgrip_adguard_clients.name='netgrip AdGuard client DNS'
firewall.netgrip_adguard_clients.src='lan'
firewall.netgrip_adguard_clients.proto='tcp udp'
firewall.netgrip_adguard_clients.src_dport='53'
firewall.netgrip_adguard_clients.dest='lan'
firewall.netgrip_adguard_clients.dest_ip='192.168.10.1'
firewall.netgrip_adguard_clients.dest_port='3053'
firewall.netgrip_adguard_clients.target='DNAT'
firewall.netgrip_adguard_clients.enabled='1'
firewall.netgrip_adguard_clients_allow=rule
firewall.netgrip_adguard_clients_allow.name='netgrip AdGuard client DNS in'
firewall.netgrip_adguard_clients_allow.src='lan'
firewall.netgrip_adguard_clients_allow.proto='tcp udp'
firewall.netgrip_adguard_clients_allow.dest_port='3053'
firewall.netgrip_adguard_clients_allow.target='ACCEPT'
`

// opsText renders ops for assertions. Named apart from the portrole test
// helper: both files land on main and a duplicate would not compile.
func opsText(ops []executor.Op) string {
	var b strings.Builder
	for _, op := range ops {
		b.WriteString(op.Kind)
		for _, a := range op.Args {
			b.WriteString(" " + a)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestAdGuardClientsProbe(t *testing.T) {
	if adGuardClientsEnabled(fwClientsFixture, 3053) {
		t.Fatal("an absent redirect must read as disabled")
	}
	if !adGuardClientsEnabled(fwClientsEnabledFixture, 3053) {
		t.Fatal("the real enabled shape must read as enabled")
	}
	if adGuardClientsEnabled(fwClientsEnabledFixture, 53) {
		t.Fatal("a redirect to another port is not this integration")
	}
	off := strings.Replace(fwClientsEnabledFixture, "firewall.netgrip_adguard_clients.enabled='1'", "firewall.netgrip_adguard_clients.enabled='0'", 1)
	if adGuardClientsEnabled(off, 3053) {
		t.Fatal("enabled=0 must read as disabled")
	}
}

func TestAdGuardClientsOpsEnable(t *testing.T) {
	ops := adGuardClientsOps(true, 3053, "192.168.10.1", false, false)
	got := opsText(ops)
	for _, want := range []string{
		"uci_set firewall.netgrip_adguard_clients redirect\n",
		"uci_set firewall.netgrip_adguard_clients.src lan\n",
		"uci_set firewall.netgrip_adguard_clients.src_dport 53\n",
		"uci_set firewall.netgrip_adguard_clients.dest_ip 192.168.10.1\n",
		"uci_set firewall.netgrip_adguard_clients.dest_port 3053\n",
		"uci_set firewall.netgrip_adguard_clients.target DNAT\n",
		"uci_set firewall.netgrip_adguard_clients_allow rule\n",
		"uci_set firewall.netgrip_adguard_clients_allow.dest_port 3053\n",
		"uci_commit firewall\n",
		"initd firewall reload\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ops missing %q in:\n%s", want, got)
		}
	}
	for _, op := range ops {
		if err := executor.Validate(op); err != nil {
			t.Fatalf("generated op fails the executor allowlist: %v", err)
		}
	}
}

func TestAdGuardClientsOpsDisable(t *testing.T) {
	ops := adGuardClientsOps(false, 3053, "", true, true)
	got := opsText(ops)
	if !strings.Contains(got, "uci_delete firewall.netgrip_adguard_clients\n") ||
		!strings.Contains(got, "uci_delete firewall.netgrip_adguard_clients_allow\n") ||
		!strings.Contains(got, "initd firewall reload\n") {
		t.Fatalf("disable must remove both sections and reload:\n%s", got)
	}
	for _, op := range ops {
		if err := executor.Validate(op); err != nil {
			t.Fatalf("generated op fails the executor allowlist: %v", err)
		}
	}
	// Sin secciones no hay nada que borrar ni que recargar.
	if len(adGuardClientsOps(false, 3053, "", false, false)) != 0 {
		t.Fatal("disable with nothing to remove must return no ops")
	}
}
