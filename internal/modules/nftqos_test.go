package modules

import (
	"strings"
	"testing"
)

func TestGenerateNftRulesetNoLimits(t *testing.T) {
	rules := generateNftRuleset(map[string]NftQoSLimit{})
	if !strings.Contains(rules, "delete table inet netgrip_qos") {
		t.Fatalf("expected delete table, got:\n%s", rules)
	}
}

func TestGenerateNftRulesetWithLimits(t *testing.T) {
	limits := map[string]NftQoSLimit{
		"aa:bb:cc:dd:ee:ff": {MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Download: 10, Upload: 5},
		"11:22:33:44:55:66": {MAC: "11:22:33:44:55:66", IP: "192.168.1.101", Download: 0, Upload: 2},
	}
	rules := generateNftRuleset(limits)
	if !strings.Contains(rules, "table inet netgrip_qos") {
		t.Fatal("missing table declaration")
	}
	if !strings.Contains(rules, "ip saddr 192.168.1.100 limit rate over 625 kbytes/second drop") {
		t.Fatalf("missing upload rule for 192.168.1.100, got:\n%s", rules)
	}
	if !strings.Contains(rules, "ip daddr 192.168.1.100 limit rate over 1250 kbytes/second drop") {
		t.Fatalf("missing download rule for 192.168.1.100, got:\n%s", rules)
	}
	if !strings.Contains(rules, "ip saddr 192.168.1.101 limit rate over 250 kbytes/second drop") {
		t.Fatalf("missing upload rule for 192.168.1.101, got:\n%s", rules)
	}
	if strings.Contains(rules, "ip daddr 192.168.1.101") {
		t.Fatalf("192.168.1.101 should not have a download rule, got:\n%s", rules)
	}
}

func TestSaveAndLoadNftQoSLimits(t *testing.T) {
	dir := t.TempDir()
	oldFile := nftQoSConfigFile
	oldDir := nftQoSConfigDir
	oldRules := nftQoSRulesFile
	nftQoSConfigDir = dir
	nftQoSConfigFile = dir + "/qos_limits.json"
	nftQoSRulesFile = dir + "/qos_limits.nft"
	defer func() {
		nftQoSConfigDir = oldDir
		nftQoSConfigFile = oldFile
		nftQoSRulesFile = oldRules
	}()

	limits := map[string]NftQoSLimit{
		"aa:bb:cc:dd:ee:ff": {MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Download: 10, Upload: 5},
	}
	if err := saveNftQoSLimits(limits); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded, err := loadNftQoSLimits()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 limit, got %d", len(loaded))
	}
	l := loaded["aa:bb:cc:dd:ee:ff"]
	if l.IP != "192.168.1.100" || l.Download != 10 || l.Upload != 5 {
		t.Fatalf("unexpected limit: %+v", l)
	}
}
