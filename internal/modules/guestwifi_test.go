package modules

import "testing"

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
