package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gnacho/netgrip/internal/modules"
	"github.com/gnacho/netgrip/internal/ubus"
)

// fakeWirelessBackend serves Wi-Fi reads without a device: the host running
// the suite has no ubus, so a green test here proves handleWireless goes
// through the Backend seam (even inside the probe cache) and never forks
// `ubus call network.wireless status`.
type fakeWirelessBackend struct {
	calls  int
	radios []ubus.WirelessRadio
	err    error
}

func (f *fakeWirelessBackend) SystemInfo() (*ubus.SystemInfo, error) { return nil, nil }

func (f *fakeWirelessBackend) WanStatus() (*ubus.WanStatus, error) { return nil, nil }

func (f *fakeWirelessBackend) WirelessStatus() ([]ubus.WirelessRadio, error) {
	f.calls++
	return f.radios, f.err
}

// GET /api/wireless behind requireAuth used to need a router: the handler
// forks the ubus CLI through GetWirelessStatus. With the backend swapped
// for a fake the handler answers 200 with the payload on the auth path,
// no device.
func TestWirelessHandlerWithFakeBackend(t *testing.T) {
	fake := &fakeWirelessBackend{radios: []ubus.WirelessRadio{{
		Name:    "radio0",
		Up:      true,
		Band:    "5g",
		Channel: "52",
		Htmode:  "HE80",
		TxPower: 20,
	}}}
	ubus.SetBackendForTest(t, fake)
	modules.InvalidateKey("wireless") // the probe cache outlives tests; start cold
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/wireless", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/wireless: %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.calls == 0 {
		t.Errorf("backend was never called; handler did not go through the seam")
	}
	var radios []ubus.WirelessRadio
	if err := json.Unmarshal(rec.Body.Bytes(), &radios); err != nil {
		t.Fatalf("response is not []WirelessRadio JSON: %v", err)
	}
	if len(radios) != 1 || radios[0].Name != "radio0" || radios[0].Channel != "52" {
		t.Errorf("unexpected payload: %+v", radios)
	}
}

// A failing device read surfaces as 502, the same contract the handler had
// when it read the device directly.
func TestWirelessHandlerBackendError(t *testing.T) {
	fake := &fakeWirelessBackend{err: errors.New("no ubus here")}
	ubus.SetBackendForTest(t, fake)
	modules.InvalidateKey("wireless") // CachedRead caches errors too; start cold
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/wireless", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("GET /api/wireless: %d, want 502", rec.Code)
	}
}
