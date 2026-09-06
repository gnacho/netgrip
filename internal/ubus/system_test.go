package ubus

import "testing"

func TestParseVmRSS(t *testing.T) {
	status := `Name:	netgrip
VmPeak:	   86720 kB
VmRSS:	   17824 kB
VmLck:	       0 kB
VmSwap:	       0 kB
`
	if got := parseVmRSS(status); got != 17824*1024 {
		t.Fatalf("parseVmRSS = %d, want %d", got, 17824*1024)
	}
	// Sin línea VmRSS → 0 (best-effort).
	if got := parseVmRSS("Name:	foo\nVmPeak:	   86720 kB\n"); got != 0 {
		t.Fatalf("parseVmRSS sin VmRSS = %d, want 0", got)
	}
	// Línea malformada → 0 (sin panic).
	if got := parseVmRSS("VmRSS:	no-number here\n"); got != 0 {
		t.Fatalf("parseVmRSS malformado = %d, want 0", got)
	}
}
