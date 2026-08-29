// netpulse_agent.go: agente NetPulse embebido (#140). NetGrip puede correr
// el bucle del agente dentro de su propio proceso usando el paquete público
// github.com/gnacho/netpulse/agent/runtime, con config propia en
// /etc/netgrip/netpulse.env (mismo formato KEY=VALUE que el standalone).
// Al arrancar adopta una instalación standalone previa: migra el env,
// apaga el servicio viejo y borra sus artefactos (idempotente).
package modules

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gnacho/netpulse/agent/runtime"
)

// netpulsePaths agrupa las rutas que usa la adopción (inyectables en tests).
type netpulsePaths struct {
	env           string // /etc/netgrip/netpulse.env
	standaloneEnv string // /etc/netpulse-agent.env
	initScript    string // /etc/init.d/netpulse-agent
	agentBin      string // /usr/sbin/netpulse-agent
	watchdogBin   string // /usr/sbin/netpulse-watchdog
	heartbeat     string // /tmp/netpulse-agent.heartbeat
	cronFile      string // /etc/crontabs/root
}

func prodNetPulsePaths() netpulsePaths {
	return netpulsePaths{
		env:           "/etc/netgrip/netpulse.env",
		standaloneEnv: "/etc/netpulse-agent.env",
		initScript:    "/etc/init.d/netpulse-agent",
		agentBin:      "/usr/sbin/netpulse-agent",
		watchdogBin:   "/usr/sbin/netpulse-watchdog",
		heartbeat:     "/tmp/netpulse-agent.heartbeat",
		cronFile:      "/etc/crontabs/root",
	}
}

// NetPulseConfig es la config persistida en el env file.
type NetPulseConfig struct {
	Server    string
	Slug      string
	Token     string
	ServerFP  string
	Interval  string // "30", "15s", "1m"
	WanTarget string
	GwTarget  string
	Enabled   bool
}

// NetPulseInfo es la vista de solo lectura para la API (nunca el token).
type NetPulseInfo struct {
	Enabled              bool
	Configured           bool // server+slug+token presentes
	Server               string
	Slug                 string
	Status               runtime.Status
	StandaloneReplacedAt time.Time // última detección+sustitución del standalone
}

var (
	npMu                 sync.Mutex
	npBaseCtx            context.Context
	npBaseCancel         context.CancelFunc // cancela el ctx de señales (parada del proceso)
	npCancel             context.CancelFunc // cancela SOLO la goroutine del agente vigente
	npStatus             runtime.Status
	npVersion            string
	npStarted            bool
	npStandaloneReplaced time.Time
	netPulseRe           = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// noteNetPulseStandaloneReplaced registra una detección del standalone (para
// que la UI pueda avisar de que fue sustituido).
func noteNetPulseStandaloneReplaced() {
	npMu.Lock()
	npStandaloneReplaced = time.Now()
	npMu.Unlock()
}

// StartNetPulseAgent arranca el agente embebido (llamar una vez desde main).
// Adopta una instalación standalone previa y, si la config está habilitada y
// es válida, corre runtime.Run en una goroutine que muere con SIGTERM/SIGINT.
func StartNetPulseAgent(version string) {
	npMu.Lock()
	if npStarted {
		npMu.Unlock()
		return
	}
	npStarted = true
	npVersion = version
	// npBaseCancel vive aparte de npCancel: applyNetPulseAgent cancela npCancel
	// para rearrancar la goroutine y NUNCA debe tumbar el ctx base de señales
	// (bug encontrado en rt3: el primer arranque nacía ya cancelado).
	npBaseCtx, npBaseCancel = signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	npMu.Unlock()

	if acted, err := AdoptStandaloneAgent(); err != nil {
		log.Printf("netpulse: adopt standalone agent: %v", err)
	} else if acted {
		noteNetPulseStandaloneReplaced()
	}
	applyNetPulseAgent(prodNetPulsePaths())
	go netPulseRecheckLoop(prodNetPulsePaths(), 60*time.Second)
}

// netPulseRecheckLoop re-comprueba periódicamente si el standalone ha
// reaparecido (reinstalación manual) y re-lanza la adopción. Barato: sin
// artefactos es solo un par de stat por pasada.
func netPulseRecheckLoop(p netpulsePaths, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		npMu.Lock()
		base := npBaseCtx
		npMu.Unlock()
		if base == nil {
			return
		}
		select {
		case <-base.Done():
			return
		case <-t.C:
			if acted, err := adoptNetPulseStandalone(p); err != nil {
				log.Printf("netpulse: standalone recheck: %v", err)
			} else if acted {
				log.Printf("netpulse: standalone agent reappeared; adopted and removed it again")
				noteNetPulseStandaloneReplaced()
			}
		}
	}
}

