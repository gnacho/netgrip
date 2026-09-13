package modules

import (
	"reflect"
	"testing"
	"time"
)

func TestParentalBlocksAt(t *testing.T) {
	// Tuesday 2026-09-08 21:30 local, weekday=2 (Tuesday).
	tue := time.Date(2026, 9, 8, 21, 30, 0, 0, time.Local)
	mon := time.Date(2026, 9, 7, 21, 30, 0, 0, time.Local)    // Monday evening
	tueMorn := time.Date(2026, 9, 8, 6, 30, 0, 0, time.Local) // Tuesday morning
	wedMorn := time.Date(2026, 9, 9, 6, 30, 0, 0, time.Local) // Wednesday morning

	cases := []struct {
		name string
		r    ParentalRule
		t    time.Time
		want bool
	}{
		{"disabled never blocks", ParentalRule{Enabled: false, Days: []int{2}, Start: "21:00", End: "23:00"}, tue, false},
		{"paused wins over disabled", ParentalRule{Enabled: false, Paused: true, Days: []int{2}, Start: "21:00", End: "23:00"}, tue, true},
		{"same-day in window", ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "23:00"}, tue, true},
		{"same-day before window", ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "23:00"}, time.Date(2026, 9, 8, 20, 0, 0, 0, time.Local), false},
		{"same-day after window", ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "23:00"}, time.Date(2026, 9, 8, 23, 0, 0, 0, time.Local), false},
		{"same-day wrong day", ParentalRule{Enabled: true, Days: []int{1}, Start: "21:00", End: "23:00"}, tue, false},
		{"overnight evening", ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "07:00"}, tue, true},
		{"overnight morning belongs to yesterday", ParentalRule{Enabled: true, Days: []int{1}, Start: "21:00", End: "07:00"}, tueMorn, true},
		{"overnight morning not selected yesterday", ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "07:00"}, tueMorn, false},
		{"overnight continues into next morning", ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "07:00"}, wedMorn, true},
		{"overnight weekend start on friday", ParentalRule{Enabled: true, Days: []int{5}, Start: "21:00", End: "07:00"}, mon, false},
		{"degenerate start==end blocks whole day", ParentalRule{Enabled: true, Days: []int{2}, Start: "08:00", End: "08:00"}, time.Date(2026, 9, 8, 14, 0, 0, 0, time.Local), true},
		{"degenerate start==end wrong day", ParentalRule{Enabled: true, Days: []int{1}, Start: "08:00", End: "08:00"}, time.Date(2026, 9, 8, 14, 0, 0, 0, time.Local), false},
	}
	for _, c := range cases {
		if got := parentalBlocksAt(c.t, c.r); got != c.want {
			t.Errorf("%s: parentalBlocksAt(%v) = %v, want %v", c.name, c.t.Format("Mon 15:04"), got, c.want)
		}
	}
}

func TestParentalNextToday(t *testing.T) {
	tue := time.Date(2026, 9, 8, 18, 0, 0, 0, time.Local) // Tuesday 18:00
	cases := []struct {
		r    ParentalRule
		t    time.Time
		want string
	}{
		{ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "23:00"}, tue, "21:00"},
		{ParentalRule{Enabled: true, Days: []int{2}, Start: "21:00", End: "23:00"}, time.Date(2026, 9, 8, 22, 0, 0, 0, time.Local), ""}, // already past start
		{ParentalRule{Enabled: true, Days: []int{1}, Start: "21:00", End: "23:00"}, tue, ""},                                            // not today
		{ParentalRule{Enabled: false, Days: []int{2}, Start: "21:00", End: "23:00"}, tue, ""},                                           // disabled
		{ParentalRule{Enabled: true, Paused: true, Days: []int{2}, Start: "21:00", End: "23:00"}, tue, ""},                              // paused
	}
	for _, c := range cases {
		if got := parentalNextToday(c.t, c.r); got != c.want {
			t.Errorf("parentalNextToday(%+v) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestParseCableParental(t *testing.T) {
	out := `firewall.netgrip_parental_001122334455.name='netgrip-parental-00:11:22:33:44:55'
firewall.netgrip_parental_001122334455.src='lan'
firewall.netgrip_parental_001122334455.src_mac='00:11:22:33:44:55'
firewall.netgrip_block_aabbccddeeff.src_mac='AA:BB:CC:DD:EE:FF'
firewall.other.src_mac='99:88:77:66:55:44'`
	got := parseCableParental(out)
	if len(got) != 1 {
		t.Fatalf("expected 1 parental, got %d: %#v", len(got), got)
	}
	if !got["00:11:22:33:44:55"] {
		t.Fatal("expected 00:11:22:33:44:55 parental-blocked")
	}
	// Manual (netgrip_block_*) and unrelated src_mac must be ignored.
	if got["aa:bb:cc:dd:ee:ff"] || got["99:88:77:66:55:44"] {
		t.Fatal("manual/unrelated src_mac should not count as parental")
	}
}

func TestParentalJSONRoundtrip(t *testing.T) {
	old := parentalPath
	defer func() { parentalPath = old }()
	parentalPath = t.TempDir() + "/parental.json"

	rules := map[string]ParentalRule{
		"aa:bb:cc:dd:ee:ff": {MAC: "aa:bb:cc:dd:ee:ff", Enabled: true, Days: []int{0, 6}, Start: "21:00", End: "07:00"},
		"11:22:33:44:55:66": {MAC: "11:22:33:44:55:66", Enabled: true, Days: []int{1, 2, 3, 4, 5}, Start: "22:00", End: "23:00", Paused: true},
	}
	if err := saveParentalRules(rules); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := loadParentalRules()
	if !reflect.DeepEqual(got, rules) {
		t.Fatalf("roundtrip mismatch:\n got %#v\nwant %#v", got, rules)
	}
}

func TestValidParentalRule(t *testing.T) {
	if err := validParentalRule(&ParentalRule{MAC: "AA:BB:CC:DD:EE:FF", Days: []int{3, 1}, Start: "21:00", End: "07:00"}); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}
	if err := validParentalRule(&ParentalRule{MAC: "nope", Days: []int{0}, Start: "21:00", End: "07:00"}); err == nil {
		t.Fatal("invalid mac accepted")
	}
	if err := validParentalRule(&ParentalRule{MAC: "aa:bb:cc:dd:ee:ff", Days: []int{0}, Start: "25:00", End: "07:00"}); err == nil {
		t.Fatal("invalid time accepted")
	}
	if err := validParentalRule(&ParentalRule{MAC: "aa:bb:cc:dd:ee:ff", Days: []int{}, Start: "21:00", End: "07:00"}); err == nil {
		t.Fatal("empty days accepted")
	}
}
