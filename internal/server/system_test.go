package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gnacho/netgrip/internal/ubus"
)

// fakeSystemBackend serves system reads without a device: the host running
// the suite has no ubus, so a green test here proves the handler goes
// through the Backend seam and never forks ubus/uci.
type fakeSystemBackend struct {
	calls int
	info  *ubus.SystemInfo
	err   error
}

func (f *fakeSystemBackend) SystemInfo() (*ubus.SystemInfo, error) {
	f.calls++
	return f.info, f.err
}

// WanStatus is only exercised by the WAN handler tests; the fake keeps the
// interface satisfied without a device.
func (f *fakeSystemBackend) WanStatus() (*ubus.WanStatus, error) { return nil, nil }

func (f *fakeSystemBackend) WirelessStatus() ([]ubus.WirelessRadio, error) {
	return nil, nil
}

// GET /api/system behind requireAuth used to need a router: GetSystemInfo
// forks `ubus call system info`. With the backend swapped for a fake the
// handler answers 200 with the payload, on the auth path, no device.
func TestSystemHandlerWithFakeBackend(t *testing.T) {
	fake := &fakeSystemBackend{info: &ubus.SystemInfo{
		Uptime: 12345,
		Load:   []float64{0.5, 0.25, 0.125},
	}}
	ubus.SetBackendForTest(t, fake)
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/system", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/system: %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.calls != 1 {
		t.Errorf("backend calls = %d, want 1", fake.calls)
	}
	var info ubus.SystemInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("response is not SystemInfo JSON: %v", err)
	}
	if info.Uptime != 12345 || len(info.Load) != 3 || info.Load[0] != 0.5 {
		t.Errorf("unexpected payload: %+v", info)
	}
}

// A failing device read surfaces as 502, the same contract the handler had
// when it forked ubus directly.
func TestSystemHandlerBackendError(t *testing.T) {
	fake := &fakeSystemBackend{err: errors.New("no ubus here")}
	ubus.SetBackendForTest(t, fake)
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/system", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("GET /api/system: %d, want 502", rec.Code)
	}
}
