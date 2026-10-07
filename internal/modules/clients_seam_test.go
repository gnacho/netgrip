package modules

import (
	"encoding/json"
	"testing"

	"github.com/gnacho/netgrip/internal/ubus"
)

// clientTypeFor only reads wireless status, so it is the first ListClients
// helper testable without a device: with the Backend swapped for a fake it
// must classify a MAC from the fake radios and never fork ubus. This pins
// phase 1 of plans/backend-seam-clients: modules read through the seam.
func TestClientTypeForUsesBackendSeam(t *testing.T) {
	fake := &fakeWirelessBackend{radios: []ubus.WirelessRadio{{
		Name: "radio0",
		Interfaces: []ubus.WirelessInterface{{
			Ifname: "phy0-ap0",
			Clients: []ubus.WirelessClient{
				{MAC: "AA:BB:CC:DD:EE:FF"},
			},
		}},
	}}}
	ubus.SetBackendForTest(t, fake)

	if got := clientTypeFor("aa:bb:cc:dd:ee:ff"); got != "wifi" {
		t.Errorf("clientTypeFor(known MAC) = %q, want wifi", got)
	}
	if got := clientTypeFor("00:11:22:33:44:55"); got != "" {
		t.Errorf("clientTypeFor(unknown MAC) = %q, want empty", got)
	}
	if fake.calls == 0 {
		t.Errorf("fake backend was never called; the helper did not go through the seam")
	}
}

type fakeWirelessBackend struct {
	calls  int
	radios []ubus.WirelessRadio
}

func (f *fakeWirelessBackend) SystemInfo() (*ubus.SystemInfo, error) { return nil, nil }

func (f *fakeWirelessBackend) WanStatus() (*ubus.WanStatus, error) { return nil, nil }

func (f *fakeWirelessBackend) WirelessStatus() ([]ubus.WirelessRadio, error) {
	f.calls++
	return f.radios, nil
}

func (f *fakeWirelessBackend) SystemBoard() (json.RawMessage, error) { return nil, nil }
