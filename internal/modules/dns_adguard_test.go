package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

func TestParseDnsmasqShowStock(t *testing.T) {
	out := `dhcp.@dnsmasq[0]=dnsmasq
dhcp.@dnsmasq[0].domainneeded='1'
dhcp.@dnsmasq[0].boguspriv='1'
dhcp.@dnsmasq[0].resolvfile='/tmp/resolv.conf.d/resolv.conf.auto'
dhcp.@dnsmasq[0].localservice='1'`
	st := parseDnsmasqShow(out)
	if len(st.servers) != 0 {
		t.Errorf("stock config must have no servers, got %v", st.servers)
	}
	if st.noresolvSet {
		t.Error("stock config must not have noresolv set")
	}
}

func TestParseDnsmasqShowLists(t *testing.T) {
	// `uci show dhcp.@dnsmasq[0]` resolves the anonymous section to its
	// cfgXXXXXX name in every emitted line; the parser must accept that
	// real shape, not only the @dnsmasq[0] spelling.
	out := `dhcp.cfg01411c=dnsmasq
dhcp.cfg01411c.server='1.1.1.1' '9.9.9.9'
dhcp.cfg01411c.noresolv='1'
dhcp.cfg01411c.cachesize='150'`
	st := parseDnsmasqShow(out)
	if len(st.servers) != 2 || st.servers[0] != "1.1.1.1" || st.servers[1] != "9.9.9.9" {
		t.Errorf("unexpected servers %v", st.servers)
	}
	if !st.noresolvSet || st.noresolv != "1" {
		t.Errorf("noresolv not parsed: %+v", st)
	}
}

func TestParseDnsmasqShowProtected(t *testing.T) {
	out := `dhcp.cfg01411c.server='127.0.0.1#5353'
dhcp.cfg01411c.noresolv='1'`
	st := parseDnsmasqShow(out)
	if !adGuardProtected(st) {
		t.Error("127.0.0.1#5353 forwarder must count as protection")
	}
	if got := adGuardForwarders(st.servers); len(got) != 1 || got[0] != "127.0.0.1#5353" {
		t.Errorf("unexpected forwarders %v", got)
	}
}

func TestAdGuardProtectedRejectsPlainLoopback(t *testing.T) {
	// A bare 127.0.0.1 (no port) is a dnsmasq upstream shortcut, not an
	// AdGuard handoff.
	st := dnsmasqState{servers: []string{"127.0.0.1", "1.1.1.1"}}
	if adGuardProtected(st) {
		t.Error("bare 127.0.0.1 must not count as AdGuard protection")
	}
}

func TestParseAdGuardDNSPort(t *testing.T) {
	yaml := `http:
  address: 0.0.0.0:3000
dns:
  bind_hosts:
    - 127.0.0.1
  port: 5353
  upstream_dns:
    - https://dns.quad9.net/dns-query
`
	if p := parseAdGuardDNSPort([]byte(yaml)); p != 5353 {
		t.Errorf("port = %d, want 5353", p)
	}
	// http.port must never be picked up as the DNS port.
	if p := parseAdGuardDNSPort([]byte("http:\n  port: 3000\n")); p != 0 {
		t.Errorf("http-only yaml must give 0, got %d", p)
	}
	if p := parseAdGuardDNSPort([]byte("dns:\n  port: abc\n")); p != 0 {
		t.Errorf("garbage port must give 0, got %d", p)
	}
	if p := parseAdGuardDNSPort(nil); p != 0 {
		t.Errorf("empty yaml must give 0, got %d", p)
	}
}

func TestAdGuardMinimalYAMLRoundTrip(t *testing.T) {
	p := parseAdGuardDNSPort([]byte(adGuardMinimalYAML(5353)))
	if p != 5353 {
		t.Errorf("minimal yaml port = %d, want 5353", p)
	}
}

func TestPlanServerListOpsReplacesExactly(t *testing.T) {
	ops := planServerListOps([]string{"1.1.1.1", "9.9.9.9"}, []string{"127.0.0.1#5353"})
	if len(ops) != 3 {
		t.Fatalf("want 3 ops, got %d: %v", len(ops), ops)
	}
	want := []executor.Op{
		{Kind: "uci_del_list", Args: []string{"dhcp.@dnsmasq[0].server", "1.1.1.1"}},
		{Kind: "uci_del_list", Args: []string{"dhcp.@dnsmasq[0].server", "9.9.9.9"}},
		{Kind: "uci_add_list", Args: []string{"dhcp.@dnsmasq[0].server", "127.0.0.1#5353"}},
	}
	for i, op := range ops {
		if op.Kind != want[i].Kind || strings.Join(op.Args, "|") != strings.Join(want[i].Args, "|") {
			t.Errorf("op %d = %v %v, want %v %v", i, op.Kind, op.Args, want[i].Kind, want[i].Args)
		}
		if err := executor.Validate(op); err != nil {
			t.Errorf("op %d fails validation: %v", i, err)
		}
	}
}

func TestPlanServerListOpsEmptyTarget(t *testing.T) {
	ops := planServerListOps([]string{"127.0.0.1#5353"}, nil)
	if len(ops) != 1 || ops[0].Kind != "uci_del_list" {
		t.Errorf("restore to stock must only delete the forwarder, got %v", ops)
	}
}

func TestPlanNoresolvOps(t *testing.T) {
	unset := planNoresolvOps(dnsmasqState{})
	if len(unset) != 1 || unset[0].Kind != "uci_delete" {
		t.Errorf("unset noresolv must delete the option, got %v", unset)
	}
	set := planNoresolvOps(dnsmasqState{noresolvSet: true, noresolv: "0"})
	if len(set) != 1 || set[0].Kind != "uci_set" || set[0].Args[1] != "0" {
		t.Errorf("set noresolv must restore the value, got %v", set)
	}
	for _, ops := range [][]executor.Op{unset, set} {
		for _, op := range ops {
			if err := executor.Validate(op); err != nil {
				t.Errorf("noresolv op fails validation: %v", err)
			}
		}
	}
}
