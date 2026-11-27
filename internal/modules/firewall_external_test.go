package modules

import (
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

// Real-shaped output for a box where the factory rules and a couple of rules
// created elsewhere live next to NetGrip's own. The factory ones are
// anonymous sections using zones, family and ICMP options: exactly the shape
// that must never be opened in the editor.
const fwExternalFixture = `firewall.@rule[0]=rule
firewall.@rule[0].name='Allow-DHCP-Renew'
firewall.@rule[0].src='wan'
firewall.@rule[0].proto='udp'
firewall.@rule[0].dest_port='68'
firewall.@rule[0].target='ACCEPT'
firewall.@rule[0].family='ipv4'
firewall.@rule[1]=rule
firewall.@rule[1].name='Allow-Ping'
firewall.@rule[1].src='wan'
firewall.@rule[1].proto='icmp'
firewall.@rule[1].icmp_type='echo-request'
firewall.@rule[1].family='ipv4'
firewall.@rule[1].target='ACCEPT'
firewall.@rule[2]=rule
firewall.@rule[2].name='GL-WebUI PBS'
firewall.@rule[2].src='wan'
firewall.@rule[2].dest_port='8007'
firewall.@rule[2].target='ACCEPT'
firewall.impresora=rule
firewall.impresora.name='Impresora taller'
firewall.impresora.src='lan'
firewall.impresora.proto='tcp'
firewall.impresora.dest_ip='192.168.1.70'
firewall.impresora.dest_port='9100'
firewall.impresora.target='DROP'
firewall.impresora.limit='1000/sec'
firewall.netgrip_fw_1=rule
firewall.netgrip_fw_1.name='Block TV'
firewall.netgrip_fw_1.proto='udp'
firewall.netgrip_fw_1.src_ip='192.168.1.51'
firewall.netgrip_fw_1.target='REJECT'
`

func fwRules() []FWRule {
	return applyStockRules(parseFWRulesFrom(fwExternalFixture))
}

func TestParseFWRulesExtraOptions(t *testing.T) {
	rules := parseFWRulesFrom(fwExternalFixture)
	bySection := map[string]FWRule{}
	for _, r := range rules {
		bySection[r.Section] = r
	}

	cases := []struct {
		section string
		want    []FWOption
	}{
		{"@rule[0]", []FWOption{{Name: "family", Value: "ipv4"}, {Name: "src", Value: "wan"}}},
		{"@rule[1]", []FWOption{
			{Name: "family", Value: "ipv4"},
			{Name: "icmp_type", Value: "echo-request"},
			{Name: "src", Value: "wan"},
		}},
		{"impresora", []FWOption{{Name: "limit", Value: "1000/sec"}, {Name: "src", Value: "lan"}}},
		{"netgrip_fw_1", nil},
	}
	for _, c := range cases {
		got := bySection[c.section]
		if len(got.ExtraOptions) != len(c.want) {
			t.Errorf("%s: extra_options = %+v, want %+v", c.section, got.ExtraOptions, c.want)
			continue
		}
		for i := range c.want {
			if got.ExtraOptions[i] != c.want[i] {
				t.Errorf("%s: extra_options[%d] = %+v, want %+v", c.section, i, got.ExtraOptions[i], c.want[i])
			}
		}
	}
}

func TestApplyStockRulesMarksFactoryAndEditability(t *testing.T) {
	rows := []struct {
		section  string
		stock    bool
		editable bool
	}{
		// Bootstrap allowance: listed, never edited.
		{"@rule[0]", true, false},
		// Not a bootstrap name, but the form cannot express "proto icmp".
		{"@rule[1]", false, false},
		// The regression this test pins: a rule that ships in the factory
		// config but is not a bootstrap allowance stays editable. On a real
		// image the whole firewall lives in /rom, so keying off /rom hid the
		// editor (#778).
		{"@rule[2]", false, true},
		// Created elsewhere, address based: editable, not deletable.
		{"impresora", false, true},
		// NetGrip's own: editable as before.
		{"netgrip_fw_1", false, true},
	}
	got := map[string]FWRule{}
	for _, r := range fwRules() {
		got[r.Section] = r
	}
	for _, c := range rows {
		r, ok := got[c.section]
		if !ok {
			t.Fatalf("%s no aparece en el probe", c.section)
		}
		if r.Stock != c.stock {
			t.Errorf("%s: stock = %v, want %v", c.section, r.Stock, c.stock)
		}
		if r.Editable != c.editable {
			t.Errorf("%s: editable = %v, want %v", c.section, r.Editable, c.editable)
		}
	}
	if got["impresora"].Managed || got["@rule[0]"].Managed {
		t.Error("solo las secciones netgrip_fw_N son managed")
	}
}

// A rule whose action the form cannot express (NOTRACK) stays read-only
// instead of being rewritten with ACCEPT/DROP/REJECT on save.
func TestApplyStockRulesRefusesUnrepresentableAction(t *testing.T) {
	parsed := parseFWRulesFrom("firewall.x=rule\nfirewall.x.name='mangle'\nfirewall.x.target='NOTRACK'\n")
	rules := applyStockRules(parsed)
	if len(rules) != 1 || rules[0].Editable {
		t.Fatalf("una regla con target NOTRACK debe quedar en solo lectura: %+v", rules)
	}
}

func TestPlanFWRuleUpdateGate(t *testing.T) {
	rules := fwRules()
	edited := FirewallRuleAdd{Name: "Impresora taller", Proto: "tcp", DestIP: "192.168.1.70", DestPort: "9100", Target: "DROP"}

	cases := []struct {
		name    string
		section string
		wantErr string
	}{
		{"bootstrap DHCP", "@rule[0]", "firmware traffic rules are read-only"},
		{"proto que el formulario no representa", "@rule[1]", "this rule uses an action or protocol the panel cannot edit; change it in LuCI"},
		{"regla de la imagen que no es bootstrap", "@rule[2]", ""},
		{"externa no stock", "impresora", ""},
		{"propia de NetGrip", "netgrip_fw_1", ""},
		{"seccion inexistente", "no-existe", "rule not found"},
	}
	for _, c := range cases {
		_, err := planFWRuleUpdate(rules, c.section, edited)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: no deberia fallar: %v", c.name, err)
			}
			continue
		}
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.wantErr)
		}
	}
}

