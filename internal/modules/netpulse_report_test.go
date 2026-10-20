package modules

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Los reportes de upgrade usan el contrato del agente standalone: POST a
// /api/agents/{slug}/upgrade-progress|upgrade-result con el token del agente
// en Bearer (#549).
func TestNpUpgradeReportsPostToNetPulse(t *testing.T) {
	type req struct {
		path string
		auth string
		body map[string]any
	}
	reqs := []req{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body: %v", err)
		}
		reqs = append(reqs, req{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body})
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := NetPulseConfig{Server: srv.URL + "/", Slug: "flint2", Token: "tok-abc"}
	npReportUpgradeProgress(cfg, "downloading", 0)
	npReportUpgradeResult(cfg, false, "no space")

	if len(reqs) != 2 {
		t.Fatalf("%d peticiones, want 2", len(reqs))
	}
	if reqs[0].path != "/api/agents/flint2/upgrade-progress" {
		t.Fatalf("path 1: %s", reqs[0].path)
	}
	if reqs[0].auth != "Bearer tok-abc" {
		t.Fatalf("auth 1: %s", reqs[0].auth)
	}
	if reqs[0].body["step"] != "downloading" {
		t.Fatalf("body 1: %v", reqs[0].body)
	}
	if reqs[1].path != "/api/agents/flint2/upgrade-result" {
		t.Fatalf("path 2: %s", reqs[1].path)
	}
	if reqs[1].body["ok"] != false || reqs[1].body["error"] != "no space" {
		t.Fatalf("body 2: %v", reqs[1].body)
	}
}

// Config incompleta (sin server/slug/token): el reporte se omite en silencio
// en vez de petar.
func TestNpUpgradeReportSkipsIncompleteConfig(t *testing.T) {
	npReportUpgradeProgress(NetPulseConfig{}, "downloading", 0)
	npReportUpgradeResult(NetPulseConfig{Server: "http://x", Slug: "s"}, true, "")
}
