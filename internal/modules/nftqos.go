package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	nftQoSConfigDir  = "/etc/netgrip"
	nftQoSConfigFile = nftQoSConfigDir + "/qos_limits.json"
	nftQoSRulesFile  = nftQoSConfigDir + "/qos_limits.nft"
	nftQoSBackupExt  = ".bak"
	nftQoSTable      = "netgrip_qos"
)

// NftQoSLimit is one per-device bandwidth limit.
type NftQoSLimit struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Download int    `json:"download"` // Mbps
	Upload   int    `json:"upload"`   // Mbps
}

// NftQoSProbe is the read-only state of per-device QoS.
type NftQoSProbe struct {
	Applicable bool                   `json:"applicable"`
	Limits     map[string]NftQoSLimit `json:"limits"`
}

// NftQoSSetRequest is the body for creating or updating a limit.
type NftQoSSetRequest struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Download int    `json:"download"`
	Upload   int    `json:"upload"`
}

// hasWan reports whether the router has a WAN interface (i.e. is the gateway).
func hasWan() bool {
	out, err := exec.Command("sh", "-c", "ubus call network.interface.wan status 2>/dev/null | grep -q l3_device").CombinedOutput()
	_ = out
	return err == nil
}

// ProbeNftQoS returns the current per-device QoS state.
func ProbeNftQoS() *NftQoSProbe {
	limits, _ := loadNftQoSLimits()
	if limits == nil {
		limits = map[string]NftQoSLimit{}
	}
	return &NftQoSProbe{
		Applicable: hasWan(),
		Limits:     limits,
	}
}

// SetNftQoSLimit sets or updates a per-device limit and applies the ruleset.
// Rates are in Mbps. A zero or negative rate disables that direction.
func SetNftQoSLimit(req NftQoSSetRequest) (*NftQoSProbe, bool, error) {
	if req.MAC == "" {
		return ProbeNftQoS(), false, fmt.Errorf("mac is required")
	}
	if req.IP == "" {
		return ProbeNftQoS(), false, fmt.Errorf("ip is required")
	}
	if !hasWan() {
		return ProbeNftQoS(), false, fmt.Errorf("per-device QoS only applies to the gateway router")
	}

	limits, err := loadNftQoSLimits()
	if err != nil {
		limits = map[string]NftQoSLimit{}
	}

	if req.Download <= 0 && req.Upload <= 0 {
		delete(limits, req.MAC)
	} else {
		limits[req.MAC] = NftQoSLimit{
			MAC:      req.MAC,
			IP:       req.IP,
			Download: req.Download,
			Upload:   req.Upload,
		}
	}

	if err := saveAndApplyLimits(limits); err != nil {
		return ProbeNftQoS(), true, err
	}
	return ProbeNftQoS(), false, nil
}

// RemoveNftQoSLimit removes a per-device limit and reapplies the ruleset.
func RemoveNftQoSLimit(mac string) (*NftQoSProbe, bool, error) {
	if !hasWan() {
		return ProbeNftQoS(), false, fmt.Errorf("per-device QoS only applies to the gateway router")
	}
	limits, _ := loadNftQoSLimits()
	if limits == nil {
		limits = map[string]NftQoSLimit{}
	}
	delete(limits, mac)
	if err := saveAndApplyLimits(limits); err != nil {
		return ProbeNftQoS(), true, err
	}
	return ProbeNftQoS(), false, nil
}

func loadNftQoSLimits() (map[string]NftQoSLimit, error) {
	data, err := os.ReadFile(nftQoSConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]NftQoSLimit{}, nil
		}
		return nil, err
	}
	var limits []NftQoSLimit
	if err := json.Unmarshal(data, &limits); err != nil {
		return nil, err
	}
	out := make(map[string]NftQoSLimit, len(limits))
	for _, l := range limits {
		out[l.MAC] = l
	}
	return out, nil
}

