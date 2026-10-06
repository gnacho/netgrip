package modules

import (
	"testing"
	"time"
)

// 2026-01-04 was a Sunday, so 2026-01-04+weekday lands on that weekday.
func at(t *testing.T, weekday time.Weekday, hhmm string) time.Time {
	t.Helper()
	if weekday < time.Sunday || weekday > time.Saturday {
		t.Fatalf("bad weekday %v", weekday)
	}
	base := time.Date(2026, 1, 4+int(weekday), 0, 0, 0, 0, time.Local)
	return base.Add(time.Duration(hhmmToMinutes(hhmm)) * time.Minute)
}

func TestWifiScheduleWindowAt(t *testing.T) {
	s := WifiSchedule{Enabled: true, Days: []int{1, 2, 3, 4, 5}, Start: "22:00", End: "23:00"}
	cases := []struct {
		name string
		when time.Time
		want bool
	}{
		{"inside weekday window", at(t, time.Monday, "22:30"), true},
		{"before window", at(t, time.Monday, "21:59"), false},
		{"after window", at(t, time.Monday, "23:00"), false},
		{"weekend not selected", at(t, time.Saturday, "22:30"), false},
		{"disabled schedule", time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := s
			if tc.name == "disabled schedule" {
				s.Enabled = false
				tc.when = at(t, time.Monday, "22:30")
			}
			if got := wifiScheduleWindowAt(tc.when, s); got != tc.want {
				t.Errorf("wifiScheduleWindowAt(%s, %+v) = %v, want %v", tc.when, s, got, tc.want)
			}
		})
	}
}

func TestWifiScheduleWindowAtOvernight(t *testing.T) {
	// 22:00 - 07:00 on weekdays: Monday 23:00 and Tuesday 06:00 are inside,
	// Saturday 23:00 and Monday 12:00 are outside.
	s := WifiSchedule{Enabled: true, Days: []int{1, 2, 3, 4, 5}, Start: "22:00", End: "07:00"}
	for hhmm, wd := range map[string]time.Weekday{
		"23:00": time.Monday, "06:00": time.Tuesday, "06:59": time.Tuesday, "22:00": time.Friday,
	} {
		if !wifiScheduleWindowAt(at(t, wd, hhmm), s) {
			t.Errorf("expected inside window at %s %s", wd, hhmm)
		}
	}
	for hhmm, wd := range map[string]time.Weekday{
		"23:00": time.Saturday, "12:00": time.Monday, "21:59": time.Monday, "07:00": time.Tuesday, "06:00": time.Sunday,
	} {
		if wifiScheduleWindowAt(at(t, wd, hhmm), s) {
			t.Errorf("expected outside window at %s %s", wd, hhmm)
		}
	}
}

func TestWifiScheduleWindowAtDegenerateWholeDay(t *testing.T) {
	s := WifiSchedule{Enabled: true, Days: []int{0, 6}, Start: "00:00", End: "00:00"}
	if !wifiScheduleWindowAt(at(t, time.Sunday, "15:00"), s) {
		t.Error("whole-day window should be active all Sunday")
	}
	if wifiScheduleWindowAt(at(t, time.Monday, "15:00"), s) {
		t.Error("whole-day window should be inactive on Monday")
	}
}

func TestWifiScheduleOffAtPausedWins(t *testing.T) {
	s := WifiSchedule{Enabled: false, Days: []int{0, 1, 2, 3, 4, 5, 6}, Start: "00:00", End: "00:00", Paused: true}
	if !wifiScheduleOffAt(at(t, time.Wednesday, "10:00"), s) {
		t.Error("paused schedule must report off even outside the window")
	}
	s.Paused = false
	if wifiScheduleOffAt(at(t, time.Wednesday, "10:00"), s) {
		t.Error("unpaused disabled schedule must not report off")
	}
}

func TestWifiScheduleOps(t *testing.T) {
	// On the dev machine uci is absent, so radiosForSections resolves none
	// and no wifi_reconf op is appended; on the router there is one per
	// affected radio.
	ops := wifiScheduleOps([]string{"wifinet0", "wifinet1"}, true)
	if len(ops) != 3 { // 2 uci_set + commit
		t.Fatalf("got %d ops, want 3 (2 sets + commit)", len(ops))
	}
	if ops[0].Kind != "uci_set" || ops[0].Args[0] != "wireless.wifinet0.disabled" || ops[0].Args[1] != "1" {
		t.Errorf("unexpected first op: %+v", ops[0])
	}
	foundCommit := false
	for _, o := range ops {
		if o.Kind == "uci_commit" && o.Args[0] == "wireless" {
			foundCommit = true
		}
	}
	if !foundCommit {
		t.Error("missing uci_commit wireless op")
	}

	ops = wifiScheduleOps([]string{"wifinet0"}, false)
	if ops[0].Args[1] != "0" {
		t.Errorf("enable op should set disabled 0, got %+v", ops[0])
	}
}

func TestValidWifiSchedule(t *testing.T) {
	s := WifiSchedule{Section: "wifinet0", Enabled: true, Days: []int{5, 1, 1, 3}, Start: "22:00", End: "07:00"}
	if err := validWifiSchedule(&s); err != nil {
		t.Fatalf("valid schedule rejected: %v", err)
	}
	if len(s.Days) != 3 || s.Days[0] != 1 || s.Days[1] != 3 || s.Days[2] != 5 {
		t.Errorf("days not deduped/sorted: %v", s.Days)
	}

	bad := []WifiSchedule{
		{Section: "", Days: []int{1}, Start: "22:00", End: "23:00"},
		{Section: "wifinet0", Days: []int{1}, Start: "25:00", End: "23:00"},
		{Section: "wifinet0", Days: []int{1}, Start: "22:00", End: "7:00"},
		{Section: "wifinet0", Days: []int{}, Start: "22:00", End: "23:00"},
		{Section: "wifinet0", Days: []int{7}, Start: "22:00", End: "23:00"},
	}
	for i, b := range bad {
		if err := validWifiSchedule(&b); err == nil {
			t.Errorf("case %d: expected validation error for %+v", i, b)
		}
	}
}
