package modules

import (
	"strings"
	"testing"
)

// snmpFixture is a panel-managed config: named agent/system/view plus the
// ro/rw com2sec/group/access chains.
func snmpFixture() map[string]uciSection {
	return map[string]uciSection{
		snmpAgentSection: {Type: "agent", Options: map[string][]string{
			"agentaddress": {"UDP:161"},
		}},
		snmpSystemSection: {Type: "system", Options: map[string][]string{
			"sysLocation": {"Rack 1"}, "sysContact": {"admin@example.com"},
		}},
		snmpViewSection: {Type: "view", Options: map[string][]string{
			"viewname": {"all"}, "type": {"included"}, "oid": {".1"},
		}},
		snmpROCom2sec: {Type: "com2sec", Options: map[string][]string{
			"secname": {"ro"}, "source": {"default"}, "community": {"monitors"},
		}},
		snmpRWCom2sec: {Type: "com2sec", Options: map[string][]string{
			"secname": {"rw"}, "source": {"default"}, "community": {"ops"},
		}},
		snmpROGroup: {Type: "group"},
		snmpRWAccess: {Type: "access"},
	}
}

// snmpStockFixture mimics the 25.12 stock config: anonymous agent/system,
// com2sec chains with 'public'/'private', agentx, views and friends.
func snmpStockFixture() map[string]uciSection {
	return map[string]uciSection{
		"cfg01agent": {Type: "agent", Options: map[string][]string{
			"agentaddress": {"UDP:161,UDP6:161"},
		}},
		"cfg02system": {Type: "system", Options: map[string][]string{
			"sysLocation": {"office"}, "sysContact": {"bofh@example.com"},
		}},
		"agentx":   {Type: "agentx"},
		"public":   {Type: "com2sec", Options: map[string][]string{"secname": {"ro"}, "community": {"public"}}},
		"private":  {Type: "com2sec", Options: map[string][]string{"secname": {"rw"}, "community": {"private"}}},
		"public6":  {Type: "com2sec6"},
		"view_all": {Type: "view"},
		"pub_grp":  {Type: "group"},
		"pub_acc":  {Type: "access"},
		"general":  {Type: "snmpd"},
		"engineid": {Type: "engineid"},
		"exec":     {Type: "exec"},
	}
}

func TestSNMPConfigManagedAcceptsPanelShape(t *testing.T) {
	if !snmpdConfigManaged(snmpFixture()) {
		t.Error("panel-shaped config should read as managed")
	}
}

func TestSNMPConfigManagedAcceptsUntouchedStock(t *testing.T) {
	if !snmpdConfigManaged(snmpStockFixture()) {
		t.Error("untouched stock config should read as managed so the panel can replace it")
	}
}

func TestSNMPConfigManagedRejectsForeignSections(t *testing.T) {
	for _, foreign := range []string{"smux", "trap", "user", "proxy"} {
		sections := snmpFixture()
		sections[foreign] = uciSection{Type: foreign}
		if snmpdConfigManaged(sections) {
			t.Errorf("config with a %s section must report unmanaged: someone else owns it", foreign)
		}
	}
}

// The stock 'public' com2sec carries secname 'ro', so the probe-style
// prefill logic (com2sec secname -> community) picks it up.
func TestStockCommunityPrefillUsesSecname(t *testing.T) {
	ro, rw := "", ""
	for _, s := range snmpStockFixture() {
		if s.Type != "com2sec" {
			continue
		}
		switch s.Option("secname") {
		case "ro":
			ro = firstNonEmpty(ro, s.Option("community"))
		case "rw":
			rw = firstNonEmpty(rw, s.Option("community"))
		}
	}
	if ro != "public" || rw != "private" {
		t.Errorf("stock prefill ro=%q rw=%q, want public/private", ro, rw)
	}
}

func TestSNMPChainOpsBuildROChain(t *testing.T) {
	ops := snmpCommunityChainOps(nil, "ro", "c", "c6", "g", "a", "monitors", "none")
	want := map[string]bool{
		"snmpd.c": false, "snmpd.c6": false, "snmpd.g": false, "snmpd.a": false,
	}
	for _, op := range ops {
		if op.Kind != "uci_set" {
			t.Fatalf("unexpected op kind %q", op.Kind)
		}
		key := op.Args[0]
		if _, ok := want[key]; ok {
			want[key] = true
		}
		if strings.Contains(key, ".write") && op.Args[1] != "none" {
			t.Errorf("ro chain write view = %q, want none", op.Args[1])
		}
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("chain op for %s missing", key)
		}
	}
}
