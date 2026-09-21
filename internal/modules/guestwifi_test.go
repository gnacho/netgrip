package modules

import (
	"fmt"
	"testing"
)

// TestGuestSectionName pins the section naming contract between guestOps and
// ProbeGuest: the base section must always exist, extra radios get _N (#315).
func TestGuestSectionName(t *testing.T) {
	if got := guestSectionName(0); got != "netgrip_guest" {
		t.Fatalf("guestSectionName(0) = %q, want netgrip_guest", got)
	}
	if got := guestSectionName(1); got != "netgrip_guest_1" {
		t.Fatalf("guestSectionName(1) = %q, want netgrip_guest_1", got)
	}
	if got := guestSectionName(2); got != "netgrip_guest_2" {
		t.Fatalf("guestSectionName(2) = %q, want netgrip_guest_2", got)
	}
}

// TestPickGuestSubnet covers subnet selection at creation: default, custom,
// collisions (error for requested, auto-pick for the default) and rejection of
// non-private, network/broadcast and malformed addresses.
func TestPickGuestSubnet(t *testing.T) {
	cases := []struct {
		name      string
		requested string
		existing  []string
		want      string
		wantErr   bool
	}{
		{
			name:      "default",
			requested: "",
			want:      "192.168.10.0/24",
		},
		{
			name:      "custom valid",
			requested: "192.168.10.1",
			want:      "192.168.10.0/24",
		},
		{
			name:      "custom valid other private",
			requested: "10.0.0.1",
			want:      "10.0.0.0/24",
		},
		{
			name:      "custom colliding with LAN",
			requested: "192.168.1.1",
			existing:  []string{"192.168.1.0/24"},
			wantErr:   true,
		},
		{
			name:     "default auto-picks past collisions",
			existing: []string{"192.168.10.0/24", "192.168.11.0/24"},
			want:     "192.168.12.0/24",
		},
		{
			name:     "default normalizes non-canonical existing",
			existing: []string{"192.168.10.5/24"},
			want:     "192.168.11.0/24",
		},
		{
			name:      "public IP rejected",
			requested: "8.8.8.8",
			wantErr:   true,
		},
		{
			name:      "network address rejected",
			requested: "192.168.10.0",
			wantErr:   true,
		},
		{
			name:      "broadcast address rejected",
			requested: "192.168.10.255",
			wantErr:   true,
		},
		{
			name:      "loopback rejected",
			requested: "127.0.0.1",
			wantErr:   true,
		},
		{
			name:      "link-local rejected",
			requested: "169.254.1.1",
			wantErr:   true,
		},
		{
			name:      "malformed rejected",
			requested: "not-an-ip",
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pickGuestSubnet(tc.requested, tc.existing...)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("pickGuestSubnet(%q, %v) = %q, want error", tc.requested, tc.existing, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("pickGuestSubnet(%q, %v) error: %v", tc.requested, tc.existing, err)
			}
			if got != tc.want {
				t.Fatalf("pickGuestSubnet(%q, %v) = %q, want %q", tc.requested, tc.existing, got, tc.want)
			}
		})
	}
}

// TestPickGuestSubnetExhausted verifies the auto-pick is bounded: when every
// candidate /24 collides the picker errors instead of looping forever.
func TestPickGuestSubnetExhausted(t *testing.T) {
	var taken []string
	for i := 0; i < guestMaxSubnetTries; i++ {
		taken = append(taken, fmt.Sprintf("192.168.%d.0/24", 10+i))
	}
	if _, err := pickGuestSubnet("", taken...); err == nil {
		t.Fatalf("pickGuestSubnet default with %d collisions = nil error, want error", len(taken))
	}
}

// TestSubnetCIDR pins the ipaddr/netmask to CIDR normalization used to feed
// the collision check.
func TestSubnetCIDR(t *testing.T) {
	cases := []struct {
		ip, mask string
		want     string
	}{
		{"192.168.1.1", "255.255.255.0", "192.168.1.0/24"},
		{"10.0.0.1", "255.255.0.0", "10.0.0.0/16"},
		{"192.168.1.1", "255.0.255.0", ""}, // non-contiguous mask
		{"", "255.255.255.0", ""},
		{"192.168.1.1", "", ""},
	}
	for _, tc := range cases {
		if got := subnetCIDR(tc.ip, tc.mask); got != tc.want {
			t.Fatalf("subnetCIDR(%q, %q) = %q, want %q", tc.ip, tc.mask, got, tc.want)
		}
	}
}

// TestGuestRouterIP ensures a requested host is kept and a picked subnet gets
// its first host as the router address.
func TestGuestRouterIP(t *testing.T) {
	if got := guestRouterIP("192.168.10.5", "192.168.10.0/24"); got != "192.168.10.5" {
		t.Fatalf("guestRouterIP(requested) = %q, want 192.168.10.5", got)
	}
	if got := guestRouterIP("", "192.168.11.0/24"); got != "192.168.11.1" {
		t.Fatalf("guestRouterIP(default) = %q, want 192.168.11.1", got)
	}
}
