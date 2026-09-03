package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// LAGEntry describes a configured link aggregation (bond).
type LAGEntry struct {
	Name   string   `json:"name"`   // UCI interface section name (e.g. lag0)
	Device string   `json:"device"` // kernel device (bond-<name>)
	Mode   string   `json:"mode"`   // 802.3ad | active-backup | balance-rr
	Slaves []string `json:"slaves"`
	Up     bool     `json:"up"` // device exists and has at least one enslaved port
}

// LAGProbe is the read-only link aggregation state.
type LAGProbe struct {
	Applicable bool       `json:"applicable"` // bridge with 2+ physical ports
	Installed  bool       `json:"installed"`  // kmod-bonding loaded + proto handler present
	LAGs       []LAGEntry `json:"lags"`
	FreePorts  []string   `json:"free_ports"` // physical ports available to join a LAG
}

// LAGConfig is the user-provided LAG definition.
type LAGConfig struct {
	Name   string   `json:"name"`
	Mode   string   `json:"mode"`
	Slaves []string `json:"slaves"`
}

var lagModes = map[string]bool{"802.3ad": true, "active-backup": true, "balance-rr": true}
var lagNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,12}$`)
var uciQuoted = regexp.MustCompile(`'([^']*)'`)

// uciListValues extracts every single-quoted value from the RHS of a
// `uci show` line. List options print all items on one line
// (key='a' 'b'); scalar options print one.
func uciListValues(line string) []string {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return nil
	}
	var out []string
	for _, m := range uciQuoted.FindAllStringSubmatch(line[idx:], -1) {
		out = append(out, m[1])
	}
	return out
}

func lagKmodLoaded() bool {
	_, err := os.Stat("/sys/class/net/bonding_masters")
	return err == nil
}

func lagProtoPresent() bool {
	_, err := os.Stat("/lib/netifd/proto/bonding.sh")
	return err == nil
}

func lagDeviceName(name string) string { return "bond-" + name }

// lagBridgeSection returns the UCI section id of the br-lan bridge device
// (e.g. "@device[0]" or an anonymous cfg id).
func lagBridgeSection() string {
	out, err := exec.Command("uci", "show", "network").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// network.<sec>.name='br-lan'
		if strings.HasSuffix(line, ".name='br-lan'") {
			sec := strings.TrimSuffix(strings.TrimPrefix(line, "network."), ".name='br-lan'")
			return sec
		}
	}
	return ""
}

func lagBridgePorts(sec string) []string {
	var ports []string
	if sec == "" {
		return ports
	}
	out, err := exec.Command("uci", "show", "network."+sec+".ports").Output()
	if err != nil {
		return ports
	}
	for _, v := range uciListValues(string(out)) {
		ports = append(ports, v)
	}
	return ports
}

