package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func jsonRPCHandler(t *testing.T, body string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

func TestProbeAcceptsJSONRPCResponse(t *testing.T) {
	srv := httptest.NewServer(jsonRPCHandler(t, `{"jsonrpc":"2.0","id":1,"result":[0,{}]}`))
	defer srv.Close()
	if !Probe(srv.URL) {
		t.Fatal("Probe = false for a JSON-RPC endpoint, want true")
	}
}

func TestProbeAcceptsJSONRPCPermissionDenied(t *testing.T) {
	srv := httptest.NewServer(jsonRPCHandler(t, `{"jsonrpc":"2.0","id":1,"result":[6]}`))
	defer srv.Close()
	if !Probe(srv.URL) {
		t.Fatal("Probe = false for a JSON-RPC permission-denied response, want true")
	}
}

func TestProbeRejectsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>302 Found</title></head><body></body></html>"))
	}))
	defer srv.Close()
	if Probe(srv.URL) {
		t.Fatal("Probe = true for an HTML endpoint, want false")
	}
}

func TestProbeRejectsRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/portal")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	if Probe(srv.URL) {
		t.Fatal("Probe = true for a redirecting endpoint, want false")
	}
}

func TestSessionLoginOverSelfSignedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(jsonRPCHandler(t, `{"jsonrpc":"2.0","id":1,"result":[0,{"ubus_rpc_session":"tok123"}]}`))
	defer srv.Close()
	token, err := SessionLogin(srv.URL, "root", "pass")
	if err != nil {
		t.Fatalf("SessionLogin over self-signed TLS: %v", err)
	}
	if token != "tok123" {
		t.Fatalf("SessionLogin token = %q, want tok123", token)
	}
}

func TestSessionLoginWrongPassword(t *testing.T) {
	srv := httptest.NewServer(jsonRPCHandler(t, `{"jsonrpc":"2.0","id":1,"result":[6]}`))
	defer srv.Close()
	token, err := SessionLogin(srv.URL, "root", "bad")
	if err != nil {
		t.Fatalf("SessionLogin with wrong password returned error: %v", err)
	}
	if token != "" {
		t.Fatalf("SessionLogin token = %q, want empty", token)
	}
}

func TestDetectRPCdEndpointPicksWorkingCandidate(t *testing.T) {
	html := httptest.NewServer(jsonRPCHandler(t, "<html></html>"))
	defer html.Close()
	good := httptest.NewServer(jsonRPCHandler(t, `{"jsonrpc":"2.0","id":1,"result":[0]}`))
	defer good.Close()

	orig := CandidateRPCdURLs
	CandidateRPCdURLs = []string{html.URL, good.URL}
	defer func() { CandidateRPCdURLs = orig }()

	if got := DetectRPCdEndpoint(""); got != good.URL {
		t.Fatalf("DetectRPCdEndpoint = %q, want %q", got, good.URL)
	}
}

func TestDetectRPCdEndpointNoCandidate(t *testing.T) {
	bad := httptest.NewServer(jsonRPCHandler(t, "<html></html>"))
	defer bad.Close()

	orig := CandidateRPCdURLs
	CandidateRPCdURLs = []string{bad.URL}
	defer func() { CandidateRPCdURLs = orig }()

	if got := DetectRPCdEndpoint(""); got != "" {
		t.Fatalf("DetectRPCdEndpoint = %q, want empty", got)
	}
}

func TestDetectRPCdEndpointExplicitWins(t *testing.T) {
	bad := httptest.NewServer(jsonRPCHandler(t, "<html></html>"))
	defer bad.Close()

	orig := CandidateRPCdURLs
	CandidateRPCdURLs = []string{bad.URL}
	defer func() { CandidateRPCdURLs = orig }()

	want := "https://127.0.0.1:9999/ubus"
	if got := DetectRPCdEndpoint(want); got != want {
		t.Fatalf("DetectRPCdEndpoint = %q, want explicit %q", got, want)
	}
}
