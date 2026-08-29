// netpulse_discovery_test.go — #147: descubrimiento zero-touch.
package modules

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gnacho/netpulse/agent/runtime"
)

func TestNetPulseSanitizeSlug(t *testing.T) {
	cases := map[string]string{
		"redmi-ax6-2":     "redmi-ax6-2",
		"Redmi AX6!":      "redmi-ax6",
		"OpenWrt_Router":  "openwrt-router",
		"  patio  ":       "patio",
		"--patio--":       "patio",
		"pa--tio":         "pa-tio",
		"UPPER-CASE":      "upper-case",
		"":                "netgrip",
		"***":             "netgrip",
		"192.168.1.1":     "192-168-1-1",
		"muy-largo-12345": "muy-largo-12345",
	}
	for in, want := range cases {
		if got := netPulseSanitizeSlug(in); got != want {
			t.Fatalf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNetPulseStatusConnectedRule(t *testing.T) {
	now := time.Now()
	if netPulseStatusConnected(runtime.Status{}) {
		t.Fatal("estado vacío nunca es connected")
	}
	if !netPulseStatusConnected(runtime.Status{Running: true, PushOk: true, LastPush: now}) {
		t.Fatal("push OK reciente debe ser connected")
	}
	old := runtime.Status{Running: true, PushOk: true, LastPush: now.Add(-10 * time.Minute)}
	if netPulseStatusConnected(old) {
		t.Fatal("push de hace 10 min no es connected")
	}
	noOk := runtime.Status{Running: true, PushOk: false, LastPush: now}
	if netPulseStatusConnected(noOk) {
		t.Fatal("sin PushOk no es connected")
	}
}

// withFakeDiscovery intercambia probe/enroll por fakes y restaura al final.
func withFakeDiscovery(t *testing.T, probe func(port int, timeout time.Duration) *netPulseDiscoveryResult, enroll func(p netpulsePaths, server, token string) error) {
	t.Helper()
	oldProbe, oldEnroll := npProbe, npEnroll
	npProbe, npEnroll = probe, enroll
	t.Cleanup(func() { npProbe, npEnroll = oldProbe, oldEnroll })
}

// waitEnrolled: espera (eventually) a que el estado de discovery registre
// un intento de enrollment.
func waitEnrolled(t *testing.T) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !netPulseDiscoverySnapshot().LastEnrollAt.IsZero() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func resetDiscoveryState(t *testing.T) {
	t.Helper()
	npDiscMu.Lock()
	npDisc = netPulseDiscoveryState{}
	npEnrolling = false
	npDiscMu.Unlock()
	StopNetPulseAgent()
	t.Cleanup(func() {
		npDiscMu.Lock()
		npDisc = netPulseDiscoveryState{}
		npEnrolling = false
		npDiscMu.Unlock()
		StopNetPulseAgent()
	})
}

func TestDiscoveryEnrollsOnIncompleteConfig(t *testing.T) {
	resetDiscoveryState(t)
	p := tmpPaths(t) // sin env file: config incompleta

	var mu sync.Mutex
	gotServer, gotToken := "", ""
	withFakeDiscovery(t,
		func(port int, timeout time.Duration) *netPulseDiscoveryResult {
			return &netPulseDiscoveryResult{V: 1, Type: "netpulse-server",
				URL: "http://192.168.1.50:3000", Autoenroll: true, PairingToken: "ptok"}
		},
		func(p netpulsePaths, server, token string) error {
			mu.Lock()
			gotServer, gotToken = server, token
			mu.Unlock()
			return nil
		})

	netPulseTryDiscovery(p)
	if !waitEnrolled(t) {
		t.Fatal("debe intentar el enrollment con config incompleta")
	}
	mu.Lock()
	defer mu.Unlock()
	if gotServer != "http://192.168.1.50:3000" || gotToken != "ptok" {
		t.Fatalf("enroll recibió %q/%q", gotServer, gotToken)
	}
	d := netPulseDiscoverySnapshot()
	if d.FoundServer != "http://192.168.1.50:3000" || d.LastDiscovery.IsZero() {
		t.Fatalf("estado de discovery: %+v", d)
	}
}

func TestDiscoverySkipsWhenConnected(t *testing.T) {
	resetDiscoveryState(t)
	p := tmpPaths(t)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://192.168.1.50:3000\nNETPULSE_SLUG=s\nNETPULSE_TOKEN=t\nNETPULSE_ENABLED=1\n")

	var probed atomic.Int32
	withFakeDiscovery(t,
		func(port int, timeout time.Duration) *netPulseDiscoveryResult {
			probed.Add(1)
			return nil
		},
		func(p netpulsePaths, server, token string) error { return nil })

	storeNetPulseStatus(runtime.Status{Running: true, PushOk: true, LastPush: time.Now()})
	netPulseTryDiscovery(p)
	time.Sleep(150 * time.Millisecond)
	if probed.Load() != 0 {
		t.Fatal("conectado: no debe sondear siquiera")
	}
}

func TestDiscoverySkipsSameServer(t *testing.T) {
	resetDiscoveryState(t)
	p := tmpPaths(t)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://192.168.1.50:3000\nNETPULSE_SLUG=s\nNETPULSE_TOKEN=t\nNETPULSE_ENABLED=1\n")

	withFakeDiscovery(t,
		func(port int, timeout time.Duration) *netPulseDiscoveryResult {
			return &netPulseDiscoveryResult{V: 1, Type: "netpulse-server",
				URL: "http://192.168.1.50:3000/", Autoenroll: true, PairingToken: "ptok"}
		},
		func(p netpulsePaths, server, token string) error {
			t.Error("mismo server configurado: no debe re-enrollar")
			return nil
		})

	storeNetPulseStatus(runtime.Status{Running: true, PushOk: false, LastPush: time.Now().Add(-time.Minute)})
	npMu.Lock()
	oldStarted := npStartedAt
	npStartedAt = time.Now().Add(-time.Hour)
	npMu.Unlock()
	t.Cleanup(func() {
		npMu.Lock()
		npStartedAt = oldStarted
		npMu.Unlock()
	})

	netPulseTryDiscovery(p)
	time.Sleep(200 * time.Millisecond)
	if netPulseDiscoverySnapshot().LastDiscovery.IsZero() {
		t.Fatal("el hallazgo sí debe registrarse aunque no se re-enrolle")
	}
	if netPulseDiscoverySnapshot().LastEnrollAt != (time.Time{}) {
		t.Fatal("mismo server configurado: no debe re-enrollar")
	}
}

func TestDiscoveryDifferentServerNeedsStale(t *testing.T) {
	resetDiscoveryState(t)
	p := tmpPaths(t)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://192.168.1.226:3000\nNETPULSE_SLUG=s\nNETPULSE_TOKEN=t\nNETPULSE_ENABLED=1\n")

	enrolled := atomic.Int32{}
	withFakeDiscovery(t,
		func(port int, timeout time.Duration) *netPulseDiscoveryResult {
			return &netPulseDiscoveryResult{V: 1, Type: "netpulse-server",
				URL: "http://192.168.1.50:3000", Autoenroll: true, PairingToken: "ptok"}
		},
		func(p netpulsePaths, server, token string) error {
			enrolled.Add(1)
			return nil
		})

	npMu.Lock()
	oldStarted := npStartedAt
	npStartedAt = time.Now().Add(-time.Hour) // proceso arrancado hace rato
	npMu.Unlock()
	t.Cleanup(func() {
		npMu.Lock()
		npStartedAt = oldStarted
		npMu.Unlock()
	})

	// Push reciente: el server configurado sigue vivo; no se cambia.
	storeNetPulseStatus(runtime.Status{Running: true, PushOk: true, LastPush: time.Now().Add(-time.Minute)})
	netPulseTryDiscovery(p)
	time.Sleep(150 * time.Millisecond)
	if enrolled.Load() != 0 {
		t.Fatal("push reciente: no debe cambiar de server")
	}

	// Sin pushes aceptados durante el periodo stale: sí se re-enrolla.
	storeNetPulseStatus(runtime.Status{Running: true, PushOk: false, LastPush: time.Now().Add(-netPulseStaleAfter - time.Minute)})
	netPulseTryDiscovery(p)
	if !waitEnrolled(t) {
		t.Fatal("server stale: debe re-enrollar contra el descubierto")
	}
	if enrolled.Load() != 1 {
		t.Fatalf("enrollments: %d (esperaba 1)", enrolled.Load())
	}
}

// TestEnrollNetPulseRetrySuffixOnSlugTaken: el pair real contra un server
// HTTP de prueba; primer slug 409 y el sufijo -2 consigue el 201, quedando
// el env persistido con el token nuevo.
func TestEnrollNetPulseRetrySuffixOnSlugTaken(t *testing.T) {
	resetDiscoveryState(t)
	p := tmpPaths(t)

	var mu sync.Mutex
	slugs := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents/pair" || r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		body := make([]byte, 512)
		n, _ := r.Body.Read(body)
		req := string(body[:n])
		mu.Lock()
		defer mu.Unlock()
		for _, s := range slugs {
			if strings.Contains(req, `"slug":"`+s+`"`) {
				// slug ocupado (el server ya lo tiene)
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"slug_taken"}`))
				return
			}
		}
		var slug string
		if i := strings.Index(req, `"slug":"`); i >= 0 {
			rest := req[i+8:]
			slug = rest[:strings.Index(rest, `"`)]
		}
		slugs = append(slugs, slug)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"slug":"` + slug + `","token":"tok-nuevo-64hex","server_fp":"FP01"}`))
	}))
	defer srv.Close()

	// Un slug previo "ocupado" en el server: fuerza el reintento con sufijo.
	mu.Lock()
	slugs = append(slugs, netPulseSanitizeSlug(netPulseHostname()))
	mu.Unlock()

	if err := enrollNetPulse(p, srv.URL, "ptok"); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	cfg, err := ReadNetPulseConfig(p.env)
	if err != nil {
		t.Fatalf("env tras enroll: %v", err)
	}
	if cfg.Server != srv.URL || cfg.Token != "tok-nuevo-64hex" || cfg.ServerFP != "FP01" {
		t.Fatalf("config tras enroll: %+v", cfg)
	}
	if cfg.Slug == slugs[0] {
		t.Fatalf("debe haber reintentado con sufijo: slugs %v", slugs)
	}
	if !strings.HasSuffix(cfg.Slug, "-2") {
		t.Fatalf("segundo intento esperado con sufijo -2: %q", cfg.Slug)
	}
	if !cfg.Enabled {
		t.Fatal("el enrollment deja la config habilitada")
	}
}

// TestEnrollNetPulseRejectsBadToken: 401 aborta sin reintentos.
func TestEnrollNetPulseRejectsBadToken(t *testing.T) {
	resetDiscoveryState(t)
	p := tmpPaths(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_pairing_token"}`))
	}))
	defer srv.Close()

	if err := enrollNetPulse(p, srv.URL, "malo"); err == nil {
		t.Fatal("401 debe devolver error")
	}
	if fileExists(p.env) {
		t.Fatal("con 401 no se escribe nada")
	}
}