func lagPhysicalPorts() []string {
	var out []string
	for p := range bridgePorts() {
		if strings.HasPrefix(p, "phy") || strings.HasPrefix(p, "wlan") || strings.HasPrefix(p, "bond-") {
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func lagSysfsSlaves(device string) []string {
	data, err := os.ReadFile("/sys/class/net/" + device + "/bonding/slaves")
	if err != nil {
		return nil
	}
	return strings.Fields(string(data))
}

// ProbeLAGs reads the link aggregation state.
func ProbeLAGs() *LAGProbe {
	p := &LAGProbe{LAGs: []LAGEntry{}, FreePorts: []string{}}
	phys := lagPhysicalPorts()
	p.Applicable = len(phys) >= 2 && lagBridgeSection() != ""
	p.Installed = lagKmodLoaded() && lagProtoPresent()

	usedByLAG := map[string]bool{}
	out, err := exec.Command("uci", "show", "network").Output()
	if err == nil {
		current := ""
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "network.") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := parts[0]
			val := strings.Trim(parts[1], "'")
			switch {
			case strings.HasSuffix(key, ".proto") && val == "bonding":
				current = strings.TrimPrefix(key, "network.")
				current = strings.TrimSuffix(current, ".proto")
				p.LAGs = append(p.LAGs, LAGEntry{Name: current, Device: lagDeviceName(current)})
			case current != "" && strings.HasSuffix(key, ".bonding_policy"):
				p.LAGs[len(p.LAGs)-1].Mode = val
			case current != "" && strings.HasSuffix(key, ".slaves"):
				for _, s := range uciListValues(line) {
					p.LAGs[len(p.LAGs)-1].Slaves = append(p.LAGs[len(p.LAGs)-1].Slaves, s)
					usedByLAG[s] = true
				}
			case current != "" && !strings.Contains(key, "network."+current+"."):
				current = ""
			}
		}
	}
	for i := range p.LAGs {
		e := &p.LAGs[i]
		if e.Mode == "" {
			e.Mode = "802.3ad"
		}
		if sysSlaves := lagSysfsSlaves(e.Device); len(sysSlaves) > 0 {
			e.Up = true
		}
	}
	sort.Slice(p.LAGs, func(i, j int) bool { return p.LAGs[i].Name < p.LAGs[j].Name })

	for _, port := range phys {
		if !usedByLAG[port] {
			p.FreePorts = append(p.FreePorts, port)
		}
	}
	return p
}

func lagFindEntry(name string) *LAGEntry {
	probe := ProbeLAGs()
	for i := range probe.LAGs {
		if probe.LAGs[i].Name == name {
			return &probe.LAGs[i]
		}
	}
	return nil
}

// SetLAG creates or updates a LAG. Ports move from the bridge into the bond;
// former slaves of an edited LAG that are no longer members go back to the
// bridge. Snapshot + healthcheck + rollback included.
func SetLAG(cfg LAGConfig) (*LAGProbe, bool, error) {
	if !lagNameRe.MatchString(cfg.Name) {
		return ProbeLAGs(), false, fmt.Errorf("invalid LAG name")
	}
	if !lagModes[cfg.Mode] {
		return ProbeLAGs(), false, fmt.Errorf("unsupported bond mode %q", cfg.Mode)
	}
	if len(cfg.Slaves) < 2 {
		return ProbeLAGs(), false, fmt.Errorf("a LAG needs at least two ports")
	}

	phys := map[string]bool{}
	for _, p := range lagPhysicalPorts() {
		phys[p] = true
	}
	for _, s := range cfg.Slaves {
		if !phys[s] {
			return ProbeLAGs(), false, fmt.Errorf("port %q is not a physical bridge port", s)
		}
	}

	// Ports already used by another LAG are rejected.
	probe := ProbeLAGs()
	oldSlaves := map[string]bool{}
	for _, lag := range probe.LAGs {
		if lag.Name == cfg.Name {
			for _, s := range lag.Slaves {
				oldSlaves[s] = true
			}
			continue
		}
		for _, s := range lag.Slaves {
			for _, want := range cfg.Slaves {
				if s == want {
					return ProbeLAGs(), false, fmt.Errorf("port %q already belongs to LAG %s", s, lag.Name)
				}
			}
		}
	}

	bridgeSec := lagBridgeSection()
	if bridgeSec == "" {
		return ProbeLAGs(), false, fmt.Errorf("br-lan bridge not found")
	}

	snap, err := executor.Snapshot("network")
	if err != nil {
		return ProbeLAGs(), false, fmt.Errorf("snapshot network: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("network", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "restart"}})
	}

	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}

	// Install kernel module and proto handler when missing. A freshly
	// installed proto handler is only registered after a network restart
	// (verified on 25.12.5: reload/ifup keep proto "none").
	needRestart := false
	if !lagKmodLoaded() || !lagProtoPresent() {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"kmod-bonding", "proto-bonding"}})
		needRestart = true
	}

	// (Re)create the bonding interface.
	if existing := lagFindEntry(cfg.Name); existing != nil {
		ops = append(ops, executor.Op{Kind: "ifdown", Args: []string{cfg.Name}})
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + cfg.Name + ".slaves"}})
	}
	set("network."+cfg.Name, "interface")
	set("network."+cfg.Name+".proto", "bonding")
	set("network."+cfg.Name+".bonding_policy", cfg.Mode)
	set("network."+cfg.Name+".link_monitoring", "mii")
	set("network."+cfg.Name+".miimon", "100")
	for _, s := range cfg.Slaves {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + cfg.Name + ".slaves", s}})
	}

	// Move ports between bridge and bond.
	currentBridgePorts := map[string]bool{}
	for _, p := range lagBridgePorts(bridgeSec) {
		currentBridgePorts[p] = true
	}
	for _, s := range cfg.Slaves {
		if currentBridgePorts[s] {
			ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{"network." + bridgeSec + ".ports", s}})
		}
	}
	if !currentBridgePorts[lagDeviceName(cfg.Name)] {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + bridgeSec + ".ports", lagDeviceName(cfg.Name)}})
	}
	for s := range oldSlaves {
		if !containsStr(cfg.Slaves, s) && !currentBridgePorts[s] {
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + bridgeSec + ".ports", s}})
		}
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeLAGs(), true, err
	}

	reload := "reload"
	if needRestart {
		reload = "restart"
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", reload}})
	// Re-assert the bond after the bridge is updated: enslaving a port that
	// is still a bridge port fails silently (verified on 25.12.5).
	_ = executor.Run(executor.Op{Kind: "ifup", Args: []string{cfg.Name}})

	if !lagHealth(cfg.Name, cfg.Slaves) {
		rollback()
		return ProbeLAGs(), true, fmt.Errorf("LAG healthcheck failed, rolled back")
	}
	return ProbeLAGs(), false, nil
}