// Delete is no longer limited to NetGrip's own rules: anything the probe lists
// can go except the bootstrap allowances. The gate runs before any snapshot,
// so it can be exercised without a router.
func TestPlanFWRuleDelete(t *testing.T) {
	rules := fwRules()
	cases := []struct {
		name    string
		section string
		wantErr string
	}{
		{"propia de NetGrip", "netgrip_fw_1", ""},
		{"creada fuera", "impresora", ""},
		{"regla de la imagen que no es bootstrap", "@rule[2]", ""},
		{"proto no representable, pero borrable", "@rule[1]", ""},
		{"bootstrap DHCP", "@rule[0]", "bootstrap traffic rules are read-only"},
		{"seccion inexistente", "no-existe", "rule not found"},
	}
	for _, c := range cases {
		err := planFWRuleDelete(rules, c.section)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: no deberia fallar: %v", c.name, err)
			}
			continue
		}
		if err == nil || err.Error() != c.wantErr {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.wantErr)
		}
	}
}

// La edicion de una regla externa solo escribe sus 11 opciones modeladas:
// nada de borrar el src/dest de zona, la familia, la MAC, el ipset o el
// limite que el panel no entiende.
func TestUpdateExternalRuleKeepsUnmodeledOptions(t *testing.T) {
	var current FWRule
	for _, r := range fwRules() {
		if r.Section == "impresora" {
			current = r
		}
	}
	rule := FirewallRuleAdd{Name: "Impresora taller", Proto: "tcp", DestIP: "192.168.1.71", DestPort: "631", Target: "ACCEPT"}
	ops, err := planFWRuleUpdate([]FWRule{current}, "impresora", rule)
	if err != nil {
		t.Fatalf("una regla externa debe poder editarse: %v", err)
	}

	modeled := map[string]bool{
		"name": true, "proto": true, "src_ip": true, "dest_ip": true,
		"src_port": true, "dest_port": true, "target": true,
		"start_time": true, "stop_time": true, "weekdays": true, "utc_time": true,
	}
	var set, deleted []string
	for _, op := range ops {
		switch op.Kind {
		case "uci_set":
			opt := lastOption(op.Args[0])
			if !modeled[opt] {
				t.Errorf("uci_set sobre una opcion no modelada: %s = %s", op.Args[0], op.Args[1])
			}
			set = append(set, opt)
		case "uci_delete":
			opt := lastOption(op.Args[0])
			if !modeled[opt] {
				t.Errorf("uci_delete sobre una opcion no modelada: %s", op.Args[0])
			}
			deleted = append(deleted, opt)
		}
	}
	if strings.Join(set, ",") != "dest_ip,dest_port,target" {
		t.Errorf("opciones escritas = %v, want solo dest_ip, dest_port, target", set)
	}
	// proto no cambia (tcp -> tcp) y el src de zona nunca se toca.
	if len(deleted) != 0 {
		t.Errorf("no se vacia ninguna opcion modelada, borradas = %v", deleted)
	}
	want := map[string]string{
		"dest_ip":   "192.168.1.71",
		"dest_port": "631",
		"target":    "ACCEPT",
	}
	for _, op := range ops {
		if op.Kind != "uci_set" {
			continue
		}
		opt := lastOption(op.Args[0])
		if want[opt] != op.Args[1] {
			t.Errorf("%s = %q, want %q", opt, op.Args[1], want[opt])
		}
	}
	if !containsOp(ops, "uci_commit", "firewall") || !containsOp(ops, "initd", "firewall") {
		t.Errorf("la tanda debe terminar en commit + reload de firewall: %+v", ops)
	}
}

// A bootstrap rule is refused by name even though the section is anonymous.
func TestUpdateStockRuleRejectedByGate(t *testing.T) {
	rules := fwRules()
	_, err := planFWRuleUpdate(rules, "@rule[0]", FirewallRuleAdd{Name: "Allow-DHCP-Renew", Proto: "udp", Target: "ACCEPT"})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("err = %v, want rechazo de regla de fabrica", err)
	}
}

func lastOption(arg string) string {
	if i := strings.LastIndex(arg, "."); i >= 0 {
		return arg[i+1:]
	}
	return arg
}

func containsOp(ops []executor.Op, kind, arg string) bool {
	for _, op := range ops {
		if op.Kind == kind && len(op.Args) > 0 && op.Args[0] == arg {
			return true
		}
	}
	return false
}