// StopNetPulseAgent para la goroutine del agente y el ctx de señales (tests y
// apagado limpio).
func StopNetPulseAgent() {
	npMu.Lock()
	defer npMu.Unlock()
	if npCancel != nil {
		npCancel()
		npCancel = nil
	}
	if npBaseCancel != nil {
		npBaseCancel()
		npBaseCancel = nil
	}
	npStatus.Running = false
}

// NetPulseStatus devuelve una copia del último estado del agente.
func NetPulseStatus() runtime.Status {
	npMu.Lock()
	defer npMu.Unlock()
	return npStatus
}

// NetPulseInfoNow compone la vista de la API desde el env file + estado.
func NetPulseInfoNow() NetPulseInfo {
	cfg, err := ReadNetPulseConfig(prodNetPulsePaths().env)
	if err != nil {
		cfg = NetPulseConfig{}
	}
	npMu.Lock()
	replaced := npStandaloneReplaced
	npMu.Unlock()
	return NetPulseInfo{
		Enabled:              cfg.Enabled,
		Configured:           cfg.Server != "" && cfg.Slug != "" && cfg.Token != "",
		Server:               cfg.Server,
		Slug:                 cfg.Slug,
		Status:               NetPulseStatus(),
		StandaloneReplacedAt: replaced,
	}
}

// ReadNetPulseConfig parsea el env file de NetGrip (KEY=VALUE).
func ReadNetPulseConfig(path string) (NetPulseConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return NetPulseConfig{}, err
	}
	kv := parseNetPulseEnv(string(data))
	return NetPulseConfig{
		Server:    kv["NETPULSE_SERVER"],
		Slug:      kv["NETPULSE_SLUG"],
		Token:     kv["NETPULSE_TOKEN"],
		ServerFP:  kv["NETPULSE_SERVER_FP"],
		Interval:  kv["NETPULSE_INTERVAL"],
		WanTarget: kv["NETPULSE_WAN_TARGET"],
		GwTarget:  kv["NETPULSE_GW_TARGET"],
		Enabled:   kv["NETPULSE_ENABLED"] == "1",
	}, nil
}

func parseNetPulseEnv(data string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

// ValidateNetPulseTarget comprueba server (URL http/https) y slug
// (^[a-z0-9][a-z0-9-]*$) para el POST de la API.
func ValidateNetPulseTarget(server, slug string) error {
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("server must be an http(s) URL")
	}
	if !netPulseRe.MatchString(slug) {
		return fmt.Errorf("slug must match ^[a-z0-9][a-z0-9-]*$")
	}
	return nil
}

// SetNetPulseConfig persiste la config (token vacío conserva el actual) y
// rearranca la goroutine del agente sin reiniciar el proceso.
func SetNetPulseConfig(cfg NetPulseConfig) error {
	p := prodNetPulsePaths()
	if cfg.Token == "" {
		if old, err := ReadNetPulseConfig(p.env); err == nil {
			cfg.Token = old.Token
		}
	}
	if err := writeNetPulseEnv(p.env, cfg); err != nil {
		return err
	}
	applyNetPulseAgent(p)
	return nil
}

