package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gnacho/netgrip/internal/ubus"
)

// fakeWanBackend serves WAN reads without a device: the host running the
// suite has no ubus, so a green test here proves handleWan goes through the
// Backend seam and never forks `ubus call network.interface dump`.
type fakeWanBackend struct {
	calls  int
	status *ubus.WanStatus
	err    error
}

func (f *fakeWanBackend) SystemInfo() (*ubus.SystemInfo, error) { return nil, nil }

func (f *fakeWanBackend) WanStatus() (*ubus.WanStatus, error) {
	f.calls++
	return f.status, f.err
}

func (f *fakeWanBackend) WirelessStatus() ([]ubus.WirelessRadio, error) {
	return nil, nil
}
func (f *fakeWanBackend) SystemBoard() (json.RawMessage, error) { return nil, nil }

// GET /api/wan behind requireAuth used to need a router: GetWanStatus forks
// the ubus CLI (and, since #467, the native client). With the backend
// swapped for a fake the handler answers 200 with the payload on the auth
// path, no device.
func TestWanHandlerWithFakeBackend(t *testing.T) {
	fake := &fakeWanBackend{status: &ubus.WanStatus{
		Present: true,
		Up:      true,
		Uptime:  83349,
		IPv4:    []string{"192.0.2.10"},
		Gateway: "192.0.2.1",
		DNS:     []string{"192.0.2.1"},
	}}
	ubus.SetBackendForTest(t, fake)
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/wan", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/wan: %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.calls != 1 {
		t.Errorf("backend calls = %d, want 1", fake.calls)
	}
	var st ubus.WanStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("response is not WanStatus JSON: %v", err)
	}
	if !st.Present || !st.Up || st.Uptime != 83349 || len(st.IPv4) != 1 || st.IPv4[0] != "192.0.2.10" {
		t.Errorf("unexpected payload: %+v", st)
	}
}

// A failing device read surfaces as 502, the same contract the handler had
// when it read the device directly.
func TestWanHandlerBackendError(t *testing.T) {
	fake := &fakeWanBackend{err: errors.New("no ubus here")}
	ubus.SetBackendForTest(t, fake)
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/wan", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("GET /api/wan: %d, want 502", rec.Code)
	}
}
