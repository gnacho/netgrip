package modules

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El panel de un compañero contesta 204 + cookie, no un token JSON: con la
// comprobación anterior (200 + {"token"}) toda adopción fallaba con "login
// failed" aunque la contraseña fuera correcta (#636).
func TestCheckNodeUpdateUsesTheSessionCookie(t *testing.T) {
	var gotPassword string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			var req struct {
				Password string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			gotPassword = req.Password
			if req.Password != "secreta" {
				http.Error(w, "invalid credentials", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "netgrip_session", Value: "tok", Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/api/selfupdate":
			c, err := r.Cookie("netgrip_session")
			if err != nil || c.Value != "tok" {
				http.Error(w, "no cookie", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"current": "v0.79.0", "latest": "v0.80.0", "available": true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	node := FleetNode{
		ID:       "n1",
		Name:     "rt4",
		Address:  strings.TrimPrefix(srv.URL, "http://"),
		Password: "secreta",
	}
	st := checkNodeUpdate(node)
	if st.Error != "" {
		t.Fatalf("no deberia fallar: %s", st.Error)
	}
	if !st.Reachable || st.CurrentVersion != "v0.79.0" || st.LatestVersion != "v0.80.0" {
		t.Fatalf("estado inesperado: %+v", st)
	}
	if gotPassword != "secreta" {
		t.Fatalf("el panel recibio %q como contraseña", gotPassword)
	}
}

func TestCheckNodeUpdateWrongPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
	}))
	defer srv.Close()

	st := checkNodeUpdate(FleetNode{Address: strings.TrimPrefix(srv.URL, "http://"), Password: "mala"})
	if !strings.Contains(st.Error, "wrong root password") {
		t.Fatalf("error = %q, want mencion clara de contraseña", st.Error)
	}
}