func writeNetPulseEnv(path string, cfg NetPulseConfig) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# managed by netgrip; netpulse embedded agent config\n")
	b.WriteString("NETPULSE_SERVER=" + cfg.Server + "\n")
	b.WriteString("NETPULSE_SLUG=" + cfg.Slug + "\n")
	if cfg.Token != "" {
		b.WriteString("NETPULSE_TOKEN=" + cfg.Token + "\n")
	}
	if cfg.ServerFP != "" {
		b.WriteString("NETPULSE_SERVER_FP=" + cfg.ServerFP + "\n")
	}
	if cfg.Interval != "" {
		b.WriteString("NETPULSE_INTERVAL=" + cfg.Interval + "\n")
	}
	if cfg.WanTarget != "" {
		b.WriteString("NETPULSE_WAN_TARGET=" + cfg.WanTarget + "\n")
	}
	if cfg.GwTarget != "" {
		b.WriteString("NETPULSE_GW_TARGET=" + cfg.GwTarget + "\n")
	}
	enabled := "0"
	if cfg.Enabled {
		enabled = "1"
	}
	b.WriteString("NETPULSE_ENABLED=" + enabled + "\n")
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func dirOf(path string) string {
	if i := strings.LastIndexByte(path, '/'); i > 0 {
		return path[:i]
	}
	return "."
}

// AdoptStandaloneAgent migra una instalación standalone previa (idempotente):
// si existe /etc/netpulse-agent.env y NO /etc/netgrip/netpulse.env, escribe la
// config nueva conservando las keys soportadas y añade NETPULSE_ENABLED=1;
// después apaga y borra el standalone (mejor esfuerzo). Si la migración ya
// ocurrió antes (solo existe el env nuevo), también retira artefactos
// standalone que hayan quedado (init.d, binario, watchdog, cron). Devuelve
// true cuando detectó (y retiró) artefactos del standalone.
func AdoptStandaloneAgent() (bool, error) {
	return adoptNetPulseStandalone(prodNetPulsePaths())
}

func adoptNetPulseStandalone(p netpulsePaths) (bool, error) {
	standaloneEnvExists := fileExists(p.standaloneEnv)
	envExists := fileExists(p.env)

	if standaloneEnvExists && !envExists {
		cfg, err := ReadNetPulseConfig(p.standaloneEnv)
		if err != nil {
			return false, fmt.Errorf("parse %s: %w", p.standaloneEnv, err)
		}
		cfg.Enabled = true
		if err := writeNetPulseEnv(p.env, cfg); err != nil {
			return false, fmt.Errorf("write %s: %w", p.env, err)
		}
		log.Printf("netpulse: migrated standalone config %s -> %s (enabled)", p.standaloneEnv, p.env)
		cleanupNetPulseStandalone(p, true)
		return true, nil
	}

	if envExists {
		// Migración previa (o config creada desde la UI): retirar restos del
		// standalone si volvieran a aparecer. El env standalone se retira
		// junto al resto (la reinstalación lo trae de nuevo); si lo único
		// que hay es un env suelto sin artefactos, se conserva.
		if netPulseStandaloneArtifacts(p) {
			cleanupNetPulseStandalone(p, fileExists(p.standaloneEnv))
			return true, nil
		}
	}
	return false, nil
}

// netPulseStandaloneArtifacts dice si hay artefactos del standalone (init.d,
// binario, watchdog o línea cron) presentes. El heartbeat de /tmp NO cuenta:
// también lo escribe el agente embebido en cada push, así que solo se retira
// como parte del cleanup cuando hay standalone real.
func netPulseStandaloneArtifacts(p netpulsePaths) bool {
	return fileExists(p.initScript) ||
		fileExists(p.agentBin) ||
		fileExists(p.watchdogBin) ||
		netPulseCronLineExists(p.cronFile)
}

// netPulseCronLineExists: true si el crontab contiene una línea del watchdog.
func netPulseCronLineExists(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), "netpulse-watchdog")
}

// removeNetPulseCronLine borra del crontab las líneas del watchdog del
// standalone conservando el resto; devuelve true si modificó el fichero.
func removeNetPulseCronLine(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "netpulse-watchdog") {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	if out == string(data) {
		return false
	}
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(out), mode); err != nil {
		log.Printf("netpulse: clean cron %s: %v", path, err)
		return false
	}
	return true
}