func saveNftQoSLimits(limits map[string]NftQoSLimit) error {
	if err := os.MkdirAll(nftQoSConfigDir, 0o755); err != nil {
		return err
	}
	list := make([]NftQoSLimit, 0, len(limits))
	for _, l := range limits {
		list = append(list, l)
	}
	sort.Slice(list, func(i, j int) bool { return strings.Compare(list[i].MAC, list[j].MAC) < 0 })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(nftQoSConfigFile, data)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// generateNftRuleset creates the nftables rules from the configured limits.
// It only ever contains the table declaration: dropping a previous version of
// the table is applyNftRuleset's job (declarative nft -f syntax merges into a
// live table instead of replacing it).
func generateNftRuleset(limits map[string]NftQoSLimit) string {
	var active []NftQoSLimit
	for _, l := range limits {
		if l.Download > 0 || l.Upload > 0 {
			active = append(active, l)
		}
	}
	if len(active) == 0 {
		return ""
	}
	sort.Slice(active, func(i, j int) bool { return strings.Compare(active[i].IP, active[j].IP) < 0 })

	var b strings.Builder
	b.WriteString(fmt.Sprintf("table inet %s {\n", nftQoSTable))
	b.WriteString("  chain upload {\n")
	b.WriteString("    type filter hook prerouting priority -150; policy accept;\n")
	for _, l := range active {
		if l.Upload > 0 {
			rate := l.Upload * 125 // Mbps -> kbytes/s
			b.WriteString(fmt.Sprintf("    ip saddr %s limit rate over %d kbytes/second drop\n", l.IP, rate))
		}
	}
	b.WriteString("  }\n")
	b.WriteString("  chain download {\n")
	b.WriteString("    type filter hook postrouting priority -150; policy accept;\n")
	for _, l := range active {
		if l.Download > 0 {
			rate := l.Download * 125
			b.WriteString(fmt.Sprintf("    ip daddr %s limit rate over %d kbytes/second drop\n", l.IP, rate))
		}
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

// saveAndApplyLimits persists the limits, writes the rules file and applies it.
func saveAndApplyLimits(limits map[string]NftQoSLimit) error {
	if err := saveNftQoSLimits(limits); err != nil {
		return err
	}

	rules := generateNftRuleset(limits)
	if err := writeFileAtomic(nftQoSRulesFile, []byte(rules)); err != nil {
		return err
	}

	snapshot, err := snapshotNftRuleset()
	if err != nil {
		return err
	}

	if err := replaceNftQoSTable(nftQoSRulesFile); err != nil {
		_ = restoreNftRuleset(snapshot)
		return err
	}

	if !healthcheckNftQoS(len(limits) > 0) {
		_ = restoreNftRuleset(snapshot)
		return fmt.Errorf("nftables healthcheck failed after applying qos rules")
	}

	return nil
}

func snapshotNftRuleset() (string, error) {
	out, err := exec.Command("nft", "list", "ruleset").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func applyNftRuleset(path string) error {
	out, err := exec.Command("nft", "-f", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft -f %s: %w (%s)", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// replaceNftQoSTable applies the rules file over a clean slate: the previous
// netgrip_qos table (if any) is deleted first, because nft -f merges
// declarations into an existing table and stale rules would accumulate.
func replaceNftQoSTable(path string) error {
	if err := exec.Command("nft", "list", "table", "inet", nftQoSTable).Run(); err == nil {
		if out, err := exec.Command("nft", "delete", "table", "inet", nftQoSTable).CombinedOutput(); err != nil {
			return fmt.Errorf("nft delete table %s: %w (%s)", nftQoSTable, err, strings.TrimSpace(string(out)))
		}
	}
	return applyNftRuleset(path)
}

func restoreNftRuleset(snapshot string) error {
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(snapshot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft restore: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func healthcheckNftQoS(shouldExist bool) bool {
	for range 3 {
		out, err := exec.Command("nft", "list", "table", "inet", nftQoSTable).CombinedOutput()
		exists := err == nil && strings.Contains(string(out), nftQoSTable)
		if exists == shouldExist {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// ApplyNftQoSAtBoot reapplies the persisted limits; called on NetGrip startup.
func ApplyNftQoSAtBoot() {
	limits, err := loadNftQoSLimits()
	if err != nil || len(limits) == 0 {
		return
	}
	if !hasWan() {
		return
	}
	rules := generateNftRuleset(limits)
	if err := writeFileAtomic(nftQoSRulesFile, []byte(rules)); err != nil {
		return
	}
	_ = replaceNftQoSTable(nftQoSRulesFile)
}

// InitNftQoS applies persisted limits on startup.
func init() {
	ApplyNftQoSAtBoot()
}
