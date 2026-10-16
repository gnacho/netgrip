package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Espejo de puertos (#443): copia todo el trafico de una o varias bocas
// origen hacia una boca monitor con la accion mirred de tc, en los dos
// sentidos. La persistencia vive en /etc/netgrip/mirror.conf y el hook
// /usr/libexec/netgrip-restore-rules lo re-aplica en el arranque.
//
// Verificado en el LGS352C (rtl931x, DSA): mirred entra en hardware
// ("in_hw" en tc filter show) tanto en ingress como en egress con el
// qdisc clsact. En drivers sin mirred el probe lo reporta y la UI muestra
// "no soportado por este dispositivo".

const mirrorConfPath = "/etc/netgrip/mirror.conf"

type MirrorPort struct {
	Port          string `json:"port"`
	LinkSpeedMbps int    `json:"link_speed_mbps"`
	// Mirrored: la boca esta en mirror.conf como origen.
	Mirrored bool `json:"mirrored"`
	// Active: los filtros tc del espejo estan presentes en la boca.
	Active bool `json:"active"`
}

type MirrorProbe struct {
	Applicable      bool         `json:"applicable"`
	Ports           []MirrorPort `json:"ports"`
	TcInstalled     bool         `json:"tc_installed"`
	MirrorSupported bool         `json:"mirror_supported"`
	// ManagementPort es una HEURISTICA: la boca por la que el equipo
	// alcanza su gateway por defecto (la del uplink, tipicamente la de
	// gestion). La UI la usa para avisar antes de espejarla; puede estar
	// vacia si no se ha podido deducir.
	ManagementPort string `json:"management_port"`
	// Config actual (mirror.conf): Monitor "" = espejo desactivado.
	Monitor string   `json:"monitor"`
	Sources []string `json:"sources"`
}

type MirrorSetRequest struct {
	// Monitor es la boca de monitorizacion; "" desactiva el espejo.
	Monitor string `json:"monitor"`
	// Sources son las bocas origen cuyo trafico se copia al monitor.
	Sources []string `json:"sources"`
}

// probeTcMirred es la costura de testeo del probe de capacidad.
var probeTcMirred = probeMirredSupport

// mirrorSupportCache memoiza el probe de capacidad por proceso (la API
// consulta el probe en cada GET; no hace falta re-probarlo, el resultado
// no cambia hasta que se cargue otro kernel).
var (
	mirrorSupportMu     sync.Mutex
	mirrorSupportDone   bool
	mirrorSupportMirror bool
	mirrorSupportEgress bool
)

func mirrorSupport() (mirror, egress bool) {
	mirrorSupportMu.Lock()
	defer mirrorSupportMu.Unlock()
	if !mirrorSupportDone {
		mirrorSupportMirror, mirrorSupportEgress = probeTcMirred()
		mirrorSupportDone = true
	}
	return mirrorSupportMirror, mirrorSupportEgress
}

// resetMirrorSupportForTest invalida la cache de capacidad (tests).
func resetMirrorSupportForTest() {
	mirrorSupportMu.Lock()
	mirrorSupportDone = false
	mirrorSupportMu.Unlock()
}

// probeMirredSupport comprueba de verdad si el kernel acepta mirred: crea
// un filtro temporal con la accion en la primera boca del puente y lo
// borra. Transitorio y sin efecto en el trafico (el filtro dura milisegundos
// y no llega a ver paquetes); si el qdisc clsact no estaba, lo quita al
// terminar. Necesita dos bocas (origen y monitor distintas).
func probeMirredSupport() (mirror, egress bool) {
	if !tcInstalled() {
		return false, false
	}
	ports := sortedBridgePorts()
	if len(ports) < 2 {
		return false, false
	}
	// Boca origen = la ULTIMA del orden alfabetico (lan9 en chasis lanN),
	// no la tipica de gestion (lan1); el add/del del probe dura milisegundos
	// y no llega a ver paquetes.
	src, dst := ports[len(ports)-1], ports[len(ports)-2]
	hadClsact := tcHasClsact(src)
	if !hadClsact {
		if out, err := addClsactRaw(src); err != nil {
			if !strings.Contains(out, "File exists") && !strings.Contains(out, "Exclusivity") {
				return false, false
			}
		}
	}
	defer func() {
		tcDelPrio(src, "ingress", 99)
		tcDelPrio(src, "egress", 99)
		if !hadClsact && !portTcInUse(src) {
			exec.Command("tc", "qdisc", "del", "dev", src, "clsact").Run()
		}
	}()

	if _, err := exec.Command("tc", mirrorFilterArgs(src, "ingress", dst, 99)...).CombinedOutput(); err != nil {
		return false, false
	}
	mirror = true
	// Si el ingress va, el egress casi seguro tambien; se prueba igual para
	// reportar con honestidad en la UI.
	_, egressErr := exec.Command("tc", mirrorFilterArgs(src, "egress", dst, 99)...).CombinedOutput()
	return mirror, egressErr == nil
}

func sortedBridgePorts() []string {
	portMap := bridgePorts()
	ports := make([]string, 0, len(portMap))
	for p := range portMap {
		ports = append(ports, p)
	}
	sort.Strings(ports)
	return ports
}

func ProbeMirror() MirrorProbe {
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return MirrorProbe{Applicable: false}
	}

	monitor, sources := loadMirrorConf()
	sourceSet := map[string]bool{}
	for _, s := range sources {
		sourceSet[s] = true
	}

	mirror, egress := mirrorSupport()

	var ports []MirrorPort
	for _, port := range sortedBridgePorts() {
		mp := MirrorPort{Port: port, Mirrored: sourceSet[port]}
		mp.LinkSpeedMbps = linkSpeedMbps(port)
		if mirror {
			_, active := parseMirrorShow(tcFilterShow(port, "ingress"))
			mp.Active = active
		}
		ports = append(ports, mp)
	}

	return MirrorProbe{
		Applicable:      true,
		Ports:           ports,
		TcInstalled:     tcInstalled(),
		MirrorSupported: mirror && egress, // la UI ofrece ambos sentidos juntos
		ManagementPort:  managementPort(),
		Monitor:         monitor,
		Sources:         sources,
	}
}

