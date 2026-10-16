package modules

import (
	"strings"
	"testing"
)

func TestVendorForKnownOUI(t *testing.T) {
	// 9C:9D:7E is a Beijing Xiaomi Mobile Software OUI in the local IEEE table.
	if vendor, random := vendorFor("9c:9d:7e:11:22:33"); vendor != "Beijing Xiaomi Mobile Software" || random {
		t.Fatalf("vendorFor(9c:9d:7e:...) = (%q, %v), want (Beijing Xiaomi Mobile Software, false)", vendor, random)
	}
	// Uppercase input must resolve the same way.
	if vendor, random := vendorFor("9C:9D:7E:AA:BB:CC"); vendor != "Beijing Xiaomi Mobile Software" || random {
		t.Fatalf("vendorFor(9C:9D:7E:...) = (%q, %v), want (Beijing Xiaomi Mobile Software, false)", vendor, random)
	}
	// A second, unrelated entry so the lookup is not only exercised through
	// the one prefix the test happens to name (#520 review).
	if vendor, random := vendorFor("00:4b:12:11:22:33"); vendor != "Espressif" || random {
		t.Fatalf("vendorFor(00:4b:12:...) = (%q, %v), want (Espressif, false)", vendor, random)
	}
}

// TestVendorForTableIsSelfConsistent walks the whole OUI table: every key must
// be six uppercase hex digits without separators (the exact shape vendorFor
// builds) and must resolve back to its own vendor. A key written in any other
// shape would never match, which is the bug this guards against.
func TestVendorForTableIsSelfConsistent(t *testing.T) {
	if len(ouiVendors) == 0 {
		t.Fatal("ouiVendors is empty")
	}
	for prefix, want := range ouiVendors {
		if len(prefix) != 6 || strings.Contains(prefix, ":") || strings.ToUpper(prefix) != prefix {
			t.Fatalf("ouiVendors key %q must be 6 uppercase hex digits without separators", prefix)
		}
		for _, c := range prefix {
			if !strings.ContainsRune("0123456789ABCDEF", c) {
				t.Fatalf("ouiVendors key %q is not hexadecimal", prefix)
			}
		}
		mac := prefix[0:2] + ":" + prefix[2:4] + ":" + prefix[4:6] + ":11:22:33"
		if vendor, random := vendorFor(mac); vendor != want || random {
			t.Fatalf("vendorFor(%s) = (%q, %v), want (%q, false), table key %q", mac, vendor, random, want, prefix)
		}
	}
}

func TestVendorForLocallyAdministered(t *testing.T) {
	// 02:xx:xx has the locally administered bit set and is not multicast:
	// random, with no vendor lookup.
	if vendor, random := vendorFor("02:9c:9d:7e:11:22"); vendor != "" || !random {
		t.Fatalf("vendorFor(02:...) = (%q, %v), want (\"\", true)", vendor, random)
	}
}

func TestVendorForMulticastNotRandom(t *testing.T) {
	// 01:xx:xx is multicast: not flagged random (it is not a client MAC).
	if vendor, random := vendorFor("01:00:5e:00:00:01"); random {
		t.Fatalf("vendorFor(multicast) = (%q, %v), want random=false", vendor, random)
	}
}

func TestVendorForUnknownOUI(t *testing.T) {
	// A universally administered OUI that is not in the table.
	if vendor, random := vendorFor("00:00:00:ef:00:01"); vendor != "" || random {
		t.Fatalf("vendorFor(unknown) = (%q, %v), want (\"\", false)", vendor, random)
	}
}
