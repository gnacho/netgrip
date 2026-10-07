package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gnacho/netgrip/internal/ubus"
)

// fakeBoardBackend serves board reads without a device: the host running
// the suite has no ubus, so a green test here proves handleBoard goes
// through the Backend seam and never forks `ubus call system board`.
type fakeBoardBackend struct {
	calls    int
	board    json.RawMessage
	boardErr error
}

func (f *fakeBoardBackend) SystemInfo() (*ubus.SystemInfo, error) { return nil, nil }

func (f *fakeBoardBackend) WanStatus() (*ubus.WanStatus, error) { return nil, nil }

func (f *fakeBoardBackend) WirelessStatus() ([]ubus.WirelessRadio, error) { return nil, nil }

func (f *fakeBoardBackend) SystemBoard() (json.RawMessage, error) {
	f.calls++
	return f.board, f.boardErr
}

// GET /api/board behind requireAuth used to need a router: the handler
// forked the ubus CLI directly. With the backend swapped for a fake the
// handler answers 200 with the raw payload on the auth path, no device.
// The payload passes through untouched, so the test round-trips an
// arbitrary board-shaped body.
func TestBoardHandlerWithFakeBackend(t *testing.T) {
	board := json.RawMessage(`{"board_name":"TestRouter 3000","model_name":"TR3000","release":{"distribution":"OpenWrt","version":"24.10.0"}}`)
	fake := &fakeBoardBackend{board: board}
	ubus.SetBackendForTest(t, fake)
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/board", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/board: %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.calls != 1 {
		t.Errorf("backend calls = %d, want 1", fake.calls)
	}
	if rec.Body.String() != string(board) {
		t.Errorf("payload = %s, want the raw board JSON passthrough", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// A failing device read surfaces as 502, the same contract the handler had
// when it forked ubus directly.
func TestBoardHandlerBackendError(t *testing.T) {
	fake := &fakeBoardBackend{boardErr: errors.New("no ubus here")}
	ubus.SetBackendForTest(t, fake)
	s := New("http://127.0.0.1/ubus", "test", false, "")

	rec := authedRequest(t, s, "GET", "/api/board", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("GET /api/board: %d, want 502", rec.Code)
	}
}
