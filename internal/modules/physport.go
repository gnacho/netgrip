package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Configuracion fisica por puerto (#485): negociacion, velocidad/duplex,
// EEE, MTU y diagnostico de modulos SFP. Todo via ethtool (ethtool-full en
// OpenWrt) y sysfs; los errores del driver se devuelven limpios, sin stack.

type LinkMode struct {
	SpeedMbps int    `json:"speed_mbps"`
	Duplex    string `json:"duplex"`
}

// SFPInfo es el resultado de `ethtool -m`: datos del EEPROM del modulo y
// lecturas DOM. State distingue: module (modulo presente), empty (jaula sin
// modulo), unsupported (puerto sin EEPROM, ej. cobre) y error.
type SFPInfo struct {
	State      string  `json:"state"`
	Identifier string  `json:"identifier,omitempty"`
	Vendor     string  `json:"vendor,omitempty"`
	PN         string  `json:"pn,omitempty"`
	SN         string  `json:"sn,omitempty"`
	Date       string  `json:"date,omitempty"`
	TempC      float64 `json:"temp_c,omitempty"`
	Voltage    float64 `json:"voltage,omitempty"`
	BiasMA     float64 `json:"bias_ma,omitempty"`
	TxPowerMW  float64 `json:"tx_power_mw,omitempty"`
	TxPowerDBM float64 `json:"tx_power_dbm,omitempty"`
	RxPowerMW  float64 `json:"rx_power_mw,omitempty"`
	RxPowerDBM float64 `json:"rx_power_dbm,omitempty"`
}

type PhysPort struct {
	Name         string     `json:"name"`
	Autoneg      bool       `json:"autoneg"`
	SpeedMbps    int        `json:"speed_mbps"`
	Duplex       string     `json:"duplex"`
	Supported    []LinkMode `json:"supported"`
	EeeSupported bool       `json:"eee_supported"`
	EeeEnabled   bool       `json:"eee_enabled"`
	MTU          int        `json:"mtu"`
	MTUMax       int        `json:"mtu_max"`
	MTUSupported bool       `json:"mtu_supported"`
	SFP          *SFPInfo   `json:"sfp,omitempty"`
}

type PhysPortsProbe struct {
	Applicable bool       `json:"applicable"`
	Ports      []PhysPort `json:"ports"`
}

// ProbePhysPorts sondea cada puerto del bridge. Sin bridge no aplica. El
// techo de MTU se descubre una sola vez por probe (el primer puerto): se
// intenta 1522 y se revierte a 1500; si el driver lo rechaza, ningun puerto
// del chip lo soporta.
func ProbePhysPorts() *PhysPortsProbe {
	ports := bridgePortList()
	probe := &PhysPortsProbe{Applicable: false, Ports: []PhysPort{}}
	if len(ports) == 0 {
		return probe
	}
	probe.Applicable = true
	mtuSupported := probeMTUCeiling(ports[0])
	for _, name := range ports {
		p := PhysPort{Name: name, Supported: []LinkMode{}, MTUSupported: mtuSupported, MTUMax: 1500}
		if mtuSupported {
			p.MTUMax = 1522
		}
		if data, err := os.ReadFile("/sys/class/net/" + name + "/mtu"); err == nil {
			p.MTU, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		p.Autoneg, p.SpeedMbps, p.Duplex, p.Supported = ethtoolPort(name)
		p.EeeSupported, p.EeeEnabled = ethtoolEEE(name)
		if sfp := ethtoolModule(name); sfp != nil {
			p.SFP = sfp
		}
		probe.Ports = append(probe.Ports, p)
	}
	return probe
}

var linkModeRe = regexp.MustCompile(`^(\d+)base[TFX]/([A-Za-z]+)`)

// ethtoolPort parsea la salida de `ethtool <dev>`: negociacion, velocidad y
// duplex actuales y modos soportados. Un puerto caido contesta "Speed:
// Unknown!": se devuelve 0 y la UI lo muestra como sin enlace.
func ethtoolPort(dev string) (autoneg bool, speedMbps int, duplex string, supported []LinkMode) {
	out, err := exec.Command("ethtool", dev).Output()
	if err != nil {
		return false, 0, "", nil
	}
	return parseEtoolOutput(string(out))
}

func parseEtoolOutput(output string) (autoneg bool, speedMbps int, duplex string, supported []LinkMode) {
	inSupported := false
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "Supported link modes:"):
			inSupported = true
			supported = append(supported, parseLinkModes(strings.TrimPrefix(line, "Supported link modes:"))...)
		case inSupported:
			// Las lineas de modos van indentadas; cualquier otra cosa cierra el bloque.
			if m := linkModeRe.FindStringSubmatch(line); m != nil {
				supported = append(supported, parseLinkModes(line)...)
			} else {
				inSupported = false
			}
		}
		if strings.HasPrefix(line, "Speed:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && strings.HasSuffix(fields[1], "Mb/s") {
				speedMbps, _ = strconv.Atoi(strings.TrimSuffix(fields[1], "Mb/s"))
			}
		}
		if strings.HasPrefix(line, "Duplex:") {
			fields := strings.Fields(line)
			// Puerto caido: "Duplex: Unknown! (255)". Se normaliza a "".
			if len(fields) >= 2 && !strings.HasPrefix(strings.ToLower(fields[1]), "unknown") {
				duplex = strings.ToLower(fields[1])
			}
		}
		if strings.HasPrefix(line, "Auto-negotiation:") {
			// Ojo: "Auto-negotiation" contiene "on"; se compara el valor.
			fields := strings.Fields(line)
			autoneg = len(fields) >= 2 && fields[1] == "on"
		}
	}
	return autoneg, speedMbps, duplex, dedupeLinkModes(supported)
}

