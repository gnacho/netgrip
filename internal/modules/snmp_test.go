package modules

import "testing"

func snmpFixture() map[string]uciSection {
	return map[string]uciSection{
		"agent": {Type: "agent", Options: map[string][]string{
			"sysLocation":  {"Rack 1"},
			"sysContact":   {"admin@example.com"},
			"agentaddress": {"UDP:161"},
		}},
		"cfg0a1b2c": {Type: "community", Options: map[string][]string{
			"name": {"public"}, "source": {"default"}, "mode": {"ro"},
		}},
		snmpROSection: {Type: "community", Options: map[string][]string{
			"name": {"monitors"}, "source": {"default"}, "mode": {"ro"},
		}},
		snmpRWSection: {Type: "community", Options: map[string][]string{
			"name": {"ops"}, "source": {"default"}, "mode": {"rw"},
		}},
	}
}

func TestSNMPConfigManagedAcceptsStockShape(t *testing.T) {
	if !snmpdConfigManaged(snmpFixture()) {
		t.Error("stock-shaped config (agent + communities) should read as managed")
	}
}

func TestSNMPConfigManagedRejectsForeignSections(t *testing.T) {
	sections := snmpFixture()
	sections["smux"] = uciSection{Type: "smux", Options: map[string][]string{}}
	if snmpdConfigManaged(sections) {
		t.Error("config with a smux section must report unmanaged: someone else owns it")
	}
}

func TestStockCommunitiesFindsAnonymousLeftovers(t *testing.T) {
	got := stockCommunities(snmpFixture())
	if len(got) != 1 || got[0] != "cfg0a1b2c" {
		t.Errorf("stockCommunities = %v, want [cfg0a1b2c]", got)
	}
}

func TestStockCommunitiesIgnoresPanelSections(t *testing.T) {
	sections := snmpFixture()
	delete(sections, "cfg0a1b2c")
	if got := stockCommunities(sections); len(got) != 0 {
		t.Errorf("stockCommunities = %v, want empty once only panel sections remain", got)
	}
}