func lagHealth(name string, want []string) bool {
	wantSet := map[string]bool{}
	for _, s := range want {
		wantSet[s] = true
	}
	for range 8 {
		got := map[string]bool{}
		for _, s := range lagSysfsSlaves(lagDeviceName(name)) {
			got[s] = true
		}
		ok := len(got) > 0
		for s := range wantSet {
			if !got[s] {
				ok = false
			}
		}
		for s := range got {
			if !wantSet[s] {
				ok = false
			}
		}
		if _, err := os.Stat("/sys/class/net/br-lan"); err == nil && ok {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

// DeleteLAG removes a LAG and returns its ports to the bridge.
func DeleteLAG(name string) (*LAGProbe, bool, error) {
	entry := lagFindEntry(name)
	if entry == nil {
		return ProbeLAGs(), false, fmt.Errorf("LAG %q not found", name)
	}
	bridgeSec := lagBridgeSection()
	if bridgeSec == "" {
		return ProbeLAGs(), false, fmt.Errorf("br-lan bridge not found")
	}

	snap, err := executor.Snapshot("network")
	if err != nil {
		return ProbeLAGs(), false, fmt.Errorf("snapshot network: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("network", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "restart"}})
	}

	currentBridgePorts := map[string]bool{}
	for _, p := range lagBridgePorts(bridgeSec) {
		currentBridgePorts[p] = true
	}

	var ops []executor.Op
	ops = append(ops, executor.Op{Kind: "ifdown", Args: []string{name}})
	ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"network." + name}})
	ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{"network." + bridgeSec + ".ports", lagDeviceName(name)}})
	for _, s := range entry.Slaves {
		if !currentBridgePorts[s] {
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"network." + bridgeSec + ".ports", s}})
		}
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"network"}})

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeLAGs(), true, err
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})

	// The bond device created via sysfs is not always torn down by netifd
	// (verified on 25.12.5: bond-lag0 lingered after the interface was gone).
	dev := lagDeviceName(name)
	if _, err := os.Stat("/sys/class/net/" + dev); err == nil {
		_ = os.WriteFile("/sys/class/net/bonding_masters", []byte("-"+dev), 0o200)
	}

	// Healthcheck: ports back in the bridge, bond gone.
	ok := func() bool {
		for range 5 {
			if _, err := os.Stat("/sys/class/net/" + dev); err != nil {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}()
	if !ok {
		rollback()
		return ProbeLAGs(), true, fmt.Errorf("LAG removal healthcheck failed, rolled back")
	}
	return ProbeLAGs(), false, nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
