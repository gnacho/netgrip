package modules

import (
	"errors"
	"testing"
)

// Phase 2 of plans/backend-seam-clients: uci reads run through the uciExec
// seam, so dhcpReservations (the hottest uci read on the clients path) can
// be fed a fake `uci show dhcp` without a router. uciShowCached memoizes,
// so the test starts cold.
func TestDhcpReservationsWithFakeUci(t *testing.T) {
	const showDhcp = "dhcp.host1=host\ndhcp.host1.name='portatil'\ndhcp.host1.mac='AA:BB:CC:DD:EE:FF'\ndhcp.host1.ip='192.168.1.50'\n"
	calls := 0
	SetUciExecForTest(t, func(args ...string) ([]byte, error) {
		calls++
		if len(args) == 2 && args[0] == "show" && args[1] == "dhcp" {
			return []byte(showDhcp), nil
		}
		return nil, errors.New("unexpected uci call: " + joinArgs(args))
	})
	InvalidateKey("ucishow|dhcp")

	res := dhcpReservations()
	if len(res) != 1 {
		t.Fatalf("dhcpReservations() = %d entries, want 1: %+v", len(res), res)
	}
	r, ok := res["aa:bb:cc:dd:ee:ff"]
	if !ok {
		t.Fatalf("reservation keyed as aa:bb:cc:dd:ee:ff missing: %+v", res)
	}
	if r.Name != "portatil" || r.IP != "192.168.1.50" {
		t.Errorf("reservation = %+v, want portatil @ 192.168.1.50", r)
	}
	if calls == 0 {
		t.Errorf("fake uciExec was never called; the read did not go through the seam")
	}
}

// A failing uci read keeps the established contract: no reservation data,
// no panic, and the listing still works (the real dhcpReservations ignores
// the memoized error the same way it ignored a fork failure). The gateway
// SSH fallback is faked too: on a LAN machine it can succeed and leak real
// data into the test, which is exactly what this seam exists to prevent.
func TestDhcpReservationsUciError(t *testing.T) {
	SetUciExecForTest(t, func(args ...string) ([]byte, error) {
		return nil, errors.New("uci not found")
	})
	SetGatewaySSHForTest(t, func(command string) (string, error) {
		return "", errors.New("no gateway in test")
	})
	InvalidateKey("ucishow|dhcp")

	if res := dhcpReservations(); len(res) != 0 {
		t.Errorf("dhcpReservations() = %+v, want empty on uci failure", res)
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
