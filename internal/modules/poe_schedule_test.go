package modules

import "testing"

func TestPoeCronSchedule(t *testing.T) {
	cases := map[string]string{
		"07:30": "30 07 * * *",
		"00:00": "00 00 * * *",
		"23:59": "59 23 * * *",
		"06:05": "05 06 * * *",
	}
	for in, want := range cases {
		if got := poeCronSchedule(in); got != want {
			t.Errorf("poeCronSchedule(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRePoEScheduleTime(t *testing.T) {
	for _, s := range []string{"00:00", "07:30", "23:59", "09:05"} {
		if !rePoEScheduleTime.MatchString(s) {
			t.Errorf("rePoEScheduleTime should accept %q", s)
		}
	}
	for _, s := range []string{"", "7:30", "24:00", "12:60", "aa:bb", "12:30 ", "12:30:00", "*/1 * * * * echo hi"} {
		if rePoEScheduleTime.MatchString(s) {
			t.Errorf("rePoEScheduleTime must reject %q", s)
		}
	}
}

func TestSetPoEScheduleRejectsBadInput(t *testing.T) {
	// Time validation runs before any uci/sysfs side effect, so these error
	// paths are exercisable on any host.
	if _, err := SetPoESchedule(PoESchedule{Port: "lan1", OnTime: "nonsense"}); err == nil {
		t.Error("expected error for garbage on_time")
	}
	if _, err := SetPoESchedule(PoESchedule{Port: "lan1", OffTime: "25:00"}); err == nil {
		t.Error("expected error for out-of-range off_time")
	}
	if _, err := SetPoESchedule(PoESchedule{Port: "lan1", OnTime: "07:00 * * * * echo hi"}); err == nil {
		t.Error("a full crontab schedule smuggled into on_time must be rejected")
	}
	// A name that is not a bridge member is rejected as well. On a dev host
	// the bridge has no members, so any port lands in the same error path.
	if _, err := SetPoESchedule(PoESchedule{Port: "lan1", OnTime: "07:00"}); err == nil {
		t.Error("expected error for a port that is not a bridge member")
	}
	// The empty-port early return stays a no-op, not an error.
	if probe, err := SetPoESchedule(PoESchedule{}); err != nil || probe != nil {
		t.Errorf("empty schedule should be a silent no-op, got probe=%v err=%v", probe, err)
	}
}
