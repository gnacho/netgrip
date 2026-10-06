package server

import (
	"net/http"
	"strings"
	"testing"
)

// The schedule endpoint validates the payload before touching UCI, so the
// error paths are testable on the dev machine (no uci binary).
func TestWifiScheduleSetRejectsInvalidJSON(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")
	rec := authedRequest(t, s, "POST", "/api/wifischedule", `{not json`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid json: %d, want 400", rec.Code)
	}
}

func TestWifiScheduleSetRejectsInvalidSchedule(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")
	for _, body := range []string{
		`{"section":"","days":[1],"start":"22:00","end":"23:00"}`,          // missing section
		`{"section":"wifinet0","days":[1],"start":"25:00","end":"23:00"}`,  // bad HH:MM
		`{"section":"wifinet0","days":[7],"start":"22:00","end":"23:00"}`,  // bad day
		`{"section":"wifinet0","days":[],"start":"22:00","end":"23:00"}`,   // no days
	} {
		rec := authedRequest(t, s, "POST", "/api/wifischedule", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400 (%s)", body, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "error") {
			t.Errorf("%s: body should carry the error: %s", body, rec.Body.String())
		}
	}
}