// validateMirrorReq comprueba la peticion contra las bocas reales del
// puente. Monitor vacio con sources vacios es la peticion de desactivar.
func validateMirrorReq(req MirrorSetRequest, portMap map[string]bool) error {
	if req.Monitor == "" {
		if len(req.Sources) > 0 {
			return fmt.Errorf("sources require a monitor port")
		}
		return nil
	}
	if !portMap[req.Monitor] {
		return fmt.Errorf("monitor port %s is not in the bridge", req.Monitor)
	}
	seen := map[string]bool{}
	for _, s := range req.Sources {
		if !portMap[s] {
			return fmt.Errorf("source port %s is not in the bridge", s)
		}
		if s == req.Monitor {
			return fmt.Errorf("source port %s is the monitor port", s)
		}
		if seen[s] {
			return fmt.Errorf("duplicate source port %s", s)
		}
		seen[s] = true
	}
	if len(req.Sources) == 0 {
		return fmt.Errorf("at least one source port is required")
	}
	return nil
}

func SetMirror(req MirrorSetRequest) error {
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return fmt.Errorf("no bridge ports found")
	}
	if err := validateMirrorReq(req, portMap); err != nil {
		return err
	}

	// Desactivar: quitar los filtros de todas las bocas que pudieran tenerlos.
	if req.Monitor == "" {
		writeMirrorConf("", nil)
		for _, port := range sortedBridgePorts() {
			removeMirrorFilters(port)
		}
		return nil
	}

	if err := ensureTc(); err != nil {
		return err
	}

	// Quitar el espejo de las bocas que ya no son origen. La config se
	// escribe ANTES de quitar filtros: maybeDropClsact la consulta para no
	// tirar el clsact de una boca que sigue configurada (y al escribir
	// primero, las bocas retiradas ya no figuran y su clsact cae).
	_, oldSources := loadMirrorConf()
	writeMirrorConf(req.Monitor, req.Sources)
	want := map[string]bool{}
	for _, s := range req.Sources {
		want[s] = true
	}
	for _, s := range oldSources {
		if !want[s] {
			removeMirrorFilters(s)
		}
	}

	for _, port := range req.Sources {
		if err := applyMirrorPort(port, req.Monitor); err != nil {
			return err
		}
	}

	writeMirrorConf(req.Monitor, req.Sources)
	return nil
}

