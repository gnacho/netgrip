package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gnacho/netgrip/internal/auth"
)

// authedRequest builds a request carrying a valid session cookie, so gate
// tests isolate the advanced check from the login flow. The session paths
// point at a temp dir: the suite does not run as root.
func authedRequest(t *testing.T, s *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	dir := t.TempDir()
	auth.SecretPathForTest(t, dir)
	tok, err := auth.NewSessionToken(10 * time.Minute)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req.AddCookie(s.sessionCookieFor(tok, 3600))
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func TestAdvancedSetRequiresConfirmation(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")

	// Enabling without confirm is refused before any UCI write happens.
	rec := authedRequest(t, s, "POST", "/api/advanced", `{"enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("enable without confirm: %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "confirmation") {
		t.Errorf("error should mention confirmation: %s", rec.Body.String())
	}
}

func TestAdvancedSetRejectsInvalidJSON(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")
	rec := authedRequest(t, s, "POST", "/api/advanced", `{not json`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid json: %d, want 400", rec.Code)
	}
}

// The gated endpoints must answer 404 while advanced mode is off. On the dev
// machine uci does not exist, which reads as off - exactly the state this
// test pins down.
func TestAdvancedGatedEndpoints404WhenOff(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")
	for _, tc := range []struct {
		method, target string
	}{
		{"GET", "/api/vlans"},
		{"GET", "/api/igmp"},
		{"GET", "/api/storm"},
		{"GET", "/api/mac-acl"},
	} {
		rec := authedRequest(t, s, tc.method, tc.target, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: %d, want 404", tc.method, tc.target, rec.Code)
		}
	}
}

// The probe itself is NOT gated: the Settings toggle needs it while the
// flag is off, and the nav needs it to decide whether to show the section.
func TestAdvancedProbeIsReachableWhenOff(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")
	rec := authedRequest(t, s, "GET", "/api/advanced", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/advanced: %d, want 200", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if out["advanced"] != false {
		t.Errorf("advanced = %v, want false without uci", out["advanced"])
	}
}
