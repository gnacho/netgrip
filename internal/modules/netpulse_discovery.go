// netpulse_discovery.go: descubrimiento zero-touch del server NetPulse
// (#147). Cuando el agente embebido NO está conectado, se emite un probe
// UDP en broadcast al puerto de beacons del server (default 5140); el
// server responde unicast con su URL HTTP y, si tiene AGENT_AUTOENROLL=1,
// un token de alta de red. Con él se llama al pairing existente
// (POST /api/agents/pair) usando como slug el hostname sanitizado; si el
// slug ya existe en el server (409 slug_taken) se reintenta con sufijos
// -2..-5. Sin dependencias nuevas: net + net/http del stdlib.
package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gnacho/netpulse/agent/runtime"
)

const (
	// netPulseDiscoveryEvery: cadencia del ciclo de descubrimiento.
	netPulseDiscoveryEvery = 30 * time.Second
	// netPulseProbeTimeout: espera de respuesta unicast del server.
	netPulseProbeTimeout = 1500 * time.Millisecond
	// netPulseStaleAfter: sin pushes aceptados durante este tiempo (y con
	// el agente arrancado al menos lo mismo) se considera que el server
	// configurado se movió: si el descubrimiento encuentra OTRO server, se
	// re-enrolla contra él.
	netPulseStaleAfter = 10 * time.Minute
	// netPulseSlugAttempts: base + sufijos -2..-5 ante slug_taken.
	netPulseSlugAttempts = 5
)

// netPulseDiscoveryDefaultPort es el puerto UDP del listener de beacons del
// server NetPulse (NETPULSE_BEACON_LISTEN en su config).
const netPulseDiscoveryDefaultPort = 5140

