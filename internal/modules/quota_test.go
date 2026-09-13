package modules

import (
	"strings"
	"testing"
	"time"
)

func TestQuotaPeriodKey(t *testing.T) {
	day := time.Date(2026, 9, 13, 23, 59, 0, 0, time.Local)
	nextDay := time.Date(2026, 9, 14, 0, 0, 0, 0, time.Local)
	if quotaPeriodKey(day, "daily") != "2026-09-13" {
		t.Fatalf("unexpected daily key: %s", quotaPeriodKey(day, "daily"))
	}
	if quotaPeriodKey(day, "daily") == quotaPeriodKey(nextDay, "daily") {
		t.Fatal("daily key should roll at midnight")
	}

	month := time.Date(2026, 9, 30, 23, 0, 0, 0, time.Local)
	nextMonth := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	if quotaPeriodKey(month, "monthly") != "2026-09" {
		t.Fatalf("unexpected monthly key: %s", quotaPeriodKey(month, "monthly"))
	}
	if quotaPeriodKey(month, "monthly") == quotaPeriodKey(nextMonth, "monthly") {
		t.Fatal("monthly key should roll at the month boundary")
	}
	if quotaPeriodKey(month, "monthly") != quotaPeriodKey(time.Date(2026, 9, 15, 0, 0, 0, 0, time.Local), "monthly") {
		t.Fatal("monthly key should be stable within a month")
	}
}

func TestQuotaByteMath(t *testing.T) {
	if !quotaExceeded(100, 100) {
		t.Fatal("used == limit should be exceeded")
	}
	if quotaExceeded(99, 100) {
		t.Fatal("used < limit should not be exceeded")
	}
	if quotaRemaining(40, 100) != 60 {
		t.Fatalf("remaining should be 60, got %d", quotaRemaining(40, 100))
	}
	if quotaRemaining(120, 100) != 0 {
		t.Fatalf("remaining should clamp to 0, got %d", quotaRemaining(120, 100))
	}
}

func TestShouldNotifyOnce(t *testing.T) {
	if !shouldNotifyOnce(true, false) {
		t.Fatal("crossed and not yet notified should notify")
	}
	if shouldNotifyOnce(true, true) {
		t.Fatal("already notified should not notify again")
	}
	if shouldNotifyOnce(false, false) {
		t.Fatal("not exceeded should not notify")
	}
}

func TestValidQuota(t *testing.T) {
	ok := Quota{MAC: "AA:BB:CC:DD:EE:FF", IP: "192.168.1.100", Period: "monthly", Limit: 1024, Action: "throttle", ThrottleKbps: 512}
	if err := validQuota(&ok); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if ok.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("expected normalized mac, got %s", ok.MAC)
	}

	bad := []Quota{
		{MAC: "", IP: "192.168.1.100", Period: "daily", Limit: 1, Action: "notify"},
		{MAC: "aa:bb:cc:dd:ee:ff", IP: "nope", Period: "daily", Limit: 1, Action: "notify"},
		{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "weekly", Limit: 1, Action: "notify"},
		{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "daily", Limit: 0, Action: "notify"},
		{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "daily", Limit: 1, Action: "ban"},
		{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "daily", Limit: 1, Action: "throttle", ThrottleKbps: 0},
	}
	for i, q := range bad {
		if err := validQuota(&q); err == nil {
			t.Fatalf("case %d should be invalid", i)
		}
	}
}

func TestBuildQuotaRuleset(t *testing.T) {
	quotas := map[string]Quota{
		"aa:bb:cc:dd:ee:ff": {MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "daily", Limit: 1000, Action: "throttle", ThrottleKbps: 256},
	}
	throttle := map[string]bool{"aa:bb:cc:dd:ee:ff": true}
	rules := buildQuotaRuleset(quotas, throttle)

	if !strings.Contains(rules, "table inet netgrip_quota") {
		t.Fatal("missing table declaration")
	}
	if !strings.Contains(rules, "ip saddr 192.168.1.100 counter") {
		t.Fatalf("missing upload counter, got:\n%s", rules)
	}
	if !strings.Contains(rules, "ip daddr 192.168.1.100 counter") {
		t.Fatalf("missing download counter, got:\n%s", rules)
	}
	if !strings.Contains(rules, "ip saddr 192.168.1.100 limit rate over 32 kbytes/second drop") {
		t.Fatalf("missing throttle rule (256 kbps = 32 kbytes/s), got:\n%s", rules)
	}
	if !strings.Contains(rules, "ip daddr 192.168.1.100 limit rate over 32 kbytes/second drop") {
		t.Fatalf("missing download throttle rule, got:\n%s", rules)
	}
}

func TestBuildQuotaRulesetNoThrottle(t *testing.T) {
	quotas := map[string]Quota{
		"aa:bb:cc:dd:ee:ff": {MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "daily", Limit: 1000, Action: "notify"},
	}
	rules := buildQuotaRuleset(quotas, map[string]bool{})
	if strings.Contains(rules, "limit rate") {
		t.Fatalf("notify action must not produce throttle rules, got:\n%s", rules)
	}
}

func TestQuotaConfigRoundtrip(t *testing.T) {
	dir := t.TempDir()
	old := quotaConfigFile
	quotaConfigFile = dir + "/quotas.json"
	defer func() { quotaConfigFile = old }()

	q := map[string]Quota{
		"aa:bb:cc:dd:ee:ff": {MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Period: "monthly", Limit: 50 * 1024 * 1024 * 1024, Action: "throttle", ThrottleKbps: 512},
	}
	if err := saveQuotaConfig(q); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded := loadQuotaConfig()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 quota, got %d", len(loaded))
	}
	if loaded["aa:bb:cc:dd:ee:ff"].Limit != 50*1024*1024*1024 {
		t.Fatalf("unexpected limit: %+v", loaded["aa:bb:cc:dd:ee:ff"])
	}
}

func TestQuotaStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	old := quotaStateFile
	quotaStateFile = dir + "/quota_state.json"
	defer func() { quotaStateFile = old }()

	s := map[string]*quotaStateEntry{
		"aa:bb:cc:dd:ee:ff": {Period: "daily", PeriodKey: "2026-09-13", IP: "192.168.1.100", Used: 123, LastRx: 10, LastTx: 20, Notified: true, Throttled: false},
	}
	if err := saveQuotaState(s); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	loaded := loadQuotaState()
	if loaded["aa:bb:cc:dd:ee:ff"] == nil || loaded["aa:bb:cc:dd:ee:ff"].Used != 123 {
		t.Fatalf("unexpected state: %+v", loaded["aa:bb:cc:dd:ee:ff"])
	}
	if loaded["aa:bb:cc:dd:ee:ff"].Notified != true {
		t.Fatalf("expected notified=true, got %+v", loaded["aa:bb:cc:dd:ee:ff"])
	}
}
