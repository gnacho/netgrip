package server

import (
	"net/http"
	"testing"
)

// El handler valida antes de tocar uci: un cuerpo sin puerto es un 400 y no
// un intento de mover "nada" en la configuración del router.
func TestPortRoleHandlersRejectEmptyPort(t *testing.T) {
	s := New("http://127.0.0.1/ubus", "test", false, "")
	cases := []struct{ path, body string }{
		{"/api/ports/role", `{"port":"","role":"wan"}`},
		{"/api/ports/role", `not json`},
		{"/api/ports/mac", `{"port":"","mac":"aa:bb:cc:dd:ee:f0"}`},
		{"/api/ports/mac", `not json`},
	}
	for _, c := range cases {
		rec := authedRequest(t, s, "POST", c.path, c.body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("POST %s %q = %d, want 400: %s", c.path, c.body, rec.Code, rec.Body.String())
		}
	}
}