// applyMirrorPort re-escribe el espejo de una boca origen hacia el
// monitor, en ingress y egress. Si el sentido egress falla se revierte el
// ingress para no dejar la boca a medias.
func applyMirrorPort(port, monitor string) error {
	if err := ensureClsact(port); err != nil {
		return err
	}
	tcDelPrio(port, "ingress", prioMirrorIngress)
	tcDelPrio(port, "egress", prioMirrorEgress)

	if out, err := exec.Command("tc", mirrorFilterArgs(port, "ingress", monitor, prioMirrorIngress)...).CombinedOutput(); err != nil {
		return fmt.Errorf("add ingress mirror on %s: %v (%s)", port, err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("tc", mirrorFilterArgs(port, "egress", monitor, prioMirrorEgress)...).CombinedOutput(); err != nil {
		tcDelPrio(port, "ingress", prioMirrorIngress)
		return fmt.Errorf("add egress mirror on %s: %v (%s)", port, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// removeMirrorFilters quita los filtros de espejo de una boca; si no
// queda nada que usarla, quita tambien el clsact.
func removeMirrorFilters(port string) {
	tcDelPrio(port, "ingress", prioMirrorIngress)
	tcDelPrio(port, "egress", prioMirrorEgress)
	maybeDropClsact(port)
}

// mirrorFilterArgs construye los argumentos del filtro de espejo (funcion
// pura, testeable).
func mirrorFilterArgs(port, direction, monitor string, prio int) []string {
	return []string{"filter", "add", "dev", port, direction,
		"protocol", "all", "prio", strconv.Itoa(prio), "matchall",
		"action", "mirred", "egress", "mirror", "dev", monitor}
}

// parseMirrorShow extrae la boca monitor de la salida de `tc filter show
// dev <port> ingress`: la accion se imprime como
// "mirred (Egress Mirror to device lan21) pipe".
func parseMirrorShow(show string) (string, bool) {
	i := strings.Index(show, "mirred (Egress Mirror to device ")
	if i < 0 {
		return "", false
	}
	rest := show[i+len("mirred (Egress Mirror to device "):]
	if j := strings.Index(rest, ")"); j > 0 {
		return strings.TrimSpace(rest[:j]), true
	}
	return "", false
}

// loadMirrorConf lee /etc/netgrip/mirror.conf:
//
//	monitor=lan21
//	sources=lan20 lan22
func loadMirrorConf() (string, []string) {
	data, err := os.ReadFile(mirrorConfPath)
	if err != nil {
		return "", nil
	}
	return parseMirrorConf(string(data))
}

// parseMirrorConf interpreta el contenido del fichero de configuracion
// (funcion pura, testeable).
func parseMirrorConf(text string) (string, []string) {
	var monitor string
	// Slice inicializado (no nil): serializado como [] y no como null,
	// que romperia la pestana Trafico al hacer join() en el cliente
	// (misma clase que el bug de loops, #175).
	sources := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "monitor":
			monitor = strings.TrimSpace(val)
		case "sources":
			for _, s := range strings.Fields(val) {
				s = strings.TrimSpace(s)
				if s != "" {
					sources = append(sources, s)
				}
			}
		}
	}
	return monitor, sources
}

// mirrorConfHasSource reporta si una boca figura como origen en la config
// persistida (lo usa maybeDropClsact para no tirar el qdisc de una boca
// configurada cuyos filtros aun no se han re-aplicado).
func mirrorConfHasSource(port string) bool {
	_, sources := loadMirrorConf()
	for _, s := range sources {
		if s == port {
			return true
		}
	}
	return false
}

func writeMirrorConf(monitor string, sources []string) {
	os.MkdirAll(filepath.Dir(mirrorConfPath), 0755)
	var b strings.Builder
	if monitor != "" {
		b.WriteString("monitor=" + monitor + "\n")
		if len(sources) > 0 {
			b.WriteString("sources=" + strings.Join(sources, " ") + "\n")
		}
	}
	os.WriteFile(mirrorConfPath, []byte(b.String()), 0600)
}

// managementPort deduce la boca por la que el equipo sale hacia su gateway
// por defecto: ruta por defecto -> MAC del gateway (ARP) -> puerto que la
// ha aprendido (FDB del puente). Heuristica para la UI: es la boca de
// gestion en un switch sin mas rutas. Devuelve "" si no se puede deducir.
func managementPort() string {
	route, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	gw := defaultGatewayIP(string(route))
	if gw == "" {
		return ""
	}
	arp, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return ""
	}
	mac := arpMACForIP(string(arp), gw)
	if mac == "" {
		return ""
	}
	for port, macs := range bridgeFdb() {
		for _, m := range macs {
			if strings.EqualFold(m, mac) {
				return port
			}
		}
	}
	return ""
}

// defaultGatewayIP extrae la IP del gateway por defecto de /proc/net/route
// (funcion pura, testeable). El kernel imprime las direcciones como el
// valor big-endian en hex (%08X de la __be32): C0A80101 -> 192.168.1.1
// (formato verificado en el LGS352C, kernel 6.12).
func defaultGatewayIP(routeText string) string {
	for _, line := range strings.Split(routeText, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		if flags, err := strconv.ParseUint(fields[3], 16, 32); err != nil || flags&0x2 == 0 {
			continue // sin flag UP/GATEWAY
		}
		hex := fields[2]
		if len(hex) != 8 {
			continue
		}
		b := make([]uint64, 4)
		ok := true
		for i := 0; i < 4; i++ {
			v, err := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
			if err != nil {
				ok = false
				break
			}
			b[i] = v
		}
		if !ok {
			continue
		}
		return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
	}
	return ""
}

// arpMACForIP busca la MAC de una IP en el contenido de /proc/net/arp
// (funcion pura, testeable).
func arpMACForIP(arpText, ip string) string {
	for _, line := range strings.Split(arpText, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == ip && fields[3] != "00:00:00:00:00:00" {
			return strings.ToLower(fields[3])
		}
	}
	return ""
}
