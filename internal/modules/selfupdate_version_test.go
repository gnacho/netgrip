package modules

import "testing"

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		tag, current string
		want         bool
	}{
		{"v0.22.1", "0.22.0", true},
		{"v0.22.1", "0.23.0", false},
		{"v0.23.0", "0.23.0", false},
		{"v0.23.0", "0.23.0-r1", false}, // r = package iteration, same app version
		{"0.24.0", "v0.23.9", true},
		{"v1.0.0", "0.99.99", true},
		{"", "0.22.0", false},
		{"v0.22.1", "dev", true},
		{"nonsense", "0.22.0", true},
		{"0.22.0", "0.22.0", false},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.tag, c.current); got != c.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", c.tag, c.current, got, c.want)
		}
	}
}
