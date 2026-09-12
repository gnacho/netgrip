package modules

import (
	"reflect"
	"testing"
)

func TestProfileSqmOpts(t *testing.T) {
	tests := []struct {
		profile string
		want    map[string]string
	}{
		// balanced deletes both managed options: exactly today's behavior.
		{"balanced", map[string]string{"eqdisc_opts": "", "iqdisc_opts": ""}},
		{"gaming", map[string]string{"eqdisc_opts": "diffserv4", "iqdisc_opts": "diffserv4"}},
		{"streaming", map[string]string{"eqdisc_opts": "besteffort", "iqdisc_opts": "besteffort"}},
		// custom leaves the managed options untouched (raw CAKE stays in LuCI).
		{"custom", nil},
		// empty/unknown fall back to balanced (defensive; sqmOps validates first).
		{"", map[string]string{"eqdisc_opts": "", "iqdisc_opts": ""}},
		{"unknown", map[string]string{"eqdisc_opts": "", "iqdisc_opts": ""}},
	}
	for _, tc := range tests {
		got := profileSqmOpts(tc.profile)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("profileSqmOpts(%q) = %v, want %v", tc.profile, got, tc.want)
		}
	}
}

// TestNormalizeProfile covers parsing the active profile from the UCI-stored
// value (what uciGet("sqm.netgrip.profile") returns). Existing installs have no
// option, so uciGet returns "" and must resolve to "balanced".
func TestNormalizeProfile(t *testing.T) {
	tests := map[string]string{
		"":          "balanced",
		"balanced":  "balanced",
		"gaming":    "gaming",
		"streaming": "streaming",
		"custom":    "custom",
		"unknown":   "balanced",
	}
	for in, want := range tests {
		if got := normalizeProfile(in); got != want {
			t.Errorf("normalizeProfile(%q) = %q, want %q", in, got, want)
		}
	}
}