func parseLinkModes(s string) []LinkMode {
	var modes []LinkMode
	for _, tok := range strings.Fields(s) {
		if m := linkModeRe.FindStringSubmatch(tok); m != nil {
			modes = append(modes, LinkMode{SpeedMbps: mustAtoi(m[1]), Duplex: strings.ToLower(m[2])})
		}
	}
	return modes
}

func dedupeLinkModes(modes []LinkMode) []LinkMode {
	seen := map[string]bool{}
	out := make([]LinkMode, 0, len(modes))
	for _, m := range modes {
		key := fmt.Sprintf("%d/%s", m.SpeedMbps, m.Duplex)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, m)
	}
	return out
}

func mustAtoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// ethtoolEEE devuelve (soportado, activado). Sin modos EEE soportados el
// hardware no tiene EEE aunque el comando exista; es el criterio que usa la
// UI para mostrar u ocultar el toggle.
func ethtoolEEE(dev string) (supported bool, enabled bool) {
	out, err := exec.Command("ethtool", "--show-eee", dev).Output()
	if err != nil {
		return false, false
	}
	return parseEeeOutput(string(out))
}

func parseEeeOutput(output string) (supported bool, enabled bool) {
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "EEE status:") {
			// "disabled" contiene "enabled" como subcadena: se compara la
			// palabra completa, no el substring.
			fields := strings.Fields(line)
			enabled = len(fields) >= 3 && fields[2] == "enabled"
		}
		if strings.HasPrefix(line, "Supported EEE link modes:") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "Supported EEE link modes:"))
			if rest != "" && rest != "Not reported" {
				supported = true
			}
		}
	}
	return supported, enabled
}

// ethtoolModule detecta la capacidad SFP del puerto con `ethtool -m`: si
// devuelve datos o el error de jaula vacia, hay jaula; "not supported" es
// un puerto de cobre corriente.
func ethtoolModule(dev string) *SFPInfo {
	out, err := exec.Command("ethtool", "-m", dev).CombinedOutput()
	if err != nil {
		msg := string(out)
		switch {
		case strings.Contains(msg, "No such device"):
			return &SFPInfo{State: "empty"}
		case strings.Contains(strings.ToLower(msg), "not supported"):
			return nil
		default:
			return &SFPInfo{State: "error"}
		}
	}
	info := parseSFPEeprom(string(out))
	info.State = "module"
	return info
}

var eepromKVRe = regexp.MustCompile(`^\s*([^:]+?)\s*:\s*(.*)$`)

// parseSFPEeprom parsea las lineas "Clave : valor" del EEPROM. Los valores
// de potencia vienen como "0.50 mW / -3.01 dBm".
func parseSFPEeprom(output string) *SFPInfo {
	info := &SFPInfo{}
	for _, raw := range strings.Split(output, "\n") {
		m := eepromKVRe.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		key, val := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
		switch {
		case key == "Identifier":
			info.Identifier = val
		case key == "Vendor name":
			info.Vendor = val
		case key == "Vendor PN":
			info.PN = val
		case key == "Vendor SN":
			info.SN = val
		case strings.HasPrefix(key, "Date code"):
			info.Date = val
		case key == "Temperature":
			info.TempC = parseLeadingFloat(val)
		case key == "Supply voltage":
			info.Voltage = parseLeadingFloat(val)
		case key == "Laser bias current":
			info.BiasMA = parseLeadingFloat(val)
		case key == "TX power":
			info.TxPowerMW, info.TxPowerDBM = parsePower(val)
		case key == "RX power":
			info.RxPowerMW, info.RxPowerDBM = parsePower(val)
		}
	}
	return info
}

