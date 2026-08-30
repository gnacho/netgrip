// fleet_discovery.go: autodiscovery de routers NetGrip en la LAN para la
// sección Flota (#178). Cada router escucha UDP en el puerto de flota
// (default 5141), responde a probes con un beacon público y emite beacons
// periódicos en broadcast. Los peers recibidos se guardan en memoria con TTL;
// la UI muestra los descubiertos y permite adoptarlos con la contraseña root
// del peer (mismo mecanismo que el alta manual).
package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	fleetDiscoveryPort        = 5141
	fleetDiscoveryEvery       = 30 * time.Second
	fleetDiscoveryProbeTimeout = 1500 * time.Millisecond
	fleetDiscoveryTTL         = 5 * time.Minute
	fleetDiscoveryVersion     = 1
)

type fleetBeacon struct {
	V       int    `json:"v"`
	Type    string `json:"type"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type DiscoveredFleetPeer struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Version string    `json:"version"`
	Address string    `json:"address"`
	Port    int       `json:"port"`
	SeenAt  time.Time `json:"seen_at"`
}

var (
	fleetDiscMu       sync.RWMutex
	fleetDiscovered   = make(map[string]*DiscoveredFleetPeer)
	fleetBeaconPort   = 8080
	fleetBeaconID     = ""
	fleetBeaconName   = ""
	fleetBeaconVersion = "dev"
	fleetDiscRunning  bool
)

var fleetIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// sanitizeFleetID normaliza un hostname a un ID válido para la flota.
func sanitizeFleetID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = netPulseSlugStrip.ReplaceAllString(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
		s = strings.Trim(s, "-")
	}
	if !fleetIDRe.MatchString(s) {
		return "netgrip"
	}
	return s
}

// StartFleetDiscovery arranca el listener y el emisor de beacons. Se llama
// una sola vez desde main.
func StartFleetDiscovery(version string, httpPort int) {
	fleetDiscMu.Lock()
	if fleetDiscRunning {
		fleetDiscMu.Unlock()
		return
	}
	fleetDiscRunning = true
	fleetBeaconVersion = version
	fleetBeaconPort = httpPort
	fleetBeaconName = fleetDiscoveryHostname()
	fleetBeaconID = sanitizeFleetID(fleetBeaconName)
	fleetDiscMu.Unlock()

	go fleetDiscoveryListen()
	go fleetDiscoveryAnnounce()
	go fleetDiscoveryCleanup()

	log.Printf("fleet discovery: listening on UDP %d", fleetDiscoveryPort)
}

// fleetDiscoveryHostname devuelve el hostname del router; nunca vacío.
func fleetDiscoveryHostname() string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return "netgrip"
	}
	return h
}

// SetFleetDiscoveryPort permite inyectar el puerto HTTP del panel en tests.
func SetFleetDiscoveryPort(port int) {
	fleetDiscMu.Lock()
	defer fleetDiscMu.Unlock()
	fleetBeaconPort = port
}

// buildBeaconForPeer construye un beacon con la IP local del interfaz que
// comparte red con el peer. Si no se puede determinar, address queda vacío y
// el receptor usará la dirección origen del paquete UDP.
func buildBeaconForPeer(peer net.IP) []byte {
	fleetDiscMu.RLock()
	b := fleetBeacon{
		V:       fleetDiscoveryVersion,
		Type:    "netgrip-fleet-beacon",
		ID:      fleetBeaconID,
		Name:    fleetBeaconName,
		Version: fleetBeaconVersion,
		Port:    fleetBeaconPort,
	}
	fleetDiscMu.RUnlock()
	if peer != nil {
		b.Address = localAddressForPeer(peer.To4())
	}
	data, _ := json.Marshal(b)
	return data
}

// localAddressForPeer devuelve la primera dirección IPv4 local cuya red
// contiene a peer. Vacío si no hay coincidencia.
func localAddressForPeer(peer net.IP) string {
	if peer == nil {
		return ""
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
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
			if ipnet.IP.To4() == nil {
				continue
			}
			if ipnet.Contains(peer) {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// fleetDiscoveryListen escucha probes y beacons UDP. Responde a probes con
// el beacon propio y registra los beacons de otros routers.
func fleetDiscoveryListen() {
	addr := &net.UDPAddr{Port: fleetDiscoveryPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Printf("fleet discovery: listen error: %v", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, peer, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		data := buf[:n]
		if isFleetProbe(data) {
			_, _ = conn.WriteToUDP(buildBeaconForPeer(peer.IP), peer)
			continue
		}
		recordFleetBeacon(peer.IP, data)
	}
}

// isFleetProbe verifica si un payload es un probe válido.
func isFleetProbe(data []byte) bool {
	var hdr struct {
		V    int    `json:"v"`
		Type string `json:"type"`
	}
	return json.Unmarshal(data, &hdr) == nil &&
		hdr.V == fleetDiscoveryVersion &&
		hdr.Type == "netgrip-fleet-probe"
}

// fleetDiscoveryAnnounce emite beacons periódicos en broadcast por cada
// interfaz IPv4 disponible.
func fleetDiscoveryAnnounce() {
	t := time.NewTicker(fleetDiscoveryEvery)
	defer t.Stop()
	for range t.C {
		broadcastFleetBeacon()
	}
}

// broadcastFleetBeacon envía un beacon a las direcciones de broadcast de
// cada interfaz IPv4 con capacidad de broadcast. El beacon incluye la IP
// del interfaz para que los peers la muestren sin depender de la dirección
// origen del paquete.
func broadcastFleetBeacon() {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	base := func() fleetBeacon {
		fleetDiscMu.RLock()
		defer fleetDiscMu.RUnlock()
		return fleetBeacon{
			V:       fleetDiscoveryVersion,
			Type:    "netgrip-fleet-beacon",
			ID:      fleetBeaconID,
			Name:    fleetBeaconName,
			Version: fleetBeaconVersion,
			Port:    fleetBeaconPort,
		}
	}()

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return
	}
	defer conn.Close()

	targets := map[string]net.IP{}
	ifaceAddrs := map[string]net.IP{}
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
			for i := 0; i < 4; i++ {
				bcast[i] = ip4[i] | ^ipnet.Mask[i]
			}
			key := string(bcast)
			targets[key] = net.IP(bcast)
			ifaceAddrs[key] = ip4
		}
	}
	targets[string([]byte{255, 255, 255, 255})] = net.IPv4(255, 255, 255, 255)

	for key, ip := range targets {
		b := base
		if addr, ok := ifaceAddrs[key]; ok {
			b.Address = addr.String()
		}
		data, _ := json.Marshal(b)
		_, _ = conn.WriteTo(data, &net.UDPAddr{IP: ip, Port: fleetDiscoveryPort})
	}
}

// fleetDiscoveryCleanup elimina peers que no se han visto en el TTL.
func fleetDiscoveryCleanup() {
	t := time.NewTicker(fleetDiscoveryEvery)
	defer t.Stop()
	for range t.C {
		fleetDiscMu.Lock()
		for id, p := range fleetDiscovered {
			if time.Since(p.SeenAt) > fleetDiscoveryTTL {
				delete(fleetDiscovered, id)
			}
		}
		fleetDiscMu.Unlock()
	}
}

// recordFleetBeacon procesa un beacon recibido y lo guarda en memoria.
func recordFleetBeacon(src net.IP, data []byte) {
	var b fleetBeacon
	if err := json.Unmarshal(data, &b); err != nil {
		return
	}
	if b.V != fleetDiscoveryVersion || b.Type != "netgrip-fleet-beacon" || b.ID == "" {
		return
	}
	if b.ID == fleetBeaconID {
		return // ignorar el propio beacon reflejado
	}

	addr := b.Address
	if addr == "" {
		addr = src.String()
	}
	if b.Port == 0 {
		b.Port = 8080
	}

	fleetDiscMu.Lock()
	fleetDiscovered[b.ID] = &DiscoveredFleetPeer{
		ID:      b.ID,
		Name:    b.Name,
		Version: b.Version,
		Address: addr,
		Port:    b.Port,
		SeenAt:  time.Now(),
	}
	fleetDiscMu.Unlock()
}

// probeFleetPeers emite un probe y escucha respuestas durante un timeout.
// Se usa principalmente en tests y para refrescar manualmente la lista.
func probeFleetPeers(timeout time.Duration) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return
	}
	defer conn.Close()

	probe := []byte(`{"v":1,"type":"netgrip-fleet-probe"}`)
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
				for i := 0; i < 4; i++ {
					bcast[i] = ip4[i] | ^ipnet.Mask[i]
				}
				_, _ = conn.WriteTo(probe, &net.UDPAddr{IP: net.IP(bcast), Port: fleetDiscoveryPort})
			}
		}
	}
	_, _ = conn.WriteTo(probe, &net.UDPAddr{IP: net.IPv4(255, 255, 255, 255), Port: fleetDiscoveryPort})

	deadline := time.Now().Add(timeout)
	buf := make([]byte, 1024)
	for {
		_ = conn.SetReadDeadline(deadline)
		n, src, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		recordFleetBeacon(src.(*net.UDPAddr).IP, buf[:n])
	}
}

// ListDiscoveredFleetPeers devuelve los routers NetGrip descubiertos que aún
// no están en la flota.
func ListDiscoveredFleetPeers() ([]DiscoveredFleetPeer, error) {
	cfg, err := LoadFleetConfig()
	if err != nil {
		return nil, err
	}
	adopted := make(map[string]bool, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		adopted[n.ID] = true
	}

	fleetDiscMu.RLock()
	out := make([]DiscoveredFleetPeer, 0, len(fleetDiscovered))
	for _, p := range fleetDiscovered {
		if adopted[p.ID] {
			continue
		}
		out = append(out, *p)
	}
	fleetDiscMu.RUnlock()

	return out, nil
}

// AdoptFleetPeer añade un peer descubierto a la flota y verifica que podemos
// loguearnos en él antes de persistirlo.
func AdoptFleetPeer(id, name, address, password string) error {
	if id == "" || name == "" || address == "" || password == "" {
		return fmt.Errorf("missing fields")
	}
	if !fleetIDRe.MatchString(id) {
		return fmt.Errorf("invalid id")
	}

	node := FleetNode{ID: id, Name: name, Address: address, Password: password}
	status := checkNodeUpdate(node)
	if !status.Reachable {
		return fmt.Errorf("unreachable: %s", status.Error)
	}

	return AddFleetNode(node)
}

// isFleetProbeDuplicate: helper para tests.
func isFleetProbeDuplicate(a, b []byte) bool {
	return bytes.Equal(a, b)
}
