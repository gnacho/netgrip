package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tmpPaths(t *testing.T) netpulsePaths {
	t.Helper()
	root := t.TempDir()
	return netpulsePaths{
		env:           filepath.Join(root, "netgrip", "netpulse.env"),
		standaloneEnv: filepath.Join(root, "netpulse-agent.env"),
		initScript:    filepath.Join(root, "init.d", "netpulse-agent"),
		agentBin:      filepath.Join(root, "sbin", "netpulse-agent"),
		watchdogBin:   filepath.Join(root, "sbin", "netpulse-watchdog"),
		heartbeat:     filepath.Join(root, "tmp", "netpulse-agent.heartbeat"),
		cronFile:      filepath.Join(root, "crontabs", "root"),
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

const sampleStandaloneEnv = `# standalone install
NETPULSE_SERVER=http://192.168.1.226:3000
NETPULSE_TOKEN=deadbeef
NETPULSE_SLUG=patio
NETPULSE_SERVER_FP=AABBCC
NETPULSE_INTERVAL=45
NETPULSE_WAN_TARGET=1.1.1.1
NETPULSE_GW_TARGET=192.168.8.1
`

func TestAdoptMigratesEnvAndRemovesArtifacts(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.standaloneEnv, sampleStandaloneEnv)
	mustWrite(t, p.initScript, "#!/bin/sh\n")
	mustWrite(t, p.agentBin, "fake-binary")

	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	data, err := os.ReadFile(p.env)
	if err != nil {
		t.Fatalf("env nuevo no creado: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"NETPULSE_SERVER=http://192.168.1.226:3000",
		"NETPULSE_TOKEN=deadbeef",
		"NETPULSE_SLUG=patio",
		"NETPULSE_SERVER_FP=AABBCC",
		"NETPULSE_INTERVAL=45",
		"NETPULSE_WAN_TARGET=1.1.1.1",
		"NETPULSE_GW_TARGET=192.168.8.1",
		"NETPULSE_ENABLED=1",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("env migrado sin %s:\n%s", want, content)
		}
	}
	st, _ := os.Stat(p.env)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("permisos env: %o", st.Mode().Perm())
	}

	for path, what := range map[string]string{
		p.standaloneEnv: "env standalone",
		p.initScript:    "init script",
		p.agentBin:      "binario",
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s debe desaparecer tras la migración", what)
		}
	}
}

func TestAdoptIdempotentSecondRun(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.standaloneEnv, sampleStandaloneEnv)
	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	before, err := os.ReadFile(p.env)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	after, err := os.ReadFile(p.env)
	if err != nil {
		t.Fatalf("ReadFile 2: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("segunda pasada modificó el env:\nantes:\n%s\ndespués:\n%s", before, after)
	}
}

func TestAdoptRemovesLeftoversAfterPreviousMigration(t *testing.T) {
	p := tmpPaths(t)
	// Migración previa ya hecha: solo existe el env nuevo, pero reaparecen
	// artefactos standalone (p. ej. reinstall del paquete viejo).
	mustWrite(t, p.env, "NETPULSE_SERVER=http://s\nNETPULSE_TOKEN=t\nNETPULSE_SLUG=s\nNETPULSE_ENABLED=1\n")
	mustWrite(t, p.initScript, "#!/bin/sh\n")
	mustWrite(t, p.agentBin, "fake")

	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if _, err := os.Stat(p.initScript); !os.IsNotExist(err) {
		t.Fatal("init.d leftover debe desaparecer si el env nuevo existe")
	}
	if _, err := os.Stat(p.agentBin); !os.IsNotExist(err) {
		t.Fatal("binario leftover debe desaparecer si el env nuevo existe")
	}
	if !fileExists(p.env) {
		t.Fatal("el env nuevo debe conservarse")
	}
}

func TestAdoptDoesNotOverwriteExistingEnv(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.standaloneEnv, sampleStandaloneEnv)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://otro\nNETPULSE_TOKEN=tk\nNETPULSE_SLUG=otro-slug\nNETPULSE_ENABLED=1\n")

	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	data, _ := os.ReadFile(p.env)
	if !strings.Contains(string(data), "http://otro") {
		t.Fatalf("el env existente NO se sobreescribe:\n%s", data)
	}
	// El env standalone no se consumió en esta pasada: se conserva.
	if !fileExists(p.standaloneEnv) {
		t.Fatal("el env standalone no consumido debe conservarse")
	}
}

func TestAdoptNoopWithoutArtifacts(t *testing.T) {
	p := tmpPaths(t)
	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("adopt sin artefactos: %v", err)
	}
	if fileExists(p.env) {
		t.Fatal("sin env standalone no debe crearse el env nuevo")
	}
}

