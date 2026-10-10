package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

const stormConfPath = "/etc/netgrip/storm.conf"

type StormPort struct {
	Port               string `json:"port"`
	LinkSpeedMbps      int    `json:"link_speed_mbps"`
	BroadcastKbps      int    `json:"broadcast_kbps"`
	MulticastKbps      int    `json:"multicast_kbps"`
	UnknownUnicastKbps int    `json:"unknown_unicast_kbps"`
	Active             bool   `json:"active"`
	// Percent es el limite configurado (storm.conf), 0 = sin control. El
	// kbps efectivo se lee de tc, pero no se puede traducir a porcentaje
	// sin velocidad de enlace (una boca caida reporta -1), asi que el
	// probe expone ambos.
	Percent int `json:"percent"`
}

type StormProbe struct {
	Applicable bool        `json:"applicable"`
	Ports      []StormPort `json:"ports"`
	// TcInstalled indica si el binario tc esta disponible. Los targets DSA
	// (p. ej. realtek rtl93xx, verificado en el LGS352C) no traen tc de
	// fabrica: SetStormControl lo instala al aplicar (#489), pero el probe
	// lo expone para que la UI pueda avisar antes del primer apply.
	TcInstalled bool `json:"tc_installed"`
}

type StormSetRequest struct {
	// Port aplica a una boca; Ports a una lista; All a todas las del
	// puente. En los tres casos con una sola llamada (#487).
	Port    string   `json:"port"`
	Ports   []string `json:"ports,omitempty"`
	All     bool     `json:"all,omitempty"`
	Percent int      `json:"percent"`
}

func ProbeStormControl() StormProbe {
	portMap := bridgePorts()
	if len(portMap) == 0 {
		return StormProbe{Applicable: false}
	}

	configs := loadStormConfigs()

	var ports []StormPort
	for port := range portMap {
		sp := readStormPort(port)
		sp.Percent = configs[port]
		ports = append(ports, sp)
	}

	return StormProbe{
		Applicable:  true,
		Ports:       ports,
		TcInstalled: tcInstalled(),
	}
}

// stormLookPath y stormInstallPkg son la costura de testeo de ensureTc: los
// tests inyectan falses y verifican la logica de decision (falta tc ->
// instalar paquete correcto -> reintentar) sin ejecutar nada real.
var (
	stormLookPath   = exec.LookPath
	stormInstallPkg = executor.Run
	// stormTcCheck verifica que tc no solo este en PATH sino que ejecute:
	// tras desinstalar tc-tiny puede quedar un stub de busybox (symlink
	// /sbin/tc -> /bin/busybox que imprime "applet not found" y sale 1).
	stormTcCheck = func() error { return exec.Command("tc", "-V").Run() }
)

// tcInstalled reporta si el binario tc esta en el PATH y realmente
// funciona (no basta LookPath: ver stormTcCheck).
func tcInstalled() bool {
	if _, err := stormLookPath("tc"); err != nil {
		return false
	}
	return stormTcCheck() == nil
}

// tcPackages son los paquetes que hacen falta para aplicar storm control:
// tc (binario), kmod-sched (em_cmp, el ematch de comparacion) y
// kmod-sched-act-police (la accion police). Verificado en el LGS352C
// (rtl931x, 25.12.1, #489). En releases con opkg (<= 24.10) act_police
// viaja dentro de kmod-sched y el paquete independiente no existe, de ahi
// el fallback.
var (
	tcPackages      = []string{"tc", "kmod-sched", "kmod-sched-act-police"}
	tcPackagesOpkg  = []string{"tc", "kmod-sched"}
	tcManualInstall = "tc, kmod-sched and kmod-sched-act-police (on opkg <= 24.10 the last one ships inside kmod-sched)"
)

