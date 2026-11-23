package modules

import "testing"

// Cambiar una regla y dejar un campo vacio tiene que BORRAR la opcion: si no,
// seguiria aplicandose la fuente o el horario anterior (#625).
func TestTrafficRuleUpdateOpsDeletesClearedOptions(t *testing.T) {
	current := FWRule{
		Section: "netgrip_rule_1", Name: "cortar", Target: "DROP",
		SrcIP: "192.168.1.0/24", StartTime: "22:00", StopTime: "07:00", Weekdays: "Mon Tue",
	}
	rule := FirewallRuleAdd{Name: "cortar", Target: "DROP"}

	ops := trafficRuleUpdateOps("netgrip_rule_1", current, rule)
	deleted := map[string]bool{}
	set := map[string]string{}
	for _, op := range ops {
		switch op.Kind {
		case "uci_delete":
			deleted[op.Args[0]] = true
		case "uci_set":
			set[op.Args[0]] = op.Args[1]
		}
	}
	for _, opt := range []string{"src_ip", "start_time", "stop_time", "weekdays"} {
		if !deleted["firewall.netgrip_rule_1."+opt] {
			t.Errorf("%s deberia borrarse al vaciarse, ops=%+v", opt, ops)
		}
	}
	if len(set) != 0 {
		t.Errorf("no cambia ningun valor, no deberia escribir: %v", set)
	}
	if ops[len(ops)-2].Kind != "uci_commit" || ops[len(ops)-1].Kind != "initd" {
		t.Fatalf("la tanda debe terminar en commit + reload: %+v", ops)
	}
}

func TestTrafficRuleUpdateOpsWritesChanges(t *testing.T) {
	current := FWRule{Section: "netgrip_rule_2", Name: "vieja", Target: "ACCEPT"}
	rule := FirewallRuleAdd{Name: "nueva", Target: "REJECT", DestPort: "443", UTCTime: true}

	ops := trafficRuleUpdateOps("netgrip_rule_2", current, rule)
	set := map[string]string{}
	for _, op := range ops {
		if op.Kind == "uci_set" {
			set[op.Args[0]] = op.Args[1]
		}
	}
	want := map[string]string{
		"firewall.netgrip_rule_2.name":      "nueva",
		"firewall.netgrip_rule_2.target":    "REJECT",
		"firewall.netgrip_rule_2.dest_port": "443",
		"firewall.netgrip_rule_2.utc_time":  "1",
	}
	for k, v := range want {
		if set[k] != v {
			t.Errorf("%s = %q, want %q (ops=%+v)", k, set[k], v, ops)
		}
	}
	if len(set) != len(want) {
		t.Errorf("opciones escritas = %v, want solo %v", set, want)
	}
}

func TestTrafficRuleUpdateOpsNoChange(t *testing.T) {
	current := FWRule{Section: "netgrip_rule_3", Name: "igual", Target: "ACCEPT"}
	rule := FirewallRuleAdd{Name: "igual", Target: "ACCEPT"}
	ops := trafficRuleUpdateOps("netgrip_rule_3", current, rule)
	if len(ops) != 2 {
		t.Fatalf("sin cambios solo deberia quedar commit + reload, ops=%+v", ops)
	}
}
