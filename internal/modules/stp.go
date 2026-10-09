package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// STP (802.1d) del kernel, leido y escrito por sysfs (#485). Los timers del
// puente se exponen en jiffies con USER_HZ=100, asi que segundos = raw/100.
// "Borde" (portfast) y "Punto a punto" no existen en el STP del kernel;
// RSTP/portfast requeriria mstpd (#449).

const userHZ = 100

// Estados STP del kernel (include/uapi/linux/if_bridge.h).
var stpStateNames = map[int]string{
	0: "disabled",
	1: "blocking",
	2: "listening",
	3: "learning",
	4: "forwarding",
}

func stpStateName(state int) string {
	if n, ok := stpStateNames[state]; ok {
		return n
	}
	return "unknown"
}

type STPBridge struct {
	Enabled        bool   `json:"enabled"`
	Priority       int    `json:"priority"`
	HelloTime      int    `json:"hello_time"`
	MaxAge         int    `json:"max_age"`
	ForwardDelay   int    `json:"forward_delay"`
	BridgeID       string `json:"bridge_id"`
	DesignatedRoot string `json:"designated_root"`
	RootPort       string `json:"root_port"`
	RootPathCost   int64  `json:"root_path_cost"`
	TopologyChange bool   `json:"topology_change"`
}

type STPPort struct {
	Port           string `json:"port"`
	State          int    `json:"state"`
	StateName      string `json:"state_name"`
	PathCost       int64  `json:"path_cost"`
	Priority       int    `json:"priority"`
	BpduGuard      bool   `json:"bpdu_guard"`
	BpduFilter     bool   `json:"bpdu_filter"`
	RootBlock      bool   `json:"root_block"`
	PortID         string `json:"port_id"`
	DesignatedRoot string `json:"designated_root"`
	DesignatedCost int64  `json:"designated_cost"`
	DesignatedPort string `json:"designated_port"`
}

type STPProbe struct {
	Applicable bool      `json:"applicable"`
	Bridge     string    `json:"bridge"`
	BridgeInfo STPBridge `json:"bridge_info"`
	Ports      []STPPort `json:"ports"`
}

func bridgeSysPath(bridge string) string {
	return "/sys/class/net/" + bridge + "/bridge"
}

// ProbeSTP lee el estado STP del puente del bridge LAN y de cada uno de sus
// puertos. Sin bridge en sysfs no hay nada que gestionar: probe limpio con
// applicable=false, como los demas modulos de switch.
func ProbeSTP() *STPProbe {
	bridge := LANBridge()
	probe := &STPProbe{Applicable: false, Bridge: bridge, Ports: []STPPort{}}
	if _, err := os.Stat(bridgeSysPath(bridge)); err != nil {
		return probe
	}
	probe.Applicable = true
	probe.BridgeInfo = readSTPBridge(bridge)
	for _, port := range bridgePortList() {
		if sp, ok := readSTPPort(bridge, port); ok {
			probe.Ports = append(probe.Ports, sp)
		}
	}
	return probe
}

func readSTPBridge(bridge string) STPBridge {
	base := bridgeSysPath(bridge)
	b := STPBridge{
		Enabled:        readSysIntDefault(filepath.Join(base, "stp_state")) == 1,
		Priority:       int(readSysIntDefault(filepath.Join(base, "priority"))),
		HelloTime:      int(readSysIntDefault(filepath.Join(base, "hello_time")) / userHZ),
		MaxAge:         int(readSysIntDefault(filepath.Join(base, "max_age")) / userHZ),
		ForwardDelay:   int(readSysIntDefault(filepath.Join(base, "forward_delay")) / userHZ),
		BridgeID:       readSysString(filepath.Join(base, "bridge_id")),
		DesignatedRoot: readSysString(filepath.Join(base, "designated_root")),
		RootPort:       resolveIfIndex(int(readSysIntDefault(filepath.Join(base, "root_port")))),
		RootPathCost:   readSysIntDefault(filepath.Join(base, "root_path_cost")),
		TopologyChange: readSysIntDefault(filepath.Join(base, "topology_change")) == 1,
	}
	return b
}

// readSTPPort devuelve false cuando el puerto no tiene brport (salio del
// bridge entre el listado y la lectura).
func readSTPPort(bridge, port string) (STPPort, bool) {
	base := "/sys/class/net/" + port + "/brport"
	if _, err := os.Stat(base); err != nil {
		return STPPort{}, false
	}
	state := int(readSysIntDefault(filepath.Join(base, "state")))
	return STPPort{
		Port:           port,
		State:          state,
		StateName:      stpStateName(state),
		PathCost:       readSysIntDefault(filepath.Join(base, "path_cost")),
		Priority:       int(readSysIntDefault(filepath.Join(base, "priority"))),
		BpduGuard:      readSysIntDefault(filepath.Join(base, "bpdu_guard")) == 1,
		BpduFilter:     readSysIntDefault(filepath.Join(base, "bpdu_filter")) == 1,
		RootBlock:      readSysIntDefault(filepath.Join(base, "root_block")) == 1,
		PortID:         readSysString(filepath.Join(base, "port_id")),
		DesignatedRoot: readSysString(filepath.Join(base, "designated_root")),
		DesignatedCost: readSysIntDefault(filepath.Join(base, "designated_cost")),
		DesignatedPort: readSysString(filepath.Join(base, "designated_port")),
	}, true
}