// ensureTc garantiza que tc este disponible antes de aplicar filtros. Si
// falta, lo instala con pkg_add (deteccion apk/opkg ya centralizada en el
// executor, incluido el opkg update automatico con listas vacias) y
// re-verifica. Error accionable si la instalacion falla (offline, sin
// espacio, feed caido): el usuario sabe que paquetes instalar a mano.
func ensureTc() error {
	if tcInstalled() {
		return nil
	}
	if err := stormInstallPkg(executor.Op{Kind: "pkg_add", Args: tcPackages}); err != nil {
		// Fallback opkg antiguo: act_police no es paquete aparte.
		if err2 := stormInstallPkg(executor.Op{Kind: "pkg_add", Args: tcPackagesOpkg}); err2 != nil {
			return fmt.Errorf("tc is missing and installing %v failed (%v; fallback %v: %v): install %s manually", tcPackages, err, tcPackagesOpkg, err2, tcManualInstall)
		}
	}
	if !tcInstalled() {
		return fmt.Errorf("installed %v but tc is still not in PATH: install %s manually", tcPackages, tcManualInstall)
	}
	return nil
}

// readStormPort lee el estado de tc de una boca. El show solo funciona si
// tc esta instalado; si falta, la boca se reporta inactiva (SetStormControl
// lo instala al aplicar, #489).
func readStormPort(port string) StormPort {
	sp := StormPort{Port: port}

	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", port)
	if data, err := os.ReadFile(speedPath); err == nil {
		s := strings.TrimSpace(string(data))
		if v, err := strconv.Atoi(s); err == nil {
			sp.LinkSpeedMbps = v
		}
	}

	out, err := exec.Command("tc", "filter", "show", "dev", port, "ingress").CombinedOutput()
	if err != nil {
		return sp
	}
	output := string(out)

	// Las firmas corresponden a los filtros que crea setStormPort con
	// ematch cmp (asic se imprimen en `tc filter show`). No buscar
	// "police" a pelo: un police ajeno (manually added) daria falsos
	// positivos de "activo".
	if strings.Contains(output, stormSigBroadcast) {
		sp.Active = true
		sp.BroadcastKbps = parseTcRate(output, "broadcast")
	}
	if strings.Contains(output, stormSigMulticast) {
		sp.Active = true
		sp.MulticastKbps = parseTcRate(output, "multicast")
	}

	return sp
}

// Firmas de los filtros storm en la salida de `tc filter show` (asic las
// imprime el ematch cmp: "layer link" se muestra como "layer 0").
const (
	stormSigBroadcast = "cmp(u16 at 0 layer 0 mask 0xffff eq 65535)"
	stormSigMulticast = "mask 0x1000000 eq"
)

func parseTcRate(output string, kind string) int {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "rate") && i > 0 {
			parts := strings.Fields(line)
			for j, p := range parts {
				if p == "rate" && j+1 < len(parts) {
					val := parts[j+1]
					mult := 1
					// tc imprime "50Mbit" aunque se configure 50000kbit.
					if strings.HasSuffix(val, "Mbit") {
						mult = 1000
						val = strings.TrimSuffix(val, "Mbit")
					}
					val = strings.TrimSuffix(val, "Kbit")
					val = strings.TrimSuffix(val, "kbit")
					if v, err := strconv.Atoi(val); err == nil {
						return v * mult
					}
				}
			}
		}
	}
	return 0
}

func SetStormControl(req StormSetRequest) error {
	if req.Percent < 0 || req.Percent > 100 {
		return fmt.Errorf("invalid percent")
	}

	targets, err := stormTargets(req)
	if err != nil {
		return err
	}

	// Aplicar con un limite exige tc; los targets DSA no lo traen de
	// fabrica (#489). El 0 (revertir) solo borra del storm.conf y hace un
	// best-effort del qdisc, asi que no exige instalar nada.
	if req.Percent > 0 {
		if err := ensureTc(); err != nil {
			return err
		}
	}

	for _, port := range targets {
		if err := setStormPort(port, req.Percent); err != nil {
			return err
		}
	}
	return nil
}