// cleanupNetPulseStandalone retira el servicio y artefactos standalone (init.d,
// binario, watchdog, heartbeat y línea cron), todo mejor esfuerzo. withEnv
// incluye el env file viejo (solo cuando la migración de esta misma pasada lo
// dejó copiado a /etc/netgrip).
func cleanupNetPulseStandalone(p netpulsePaths, withEnv bool) {
	if fileExists(p.initScript) {
		if out, err := exec.Command(p.initScript, "stop").CombinedOutput(); err != nil {
			log.Printf("netpulse: init stop: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.Command(p.initScript, "disable").CombinedOutput(); err != nil {
			log.Printf("netpulse: init disable: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		if err := os.Remove(p.initScript); err != nil {
			log.Printf("netpulse: remove %s: %v", p.initScript, err)
		} else {
			log.Printf("netpulse: removed %s", p.initScript)
		}
	}
	for _, bin := range []string{p.agentBin, p.watchdogBin, p.heartbeat} {
		if err := os.Remove(bin); err == nil {
			log.Printf("netpulse: removed %s", bin)
		}
	}
	if removeNetPulseCronLine(p.cronFile) {
		log.Printf("netpulse: removed netpulse-watchdog cron line from %s", p.cronFile)
	}
	if withEnv {
		if err := os.Remove(p.standaloneEnv); err == nil {
			log.Printf("netpulse: removed %s (migrated)", p.standaloneEnv)
		}
	}
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// applyNetPulseAgent (re)arranca la goroutine del agente según el env file:
// para la anterior, y si la config está habilitada y es válida lanza
// runtime.Run con un ctx derivado del base (SIGTERM) del proceso.
func applyNetPulseAgent(p netpulsePaths) {
	npMu.Lock()
	if npCancel != nil {
		npCancel()
		npCancel = nil
	}
	npStatus = runtime.Status{}
	base := npBaseCtx
	version := npVersion
	npMu.Unlock()
	if base == nil {
		base = context.Background()
	}

	cfg, err := ReadNetPulseConfig(p.env)
	if err != nil {
		return // sin config: agente desactivado
	}

	if !cfg.Enabled {
		log.Printf("netpulse: agent disabled (NETPULSE_ENABLED=0)")
		return
	}
	if cfg.Server == "" || cfg.Slug == "" || cfg.Token == "" {
		log.Printf("netpulse: config incomplete (server/slug/token), agent not started")
		return
	}

	opts := runtime.Options{
		Server:    cfg.Server,
		Token:     cfg.Token,
		Slug:      cfg.Slug,
		ServerFP:  cfg.ServerFP,
		Interval:  parseNetPulseInterval(cfg.Interval),
		WanTarget: cfg.WanTarget,
		GwTarget:  cfg.GwTarget,
		EnvFile:   p.env,
		Version:   version,
		Kind:      "netgrip",
		OnStatus:  storeNetPulseStatus,
		OnUpgrade: netPulseUpgradeTrigger,
	}

	ctx, cancel := context.WithCancel(base)
	npMu.Lock()
	npCancel = cancel
	npMu.Unlock()

	interval := opts.Interval
	if interval <= 0 {
		interval = runtime.DefaultInterval
	}
	log.Printf("netpulse: embedded agent starting (slug=%s server=%s interval=%s)", opts.Slug, opts.Server, interval)
	go func() {
		if err := runtime.Run(ctx, opts); err != nil {
			log.Printf("netpulse: agent stopped: %v", err)
			npMu.Lock()
			npStatus.Running = false
			npMu.Unlock()
		}
	}()
}

// parseNetPulseInterval acepta "30" (segundos) o duraciones ("15s", "1m");
// vacío/inválido devuelve 0 (runtime aplicará su default de 30s).
func parseNetPulseInterval(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	return 0
}

func storeNetPulseStatus(st runtime.Status) {
	npMu.Lock()
	npStatus = st
	npMu.Unlock()
}

// netPulseUpgradeTrigger: evento SSE "upgrade" del servidor NetPulse (#363):
// NetGrip se actualiza a sí mismo con su propio updater (sus releases de
// GitHub, sha verificado); el servidor solo dispara. Si ya está en la última,
// no hace nada (el guard de downgrades vive en el self-update).
func netPulseUpgradeTrigger(data string) {
	check := CheckSelfUpdate(npVersion)
	if check == nil || !check.Available {
		log.Printf("netpulse: upgrade solicitado por el servidor: ya en la última (%s)", npVersion)
		return
	}
	log.Printf("netpulse: upgrade solicitado por el servidor → %s", check.Latest)
	if err := StartSelfUpdate(npVersion); err != nil {
		log.Printf("netpulse: self-update falló: %v", err)
	}
}