type STPBridgeEdit struct {
	Enabled      bool `json:"enabled"`
	Priority     int  `json:"priority"`
	HelloTime    int  `json:"hello_time"`
	MaxAge       int  `json:"max_age"`
	ForwardDelay int  `json:"forward_delay"`
}

// Validaciones del kernel, comprobadas antes de escribir para que el error
// sea claro en lugar de un EINVAL crudo de sysfs:
//   - priority: multiplo de 4096, 0-61440
//   - hello: 1-10 s, max_age: 6-40 s, forward_delay: 4-30 s
func validateSTPBridge(e STPBridgeEdit) error {
	if e.Priority < 0 || e.Priority > 61440 || e.Priority%4096 != 0 {
		return fmt.Errorf("priority must be a multiple of 4096 between 0 and 61440")
	}
	if e.HelloTime < 1 || e.HelloTime > 10 {
		return fmt.Errorf("hello time must be 1-10 seconds")
	}
	if e.MaxAge < 6 || e.MaxAge > 40 {
		return fmt.Errorf("max age must be 6-40 seconds")
	}
	if e.ForwardDelay < 4 || e.ForwardDelay > 30 {
		return fmt.Errorf("forward delay must be 4-30 seconds")
	}
	return nil
}

// SetSTPBridge escribe los parametros del puente. Los timers van en jiffies
// (segundos * USER_HZ) y stp_state se escribe el ultimo: activar STP es lo
// que dispara la convergencia, y sus parametros deben estar ya fijados.
func SetSTPBridge(e STPBridgeEdit) error {
	if err := validateSTPBridge(e); err != nil {
		return err
	}
	base := bridgeSysPath(LANBridge())
	if _, err := os.Stat(base); err != nil {
		return fmt.Errorf("no bridge found")
	}
	writes := []struct {
		file string
		val  string
	}{
		{"priority", strconv.Itoa(e.Priority)},
		{"hello_time", strconv.Itoa(e.HelloTime * userHZ)},
		{"max_age", strconv.Itoa(e.MaxAge * userHZ)},
		{"forward_delay", strconv.Itoa(e.ForwardDelay * userHZ)},
	}
	for _, wr := range writes {
		if err := os.WriteFile(filepath.Join(base, wr.file), []byte(wr.val), 0644); err != nil {
			return fmt.Errorf("write %s: %w", wr.file, err)
		}
	}
	stp := "0"
	if e.Enabled {
		stp = "1"
	}
	if err := os.WriteFile(filepath.Join(base, "stp_state"), []byte(stp), 0644); err != nil {
		return fmt.Errorf("write stp_state: %w", err)
	}
	return nil
}

type STPPortEdit struct {
	Name       string `json:"name"`
	PathCost   *int64 `json:"path_cost,omitempty"`
	Priority   *int   `json:"priority,omitempty"`
	BpduGuard  *bool  `json:"bpdu_guard,omitempty"`
	BpduFilter *bool  `json:"bpdu_filter,omitempty"`
	RootBlock  *bool  `json:"root_block,omitempty"`
}

// Validaciones del kernel por puerto: path_cost 1-200000000 y priority
// multiplo de 16 entre 0 y 240.
func validateSTPPort(e STPPortEdit) error {
	if e.PathCost != nil && (*e.PathCost < 1 || *e.PathCost > 200000000) {
		return fmt.Errorf("path cost must be 1-200000000")
	}
	if e.Priority != nil && (*e.Priority < 0 || *e.Priority > 240 || *e.Priority%16 != 0) {
		return fmt.Errorf("port priority must be a multiple of 16 between 0 and 240")
	}
	return nil
}

func SetSTPPort(e STPPortEdit) error {
	if e.Name == "" {
		return fmt.Errorf("port name is required")
	}
	if err := validateSTPPort(e); err != nil {
		return err
	}
	base := "/sys/class/net/" + e.Name + "/brport"
	if _, err := os.Stat(base); err != nil {
		return fmt.Errorf("port %s is not a bridge member", e.Name)
	}
	writes := []struct {
		file string
		val  string
	}{
		{"path_cost", int64PtrString(e.PathCost)},
		{"priority", intPtrString(e.Priority)},
		{"bpdu_guard", boolPtrString(e.BpduGuard)},
		{"bpdu_filter", boolPtrString(e.BpduFilter)},
		{"root_block", boolPtrString(e.RootBlock)},
	}
	for _, wr := range writes {
		if wr.val == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(base, wr.file), []byte(wr.val), 0644); err != nil {
			return fmt.Errorf("write %s on %s: %w", wr.file, e.Name, err)
		}
	}
	return nil
}

func int64PtrString(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func intPtrString(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func boolPtrString(v *bool) string {
	if v == nil {
		return ""
	}
	if *v {
		return "1"
	}
	return "0"
}

// resolveIfIndex traduce el ifindex numerico que publica root_port al nombre
// de interfaz; 0 (sin puerto raiz) devuelve "".
func resolveIfIndex(idx int) string {
	if idx <= 0 {
		return ""
	}
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if data, err := os.ReadFile("/sys/class/net/" + e.Name() + "/ifindex"); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && v == idx {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func readSysIntDefault(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return v
}

func readSysString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