// stormTargets resuelve la lista de bocas destino: todas las del puente
// (All), una lista explicita (Ports) o una sola (Port), validando que
// existan en el puente.
func stormTargets(req StormSetRequest) ([]string, error) {
	portMap := bridgePorts()
	if req.All {
		ports := make([]string, 0, len(portMap))
		for p := range portMap {
			ports = append(ports, p)
		}
		sort.Strings(ports)
		return ports, nil
	}
	if len(req.Ports) > 0 {
		for _, p := range req.Ports {
			if _, ok := portMap[p]; !ok {
				return nil, fmt.Errorf("port %s not in bridge", p)
			}
		}
		return req.Ports, nil
	}
	if req.Port == "" {
		return nil, fmt.Errorf("invalid port")
	}
	if _, ok := portMap[req.Port]; !ok {
		return nil, fmt.Errorf("port not in bridge")
	}
	return []string{req.Port}, nil
}

func setStormPort(port string, percent int) error {
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", port)
	speedMbps := 1000
	if data, err := os.ReadFile(speedPath); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v > 0 {
			speedMbps = v
		}
	}

	rateKbps := speedMbps * 1000 * percent / 100

	exec.Command("tc", "qdisc", "del", "dev", port, "ingress").Run()

	if percent == 0 {
		removeStormConfig(port)
		return nil
	}

	if err := exec.Command("tc", "qdisc", "add", "dev", port, "ingress").Run(); err != nil {
		return fmt.Errorf("add ingress qdisc on %s: %v", port, err)
	}

	bcRate := fmt.Sprintf("%dkbit", rateKbps)

	// Filtros de MAC destino con ematch cmp sobre "layer link": en ingress
	// skb->data apunta a la cabecera L3 y cls_u32 no admite offsets
	// negativos en este kernel (verificado en el LGS352C, rtl931x, #489).
	// Difusion: 6 primeros bytes del dst = ff. Multidifusion: bit de grupo
	// (LSB del primer byte) activado (cubre 01:00:5e IPv4 y 33:33 IPv6).
	if out, err := exec.Command("tc", "filter", "add", "dev", port, "parent", "ffff:",
		"protocol", "all", "prio", "1",
		"basic", "match", stormMatchBroadcast,
		"police", "rate", bcRate, "burst", "32k", "drop").CombinedOutput(); err != nil {
		return fmt.Errorf("add broadcast filter on %s: %v (%s)", port, err, strings.TrimSpace(string(out)))
	}

	if out, err := exec.Command("tc", "filter", "add", "dev", port, "parent", "ffff:",
		"protocol", "all", "prio", "2",
		"basic", "match", stormMatchMulticast,
		"police", "rate", bcRate, "burst", "32k", "drop").CombinedOutput(); err != nil {
		return fmt.Errorf("add multicast filter on %s: %v (%s)", port, err, strings.TrimSpace(string(out)))
	}

	saveStormConfig(port, percent)

	return nil
}

const (
	stormMatchBroadcast = "cmp(u16 at 0 layer link mask 0xffff eq 0xffff) and cmp(u16 at 2 layer link mask 0xffff eq 0xffff) and cmp(u16 at 4 layer link mask 0xffff eq 0xffff)"
	stormMatchMulticast = "cmp(u32 at 0 layer link mask 0x01000000 eq 0x01000000)"
)

func loadStormConfigs() map[string]int {
	configs := make(map[string]int)
	f, err := os.Open(stormConfPath)
	if err != nil {
		return configs
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			port := strings.TrimSpace(parts[0])
			pct, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err == nil && pct > 0 {
				configs[port] = pct
			}
		}
	}
	return configs
}

func saveStormConfig(port string, percent int) {
	configs := loadStormConfigs()
	configs[port] = percent
	writeStormConfigs(configs)
}

func removeStormConfig(port string) {
	configs := loadStormConfigs()
	delete(configs, port)
	writeStormConfigs(configs)
}

func writeStormConfigs(configs map[string]int) {
	os.MkdirAll(filepath.Dir(stormConfPath), 0755)
	var lines []string
	for port, pct := range configs {
		lines = append(lines, fmt.Sprintf("%s=%d", port, pct))
	}
	os.WriteFile(stormConfPath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}
