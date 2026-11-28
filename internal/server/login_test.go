package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gnacho/netgrip/internal/auth"
)

// loginPost posts a password to the login handler with no rpcd answer assumed
// until the test wires one.
func loginPost(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

// A router where rpcd is not running and no HTTP endpoint answers used to be
// reported by the app as "we couldn't reach the router", which sends people to
// check a router that is up (#812). The panel answers 502 so the login screen
// can say which service is missing (#813).
func TestLoginWithoutRPCDIsABadGateway(t *testing.T) {
	s := New("http://127.0.0.1:1/ubus", "test", false, "")
	s.rpcd.Socket = filepath.Join(t.TempDir(), "absent.sock")

	if rec := loginPost(t, s, `{"password":"whatever"}`); rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

func TestLoginRefusedIsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[6]}`))
	}))
	defer srv.Close()

	s := New(srv.URL, "test", false, "")
	s.rpcd.Socket = filepath.Join(t.TempDir(), "absent.sock")

	if rec := loginPost(t, s, `{"password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginAcceptedIssuesTheSessionCookie(t *testing.T) {
	auth.SecretPathForTest(t, t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[0,{"ubus_rpc_session":"tok"}]}`))
	}))
	defer srv.Close()

	s := New(srv.URL, "test", false, "")
	s.rpcd.Socket = filepath.Join(t.TempDir(), "absent.sock")

	rec := loginPost(t, s, `{"password":"right"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}
	if !auth.ValidSessionToken(cookies[0].Value) {
		t.Fatal("cookie does not carry a valid session token")
	}
}
