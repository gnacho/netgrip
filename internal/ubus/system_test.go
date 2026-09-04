package ubus

import "testing"

func TestParseMeminfo(t *testing.T) {
	text := `MemTotal:       414112 kB
MemFree:         51240 kB
MemAvailable:   110080 kB
Buffers:         16384 kB
Cached:         204800 kB
SReclaimable:    60000 kB
SwapCached:          0 kB
`
	m := parseMeminfo(text)
	if m["MemTotal"] != 414112 || m["MemFree"] != 51240 || m["Buffers"] != 16384 || m["SReclaimable"] != 60000 {
		t.Fatalf("parsed wrong: %#v", m)
	}
	// available_eff = free + buffers + cached + sreclaimable (in KiB)
	avail := (m["MemFree"] + m["Buffers"] + m["Cached"] + m["SReclaimable"]) * 1024
	total := m["MemTotal"] * 1024
	usedPct := int(float64(total-avail) / float64(total) * 100)
	// With the raw MemAvailable the pct would be ~73%; the recomputed one
	// discounts the reclaimable cache and must be much lower.
	if usedPct > 45 {
		t.Fatalf("expected much lower used %% after discounting cache, got %d%%", usedPct)
	}
}