func TestNetPulseEnvRoundTrip(t *testing.T) {
	p := tmpPaths(t)
	cfg := NetPulseConfig{
		Server: "http://192.168.1.226:3000", Slug: "patio", Token: "tok",
		ServerFP: "AABB", Interval: "15s", WanTarget: "1.1.1.1", GwTarget: "192.168.8.1",
		Enabled: true,
	}
	if err := writeNetPulseEnv(p.env, cfg); err != nil {
		t.Fatalf("writeNetPulseEnv: %v", err)
	}
	got, err := ReadNetPulseConfig(p.env)
	if err != nil {
		t.Fatalf("ReadNetPulseConfig: %v", err)
	}
	if got != cfg {
		t.Fatalf("round trip: got %+v want %+v", got, cfg)
	}

	// Disabled se persiste como 0.
	cfg.Enabled = false
	if err := writeNetPulseEnv(p.env, cfg); err != nil {
		t.Fatalf("writeNetPulseEnv 2: %v", err)
	}
	got, _ = ReadNetPulseConfig(p.env)
	if got.Enabled {
		t.Fatal("NETPULSE_ENABLED=0 debe leerse como disabled")
	}
}

func TestParseNetPulseInterval(t *testing.T) {
	cases := map[string]time.Duration{
		"30":   30 * time.Second,
		"15s":  15 * time.Second,
		"1m":   time.Minute,
		"":     0,
		"nada": 0,
		"-5":   0,
	}
	for in, want := range cases {
		if got := parseNetPulseInterval(in); got != want {
			t.Fatalf("parseNetPulseInterval(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidateNetPulseTarget(t *testing.T) {
	if err := ValidateNetPulseTarget("http://192.168.1.226:3000", "patio"); err != nil {
		t.Fatalf("válido: %v", err)
	}
	if err := ValidateNetPulseTarget("https://np.example.com", "rt-2"); err != nil {
		t.Fatalf("válido https: %v", err)
	}
	if err := ValidateNetPulseTarget("ftp://x", "patio"); err == nil {
		t.Fatal("esquema no http(s) debe fallar")
	}
	if err := ValidateNetPulseTarget("no-url", "patio"); err == nil {
		t.Fatal("no-URL debe fallar")
	}
	if err := ValidateNetPulseTarget("http://s", "Patio"); err == nil {
		t.Fatal("slug con mayúscula debe fallar")
	}
	if err := ValidateNetPulseTarget("http://s", "-patio"); err == nil {
		t.Fatal("slug con guion inicial debe fallar")
	}
}

// TestStartNetPulseAgentKeepsBaseContextAlive: regresión del bug visto en rt3
// (29-Ago-2026). StartNetPulseAgent dejaba npCancel apuntando al cancel del
// ctx base de señales y applyNetPulseAgent lo ejecutaba al "parar el agente
// anterior", cancelando el contexto base: el agente nacía muerto ("context
// canceled" en el primer push). Tras el fix, npCancel solo representa la
// goroutine del agente y el ctx base sobrevive a los rearranques.
func TestStartNetPulseAgentKeepsBaseContextAlive(t *testing.T) {
	if fileExists(prodNetPulsePaths().standaloneEnv) || fileExists(prodNetPulsePaths().env) {
		t.Skip("máquina con config netpulse real: el test no debe tocar artefactos de producción")
	}
	StopNetPulseAgent()
	defer StopNetPulseAgent()

	StartNetPulseAgent("test")

	npMu.Lock()
	base := npBaseCtx
	npMu.Unlock()
	if base == nil {
		t.Fatal("npBaseCtx debe existir tras StartNetPulseAgent")
	}
	select {
	case <-base.Done():
		t.Fatal("el ctx base quedó cancelado tras el arranque: el agente embebido nacería muerto")
	default:
	}
}

func TestApplyNetPulseAgentDisabled(t *testing.T) {
	StopNetPulseAgent()
	defer StopNetPulseAgent()
	p := tmpPaths(t)

	mustWrite(t, p.env, "NETPULSE_SERVER=http://s\nNETPULSE_TOKEN=t\nNETPULSE_SLUG=s\nNETPULSE_ENABLED=0\n")
	applyNetPulseAgent(p)
	npMu.Lock()
	started := npCancel != nil
	npMu.Unlock()
	if started {
		t.Fatal("NETPULSE_ENABLED=0: no debe arrancar el agente")
	}

	mustWrite(t, p.env, "NETPULSE_SERVER=http://s\nNETPULSE_SLUG=s\nNETPULSE_ENABLED=1\n")
	applyNetPulseAgent(p)
	npMu.Lock()
	started = npCancel != nil
	npMu.Unlock()
	if started {
		t.Fatal("config incompleta (sin token): no debe arrancar el agente")
	}
	if NetPulseStatus().Running {
		t.Fatal("status.Running debe ser false sin agente")
	}
}

// TestAdoptRemovesWatchdogAndCron (#141): el cleanup también retira el
// watchdog, su heartbeat y la línea del cron, conservando el resto del
// crontab.
func TestAdoptRemovesWatchdogAndCron(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://s\nNETPULSE_TOKEN=t\nNETPULSE_SLUG=s\nNETPULSE_ENABLED=1\n")
	mustWrite(t, p.watchdogBin, "#!/bin/sh\n")
	mustWrite(t, p.heartbeat, "")
	mustWrite(t, p.cronFile, "0 3 * * * /sbin/logread rotate\n*/1 * * * * /usr/sbin/netpulse-watchdog\n17 4 * * * /usr/sbin/ntpd -q\n")

	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	for path, what := range map[string]string{
		p.watchdogBin: "watchdog",
		p.heartbeat:   "heartbeat",
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s debe desaparecer tras la limpieza", what)
		}
	}
	data, err := os.ReadFile(p.cronFile)
	if err != nil {
		t.Fatalf("crontab debe conservarse: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "netpulse-watchdog") {
		t.Fatalf("la línea del watchdog debe desaparecer del crontab:\n%s", content)
	}
	for _, want := range []string{"/sbin/logread rotate", "/usr/sbin/ntpd -q"} {
		if !strings.Contains(content, want) {
			t.Fatalf("el crontab debe conservar %q:\n%s", want, content)
		}
	}
}

// TestAdoptMigracionCompletaRetiraWatchdogYCron: la migración inicial (env
// standalone presente, sin env nuevo) también limpia watchdog+heartbeat+cron.
func TestAdoptMigracionCompletaRetiraWatchdogYCron(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.standaloneEnv, sampleStandaloneEnv)
	mustWrite(t, p.watchdogBin, "#!/bin/sh\n")
	mustWrite(t, p.heartbeat, "")
	mustWrite(t, p.cronFile, "*/1 * * * * /usr/sbin/netpulse-watchdog\n")

	acted, err := adoptNetPulseStandalone(p)
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if !acted {
		t.Fatal("la migración inicial debe reportar acted=true")
	}
	if fileExists(p.watchdogBin) || fileExists(p.heartbeat) || netPulseCronLineExists(p.cronFile) {
		t.Fatal("watchdog/heartbeat/cron deben desaparecer tras la migración inicial")
	}
	if !fileExists(p.env) {
		t.Fatal("el env nuevo debe existir tras la migración")
	}
}

// TestAdoptWithoutCronFile: sin crontab no pasa nada (mejor esfuerzo).
func TestAdoptWithoutCronFile(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://s\nNETPULSE_TOKEN=t\nNETPULSE_SLUG=s\nNETPULSE_ENABLED=1\n")
	if _, err := adoptNetPulseStandalone(p); err != nil {
		t.Fatalf("adopt sin crontab: %v", err)
	}
	if netPulseCronLineExists(p.cronFile) {
		t.Fatal("cronLineExists debe ser false sin fichero")
	}
}

// TestAdoptReturnsFalseWithoutArtifacts: sin artefactos no reporta acción.
func TestAdoptReturnsFalseWithoutArtifacts(t *testing.T) {
	p := tmpPaths(t)
	acted, err := adoptNetPulseStandalone(p)
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if acted {
		t.Fatal("sin artefactos no debe reportar acted")
	}
}

// TestRecheckCleansReinstalledStandalone (#143): si el standalone reaparece
// (reinstalación manual), la pasada de re-chequeo lo retira de nuevo y queda
// registrada la detección para la UI.
func TestRecheckCleansReinstalledStandalone(t *testing.T) {
	p := tmpPaths(t)
	mustWrite(t, p.env, "NETPULSE_SERVER=http://s\nNETPULSE_TOKEN=t\nNETPULSE_SLUG=s\nNETPULSE_ENABLED=1\n")
	if acted, _ := adoptNetPulseStandalone(p); acted {
		t.Fatal("estado limpio no debe reportar acted")
	}

	// Alguien reinstala el standalone encima (env incluido).
	mustWrite(t, p.standaloneEnv, sampleStandaloneEnv)
	mustWrite(t, p.initScript, "#!/bin/sh\n")
	mustWrite(t, p.agentBin, "fake")
	mustWrite(t, p.watchdogBin, "fake")
	mustWrite(t, p.cronFile, "*/1 * * * * /usr/sbin/netpulse-watchdog\n")

	acted, err := adoptNetPulseStandalone(p)
	if err != nil {
		t.Fatalf("recheck: %v", err)
	}
	if !acted {
		t.Fatal("la reinstalación debe detectarse (acted=true)")
	}
	for path, what := range map[string]string{
		p.initScript:    "init.d",
		p.agentBin:      "binario",
		p.watchdogBin:   "watchdog",
		p.standaloneEnv: "env standalone",
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s reaparecido debe desaparecer tras el recheck", what)
		}
	}
	if netPulseCronLineExists(p.cronFile) {
		t.Fatal("la línea cron debe desaparecer tras el recheck")
	}
	if !fileExists(p.env) {
		t.Fatal("el env de netgrip debe conservarse")
	}
}
