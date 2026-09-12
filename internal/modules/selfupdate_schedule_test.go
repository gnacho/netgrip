package modules

import "testing"

func TestSelfUpdateConfigNormalized(t *testing.T) {
	cases := []struct {
		in, want SelfUpdateConfig
	}{
		{SelfUpdateConfig{}, SelfUpdateConfig{Enabled: false, IntervalH: 168, WindowStart: 0, WindowEnd: 2}},
		{SelfUpdateConfig{IntervalH: -5, WindowStart: 40, WindowEnd: 90}, SelfUpdateConfig{IntervalH: 168, WindowStart: 3, WindowEnd: 5}},
		{SelfUpdateConfig{IntervalH: 99999, WindowStart: 23, WindowEnd: 25}, SelfUpdateConfig{IntervalH: 720, WindowStart: 23, WindowEnd: 1}},
		{SelfUpdateConfig{IntervalH: 24, WindowStart: 22, WindowEnd: 22}, SelfUpdateConfig{IntervalH: 24, WindowStart: 22, WindowEnd: 24}},
	}
	for _, c := range cases {
		if got := c.in.normalized(); got != c.want {
			t.Errorf("normalized(%+v) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestSelfUpdateInWindow(t *testing.T) {
	cases := []struct {
		cfg      SelfUpdateConfig
		hour     int
		expected bool
	}{
		{SelfUpdateConfig{WindowStart: 3, WindowEnd: 5}, 3, true},
		{SelfUpdateConfig{WindowStart: 3, WindowEnd: 5}, 4, true},
		{SelfUpdateConfig{WindowStart: 3, WindowEnd: 5}, 5, false},
		{SelfUpdateConfig{WindowStart: 3, WindowEnd: 5}, 2, false},
		{SelfUpdateConfig{WindowStart: 22, WindowEnd: 2}, 23, true}, // wraps midnight
		{SelfUpdateConfig{WindowStart: 22, WindowEnd: 2}, 0, true},
		{SelfUpdateConfig{WindowStart: 22, WindowEnd: 2}, 1, true},
		{SelfUpdateConfig{WindowStart: 22, WindowEnd: 2}, 2, false},
		{SelfUpdateConfig{WindowStart: 22, WindowEnd: 2}, 12, false},
		{SelfUpdateConfig{WindowStart: 4, WindowEnd: 4}, 13, true}, // degenerate: always open
	}
	for _, c := range cases {
		if got := c.cfg.inWindow(c.hour); got != c.expected {
			t.Errorf("inWindow(%+v, %d) = %v, want %v", c.cfg, c.hour, got, c.expected)
		}
	}
}
