package modules

import "testing"

// #531: admin_up debe salir del bit IFF_UP de flags, no de carrier.
func TestPortAdminUp(t *testing.T) {
	cases := []struct {
		flags string
		want  bool
	}{
		{"0x1303", true},  // lan1 real (rt-lab): IFF_UP|IFF_BROADCAST|IFF_MULTICAST|IFF_LOWER_UP, sin cable
		{"0x1003", true},  // wan real (rt-lab): IFF_UP|IFF_BROADCAST|IFF_MULTICAST
		{"0x1302", false}, // IFF_UP apagado (boca desactivada de verdad)
		{"0x0", false},
		{"basura", false},
		{"", false},
	}
	for _, c := range cases {
		if got := portAdminUp(c.flags); got != c.want {
			t.Errorf("portAdminUp(%q) = %v, quiero %v", c.flags, got, c.want)
		}
	}
}