func parseLeadingFloat(s string) float64 {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func parsePower(s string) (mw float64, dbm float64) {
	for _, part := range strings.Split(s, "/") {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		switch fields[1] {
		case "mW":
			mw = v
		case "dBm":
			dbm = v
		}
	}
	return mw, dbm
}

// probeMTUCeiling intenta subir el MTU del puerto a 1522 y lo revierte.
// Si el driver rechaza el cambio (EINVAL en este hardware), el chip no
// soporta jumbo frames y la UI muestra "no soportado" en vez de un error.
func probeMTUCeiling(dev string) bool {
	mtuPath := "/sys/class/net/" + dev + "/mtu"
	data, err := os.ReadFile(mtuPath)
	if err != nil {
		return false
	}
	orig := strings.TrimSpace(string(data))
	if err := os.WriteFile(mtuPath, []byte("1522"), 0644); err != nil {
		return false
	}
	if err := os.WriteFile(mtuPath, []byte(orig), 0644); err != nil {
		// No deberia pasar (ya escribimos antes); si pasa, al menos no mentimos.
		return true
	}
	return true
}

type PhysPortEdit struct {
	Name      string `json:"name"`
	Autoneg   *bool  `json:"autoneg,omitempty"`
	SpeedMbps *int   `json:"speed_mbps,omitempty"`
	Duplex    string `json:"duplex,omitempty"`
	MTU       *int   `json:"mtu,omitempty"`
	EEE       *bool  `json:"eee,omitempty"`
}

// SetPhysPort aplica la configuracion fisica pedida. Las escrituras van en
// el orden MTU (la mas probable de fallar) -> EEE -> negociacion/velocidad,
// asi el error que llega al usuario es del driver que realmente lo rechazo.
func SetPhysPort(e PhysPortEdit) error {
	if e.Name == "" {
		return fmt.Errorf("port name is required")
	}
	if _, err := os.Stat("/sys/class/net/" + e.Name); err != nil {
		return fmt.Errorf("port %s not found", e.Name)
	}
	if e.MTU != nil {
		if err := setPhysPortMTU(e.Name, *e.MTU); err != nil {
			return err
		}
	}
	if e.EEE != nil {
		if err := setPhysPortEEE(e.Name, *e.EEE); err != nil {
			return err
		}
	}
	if e.Autoneg != nil {
		if err := setPhysPortAutoneg(e.Name, *e.Autoneg, e.SpeedMbps, e.Duplex); err != nil {
			return err
		}
	}
	return nil
}

func setPhysPortMTU(dev string, mtu int) error {
	if mtu < 576 || mtu > 9700 {
		return fmt.Errorf("MTU must be 576-9700")
	}
	if mtu > 1500 {
		if !probeMTUCeiling(dev) {
			return fmt.Errorf("MTU above 1500 is not supported by this device")
		}
	}
	out, err := exec.Command("ip", "link", "set", "dev", dev, "mtu", strconv.Itoa(mtu)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("MTU %d rejected by the driver on %s: %s", mtu, dev, strings.TrimSpace(string(out)))
	}
	return nil
}

func setPhysPortEEE(dev string, enabled bool) error {
	// ethtool 6.15 (OpenWrt 25.12) rechaza "enabled"/"disabled" para eee y
	// acepta on/off; verificado en hardware real (#485).
	state := "off"
	if enabled {
		state = "on"
	}
	out, err := exec.Command("ethtool", "--set-eee", dev, "eee", state).CombinedOutput()
	if err != nil {
		return fmt.Errorf("EEE %s rejected on %s: %s", state, dev, strings.TrimSpace(string(out)))
	}
	return nil
}

func setPhysPortAutoneg(dev string, autoneg bool, speedMbps *int, duplex string) error {
	var args []string
	if autoneg {
		args = []string{"-s", dev, "autoneg", "on"}
	} else {
		speed := 0
		if speedMbps != nil {
			speed = *speedMbps
		}
		d := strings.ToLower(duplex)
		if speed <= 0 || (d != "full" && d != "half") {
			return fmt.Errorf("speed and duplex are required when auto-negotiation is off")
		}
		if speed != 10 && speed != 100 && speed != 1000 && speed != 2500 && speed != 10000 {
			return fmt.Errorf("speed must be 10, 100, 1000, 2500 or 10000 Mbps")
		}
		args = []string{"-s", dev, "autoneg", "off", "speed", strconv.Itoa(speed), "duplex", d}
	}
	out, err := exec.Command("ethtool", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("link settings rejected on %s: %s", dev, strings.TrimSpace(string(out)))
	}
	return nil
}
