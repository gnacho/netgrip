package server

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestMCPURLFollowsTheRequest(t *testing.T) {
	plain := httptest.NewRequest("GET", "http://192.168.1.1:8090/api/mcp", nil)
	if got := mcpURL(plain); got != "http://192.168.1.1:8090/mcp" {
		t.Fatalf("mcpURL = %q", got)
	}
	secure := httptest.NewRequest("GET", "https://router.lan/api/mcp", nil)
	secure.TLS = &tls.ConnectionState{}
	if got := mcpURL(secure); got != "https://router.lan/mcp" {
		t.Fatalf("mcpURL over TLS = %q", got)
	}
}
