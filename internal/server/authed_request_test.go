package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gnacho/netgrip/internal/auth"
)

// authedRequest builds a request carrying a valid session cookie, so handler
// tests isolate authentication from the login flow. The session paths point
// at a temp dir: the suite does not run as root.
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
