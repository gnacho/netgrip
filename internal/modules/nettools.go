package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

var uciConfigs = []string{
	"network", "dhcp", "firewall", "wireless",
	"system", "dropbear", "uhttpd", "sqm", "ddns",
}

const snapshotDir = "/etc/netgrip/snapshots"

type ConfigSnapshot struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Configs   int    `json:"configs"`
}

type ConfigDiff struct {
	Config string `json:"config"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type DriftLine struct {
	Kind string `json:"kind"` // added | removed
	Text string `json:"text"`
}

type DriftConfig struct {
	Config string      `json:"config"`
	Lines  []DriftLine `json:"lines"`
}

type DriftProbe struct {
	HasBaseline bool          `json:"has_baseline"`
	SnapshotID  string        `json:"snapshot_id"`
	SnapshotTS  int64         `json:"snapshot_ts"`
	Changes     int           `json:"changes"`
	Configs     []DriftConfig `json:"configs"`
}

func ListSnapshots() []ConfigSnapshot {
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil
	}
	var snaps []ConfigSnapshot
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(snapshotDir, e.Name()))
		snaps = append(snaps, ConfigSnapshot{
			ID:        e.Name(),
			Timestamp: info.ModTime().Unix(),
			Configs:   len(files),
		})
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Timestamp > snaps[j].Timestamp })
	return snaps
}

func CreateSnapshot() (*ConfigSnapshot, error) {
	id := time.Now().Format("20060102-150405")
	dir := filepath.Join(snapshotDir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	count := 0
	for _, cfg := range uciConfigs {
		content, err := executor.Snapshot(cfg)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, cfg+".uci"), []byte(content), 0644); err != nil {
			continue
		}
		count++
	}
	if count == 0 {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("no configs exported")
	}
	return &ConfigSnapshot{ID: id, Timestamp: time.Now().Unix(), Configs: count}, nil
}

func DeleteSnapshot(id string) error {
	if !isValidSnapshotID(id) {
		return fmt.Errorf("invalid snapshot id")
	}
	return os.RemoveAll(filepath.Join(snapshotDir, id))
}

func DiffSnapshots(from, to string) ([]ConfigDiff, error) {
	if !isValidSnapshotID(from) || !isValidSnapshotID(to) {
		return nil, fmt.Errorf("invalid snapshot id")
	}
	fromDir := filepath.Join(snapshotDir, from)
	toDir := filepath.Join(snapshotDir, to)
	var diffs []ConfigDiff
	for _, cfg := range uciConfigs {
		before, _ := os.ReadFile(filepath.Join(fromDir, cfg+".uci"))
		after, _ := os.ReadFile(filepath.Join(toDir, cfg+".uci"))
		if string(before) == string(after) {
			continue
		}
		diffs = append(diffs, ConfigDiff{
			Config: cfg,
			Before: string(before),
			After:  string(after),
		})
	}
	return diffs, nil
}

func RollbackSnapshot(id string) error {
	if !isValidSnapshotID(id) {
		return fmt.Errorf("invalid snapshot id")
	}
	dir := filepath.Join(snapshotDir, id)
	restored := 0
	for _, cfg := range uciConfigs {
		content, err := os.ReadFile(filepath.Join(dir, cfg+".uci"))
		if err != nil {
			continue
		}
		if err := executor.Restore(cfg, string(content)); err != nil {
			continue
		}
		restored++
	}
	if restored == 0 {
		return fmt.Errorf("no configs restored from snapshot %s", id)
	}
	executor.Run(executor.Op{Kind: "initd", Args: []string{"network", "reload"}})
	if executor.ServiceEnabled("firewall") {
		executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	if executor.ServiceRunning("dnsmasq") {
		executor.Run(executor.Op{Kind: "initd", Args: []string{"dnsmasq", "restart"}})
	}
	return nil
}

func isValidSnapshotID(id string) bool {
	if len(id) != 15 || id[8] != '-' {
		return false
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

type LinkBounceResult struct {
	Iface string `json:"iface"`
	Ok    bool   `json:"ok"`
}

func BlockPort(iface string, blocked bool) (*LinkBounceResult, error) {
	if err := executor.Validate(executor.Op{Kind: "ip_link", Args: []string{iface, "down"}}); err != nil {
		return nil, fmt.Errorf("invalid interface: %s", iface)
	}
	ports := bridgePorts()
	if !ports[iface] {
		return nil, fmt.Errorf("interface %s is not a bridge port", iface)
	}
	action := "down"
	if !blocked {
		action = "up"
	}
	if err := executor.Run(executor.Op{Kind: "ip_link", Args: []string{iface, action}}); err != nil {
		return nil, err
	}
	return &LinkBounceResult{Iface: iface, Ok: true}, nil
}

func BounceLink(iface string) (*LinkBounceResult, error) {
	if err := executor.Validate(executor.Op{Kind: "ip_link", Args: []string{iface, "down"}}); err != nil {
		return nil, fmt.Errorf("invalid interface: %s", iface)
	}
	ports := bridgePorts()
	if !ports[iface] {
		return nil, fmt.Errorf("interface %s is not a bridge port", iface)
	}
	if err := executor.Run(executor.Op{Kind: "ip_link", Args: []string{iface, "down"}}); err != nil {
		return nil, err
	}
	time.Sleep(2 * time.Second)
	if err := executor.Run(executor.Op{Kind: "ip_link", Args: []string{iface, "up"}}); err != nil {
		return nil, err
	}
	return &LinkBounceResult{Iface: iface, Ok: true}, nil
}

func bridgePorts() map[string]bool {
	ports := map[string]bool{}
	entries, err := os.ReadDir("/sys/class/net/br-lan/brif")
	if err != nil {
		return ports
	}
	for _, e := range entries {
		ports[e.Name()] = true
	}
	return ports
}

type LoopEntry struct {
	MAC   string   `json:"mac"`
	Ports []string `json:"ports"`
}

type LoopResult struct {
	Loops  []LoopEntry `json:"loops"`
	HasHub bool        `json:"has_hub"`
}

func DetectLoops() *LoopResult {
	fdb := bridgeFdb()
	macPorts := map[string][]string{}
	for port, macs := range fdb {
		for _, mac := range macs {
			mac = strings.ToLower(mac)
			macPorts[mac] = append(macPorts[mac], port)
		}
	}
	var loops []LoopEntry
	hasHub := false
	wifiPorts := map[string]bool{}
	for name := range bridgePorts() {
		if strings.HasPrefix(name, "phy") || strings.HasPrefix(name, "wlan") {
			wifiPorts[name] = true
		}
	}
	for mac, ports := range macPorts {
		wiredOnly := true
		for _, p := range ports {
			if wifiPorts[p] {
				wiredOnly = false
				break
			}
		}
		if !wiredOnly || len(ports) < 2 {
			continue
		}
		loops = append(loops, LoopEntry{MAC: mac, Ports: ports})
	}
	for _, ports := range macPorts {
		wired := 0
		for _, p := range ports {
			if !wifiPorts[p] {
				wired++
			}
		}
		if wired > 5 {
			hasHub = true
			break
		}
	}
	return &LoopResult{Loops: loops, HasHub: hasHub}
}

type IGMPProbe struct {
	Applicable bool `json:"applicable"`
	Enabled    bool `json:"enabled"`
}

func ProbeIGMP() *IGMPProbe {
	data, err := os.ReadFile("/sys/class/net/br-lan/bridge/multicast_snooping")
	if err != nil {
		return &IGMPProbe{Applicable: false}
	}
	return &IGMPProbe{
		Applicable: true,
		Enabled:    strings.TrimSpace(string(data)) == "1",
	}
}

func SetIGMP(enabled bool) (*IGMPProbe, error) {
	probe := ProbeIGMP()
	if !probe.Applicable {
		return nil, fmt.Errorf("IGMP snooping not available")
	}
	val := "0"
	if enabled {
		val = "1"
	}
	if err := os.WriteFile("/sys/class/net/br-lan/bridge/multicast_snooping", []byte(val), 0644); err != nil {
		return nil, fmt.Errorf("write igmp: %w", err)
	}
	return ProbeIGMP(), nil
}

func init() {
	os.MkdirAll(snapshotDir, 0755)
}

func ProbeDrift() *DriftProbe {
	snaps := ListSnapshots()
	if len(snaps) == 0 {
		return &DriftProbe{HasBaseline: false}
	}
	latest := snaps[0]
	dir := filepath.Join(snapshotDir, latest.ID)
	var changed []DriftConfig
	for _, cfg := range uciConfigs {
		baseContent, err := os.ReadFile(filepath.Join(dir, cfg+".uci"))
		if err != nil {
			continue
		}
		current, err := executor.Snapshot(cfg)
		if err != nil {
			continue
		}
		lines := diffLines(string(baseContent), current)
		if len(lines) > 0 {
			changed = append(changed, DriftConfig{Config: cfg, Lines: lines})
		}
	}
	return &DriftProbe{
		HasBaseline: true,
		SnapshotID:  latest.ID,
		SnapshotTS:  latest.Timestamp,
		Changes:     len(changed),
		Configs:     changed,
	}
}

func diffLines(before, after string) []DriftLine {
	beforeSet := map[string]bool{}
	afterSet := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(before), "\n") {
		if l != "" {
			beforeSet[l] = true
		}
	}
	for _, l := range strings.Split(strings.TrimSpace(after), "\n") {
		if l != "" {
			afterSet[l] = true
		}
	}
	var lines []DriftLine
	for l := range afterSet {
		if !beforeSet[l] {
			lines = append(lines, DriftLine{Kind: "added", Text: l})
		}
	}
	for l := range beforeSet {
		if !afterSet[l] {
			lines = append(lines, DriftLine{Kind: "removed", Text: l})
		}
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Kind != lines[j].Kind {
			return lines[i].Kind < lines[j].Kind
		}
		return lines[i].Text < lines[j].Text
	})
	return lines
}