// netPulseDiscoveryResult es la respuesta del server a un probe.
type netPulseDiscoveryResult struct {
	V            int    `json:"v"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	Autoenroll   bool   `json:"autoenroll"`
	PairingToken string `json:"pairing_token"`
}

// netPulseDiscoveryState expone el último hallazgo para la UI/API.
type netPulseDiscoveryState struct {
	FoundServer    string    // URL del último server que respondió
	LastDiscovery  time.Time // momento del último probe con respuesta
	LastEnrollAt   time.Time // último intento de enrollment
	LastEnrollNote string    // resultado del último enrollment ("", ok o error)
}

var (
	npDiscMu    sync.Mutex
	npDisc      netPulseDiscoveryState
	npEnrolling bool
	npStartedAt time.Time
	// inyectables en tests
	npProbe  func(port int, timeout time.Duration) *netPulseDiscoveryResult = probeNetPulseServers
	npEnroll func(p netpulsePaths, server, pairingToken string) error       = enrollNetPulse
)

var netPulseSlugStrip = regexp.MustCompile(`[^a-z0-9-]+`)

// netPulseSanitizeSlug normaliza un hostname a slug válido
// (^[a-z0-9][a-z0-9-]{0,63}$): minúsculas, colapsa caracteres raros a
// guiones, recorta y acorta a 64. Vacío → "netgrip".
func netPulseSanitizeSlug(hostname string) string {
	s := strings.ToLower(strings.TrimSpace(hostname))
	s = netPulseSlugStrip.ReplaceAllString(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
		s = strings.Trim(s, "-")
	}
	if !netPulseRe.MatchString(s) {
		return "netgrip"
	}
	return s
}

// netPulseHostname: hostname del router (fallback "netgrip", mismo criterio
// que el CN del certificado de acceso remoto).
func netPulseHostname() string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return "netgrip"
	}
	return h
}

// netPulseDiscoveryPort lee NETPULSE_DISCOVERY_PORT del env file (default
// 5140).
func netPulseDiscoveryPort(p netpulsePaths) int {
	cfg, err := ReadNetPulseConfig(p.env)
	if err == nil && cfg.DiscoveryPort != "" {
		if n, perr := strconv.Atoi(strings.TrimSpace(cfg.DiscoveryPort)); perr == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return netPulseDiscoveryDefaultPort
}

// probeNetPulseServers emite el probe en broadcast por cada interfaz IPv4
// con broadcast y espera la primera respuesta válida. Devuelve nil si nadie
// contesta dentro del timeout.
func probeNetPulseServers(port int, timeout time.Duration) *netPulseDiscoveryResult {
	pc, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil
	}
	defer pc.Close()
	probe := []byte(`{"v":1,"type":"netgrip-probe"}`)
	targets := map[string]bool{}
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifi := range ifaces {
			if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 || ifi.Flags&net.FlagBroadcast == 0 {
				continue
			}
			addrs, err := ifi.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				ipnet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				ip4 := ipnet.IP.To4()
				if ip4 == nil || ipnet.Mask == nil || len(ipnet.Mask) != 4 {
					continue
				}
				bcast := make([]byte, 4)
				// broadcast = ip | ~máscara (192.168.1.5/24 → 192.168.1.255)
				for i := 0; i < 4; i++ {
					bcast[i] = ip4[i] | ^ipnet.Mask[i]
				}
				targets[string(bcast)] = true
			}
		}
	}
	targets[string([]byte{255, 255, 255, 255})] = true
	for raw := range targets {
		ip := net.IP([]byte(raw))
		if _, err := pc.WriteTo(probe, &net.UDPAddr{IP: ip, Port: port}); err != nil {
			continue
		}
	}
	buf := make([]byte, 1024)
	deadline := time.Now().Add(timeout)
	for {
		_ = pc.SetReadDeadline(deadline)
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return nil // timeout sin respuestas
		}
		var hdr struct {
			V    int    `json:"v"`
			Type string `json:"type"`
		}
		if json.Unmarshal(buf[:n], &hdr) != nil || hdr.Type != "netpulse-server" || hdr.V != 1 {
			continue
		}
		var r netPulseDiscoveryResult
		if err := json.Unmarshal(buf[:n], &r); err != nil || !netPulseValidServerURL(r.URL) {
			continue
		}
		return &r
	}
}

// netPulseValidServerURL: http/https con host.
func netPulseValidServerURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// netPulseDiscoveryLoop corre mientras viva el proceso: si el agente está
// conectado no hace nada; si no, probe + (si procede) enrollment.
func netPulseDiscoveryLoop(p netpulsePaths, every time.Duration) {
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
			if netPulseIsConnected() {
				continue
			}
			netPulseTryDiscovery(p)
		}
	}
}

// netPulseTryDiscovery: un ciclo de probe + política de enrollment. Si el
// agente ya está conectado no hace nada (ni siquiera sondea).
func netPulseTryDiscovery(p netpulsePaths) {
	if netPulseIsConnected() {
		return
	}
	res := npProbe(netPulseDiscoveryPort(p), netPulseProbeTimeout)
	if res == nil {
		return
	}
	npDiscMu.Lock()
	npDisc.FoundServer = res.URL
	npDisc.LastDiscovery = time.Now()
	npDiscMu.Unlock()

	if !res.Autoenroll || res.PairingToken == "" {
		log.Printf("netpulse: discovery encontró %s (autoenroll desactivado en el server)", res.URL)
		return
	}

	cfg, err := ReadNetPulseConfig(p.env)
	if err != nil {
		cfg = NetPulseConfig{}
	}
	complete := cfg.Server != "" && cfg.Slug != "" && cfg.Token != ""
	if complete && strings.TrimRight(cfg.Server, "/") == strings.TrimRight(res.URL, "/") {
		return // mismo server configurado: el agente reintentará el push
	}
	if complete {
		// Server configurado distinto del descubierto: solo se cambia si
		// el configurado lleva netPulseStaleWithoutPush sin aceptar pushes
		// (cubren el caso de IP cambiada por DHCP).
		npMu.Lock()
		lastPush := npStatus.LastPush
		started := npStartedAt
		npMu.Unlock()
		if time.Since(started) < netPulseStaleAfter || time.Since(lastPush) < netPulseStaleAfter {
			return
		}
	}

	npDiscMu.Lock()
	already := npEnrolling
	npDiscMu.Unlock()
	if already {
		return
	}
	npDiscMu.Lock()
	npEnrolling = true
	npDiscMu.Unlock()
	go func() {
		defer func() {
			npDiscMu.Lock()
			npEnrolling = false
			npDiscMu.Unlock()
		}()
		err := npEnroll(p, res.URL, res.PairingToken)
		npDiscMu.Lock()
		npDisc.LastEnrollAt = time.Now()
		if err != nil {
			npDisc.LastEnrollNote = err.Error()
		} else {
			npDisc.LastEnrollNote = ""
		}
		npDiscMu.Unlock()
		if err != nil {
			log.Printf("netpulse: enrollment contra %s falló: %v", res.URL, err)
		}
	}()
}

// enrollNetPulse: pairing contra el server descubierto con slug derivado
// del hostname y reintentos con sufijo si el slug está ocupado. Al conseguir
// token persiste la config completa (conserva interval/targets previos).
func enrollNetPulse(p netpulsePaths, server, pairingToken string) error {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	base := netPulseSanitizeSlug(netPulseHostname())
	var lastErr error
	for i := 0; i < netPulseSlugAttempts; i++ {
		slug := base
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, i+1)
		}
		body := fmt.Sprintf(`{"pairing_token":%q,"slug":%q}`, pairingToken, slug)
		req, err := http.NewRequest("POST", server+"/api/agents/pair", strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		hc := &http.Client{Timeout: 10 * time.Second}
		res, err := hc.Do(req)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		res.Body.Close()
		switch res.StatusCode {
		case http.StatusCreated:
			var pr struct {
				Slug     string `json:"slug"`
				Token    string `json:"token"`
				ServerFP string `json:"server_fp"`
			}
			if err := json.Unmarshal(data, &pr); err != nil || pr.Token == "" {
				return fmt.Errorf("respuesta de pairing inválida: %s", strings.TrimSpace(string(data)))
			}
			old, _ := ReadNetPulseConfig(p.env)
			cfg := NetPulseConfig{
				Server:        server,
				Slug:          pr.Slug,
				Token:         pr.Token,
				ServerFP:      pr.ServerFP,
				Interval:      old.Interval,
				WanTarget:     old.WanTarget,
				GwTarget:      old.GwTarget,
				Enabled:       true,
				DiscoveryPort: old.DiscoveryPort,
			}
			if err := setNetPulseConfigAt(p, cfg); err != nil {
				return fmt.Errorf("guardar config: %w", err)
			}
			log.Printf("netpulse: auto-enrolled en %s como %s (zero-touch)", server, pr.Slug)
			return nil
		case http.StatusConflict:
			lastErr = fmt.Errorf("slug %s ya existe en %s", slug, server)
			continue // siguiente sufijo
		case http.StatusUnauthorized:
			return fmt.Errorf("pairing rechazado (401): token de alta inválido")
		default:
			return fmt.Errorf("pairing HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
		}
	}
	return lastErr
}

// netPulseIsConnected lee el estado compartido y delega en la regla pura.
func netPulseIsConnected() bool {
	npMu.Lock()
	st := npStatus
	npMu.Unlock()
	return netPulseStatusConnected(st)
}

// netPulseStatusConnected: agente corriendo, último push OK y reciente
// (3 intervalos por defecto = 90 s).
func netPulseStatusConnected(st runtime.Status) bool {
	if !st.Running || !st.PushOk || st.LastPush.IsZero() {
		return false
	}
	return time.Since(st.LastPush) < 3*runtime.DefaultInterval
}

// noteNetPulseDiscovery expone el estado de discovery (API/UI).
func netPulseDiscoverySnapshot() netPulseDiscoveryState {
	npDiscMu.Lock()
	defer npDiscMu.Unlock()
	return npDisc
}
