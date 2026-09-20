package modules

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// banIP 1.5.x integration (#351). banIP owns its nft table: NetGrip only
// reads UCI (/etc/config/banip) and drives /etc/init.d/banip
// (start/stop/reload plus the read-only report/status/search commands).
// It never writes nft rules or touches the banIP table.
//
// Options used below are the ones documented for banIP 1.5.x
// (openwrt/packages net/banip README): ban_enabled, ban_feed,
// ban_feedin/ban_feedout/ban_feedinout, ban_nftcount, ban_loglimit,
// ban_logcount, ban_logterm. Local lists live in
// /etc/banip/banip.allowlist and /etc/banip/banip.blocklist.

const (
	banipAllowlistPath = "/etc/banip/banip.allowlist"
	banipBlocklistPath = "/etc/banip/banip.blocklist"
)

// BanipFeed is one configured blocklist feed. Direction is "in", "out" or
// "inout" when overridden via ban_feedin/ban_feedout/ban_feedinout; "" means
// the feed default from the banIP feed table applies.
type BanipFeed struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Direction string `json:"direction"`
}

// BanipSetStat is one row of the banIP Set report.
type BanipSetStat struct {
	Name       string `json:"name"`
	Elements   int64  `json:"elements"`
	PacketsIn  int64  `json:"packets_in"`
	PacketsOut int64  `json:"packets_out"`
	LocalAllow bool   `json:"local_allow"`
	LocalBlock bool   `json:"local_block"`
}

// BanipDos holds the DoS counters from the report header plus the
// thresholds from the runtime status (nft_info limits).
type BanipDos struct {
	SynPackets        int64 `json:"syn_packets"`
	UdpPackets        int64 `json:"udp_packets"`
	IcmpPackets       int64 `json:"icmp_packets"`
	InvalidCtPackets  int64 `json:"invalid_ct_packets"`
	InvalidTcpPackets int64 `json:"invalid_tcp_packets"`
	SynLimit          int   `json:"syn_limit"`
	UdpLimit          int   `json:"udp_limit"`
	IcmpLimit         int   `json:"icmp_limit"`
}

// BanipReport is the parsed output of `/etc/init.d/banip report`.
// Packet counters are only real with ban_nftcount=1; when disabled they
// stay 0 and the UI omits them.
type BanipReport struct {
	Parsed     bool           `json:"parsed"`
	Timestamp  string         `json:"timestamp"`
	Sets       []BanipSetStat `json:"sets"`
	TotalIPs   int64          `json:"total_ips"`   // elements of feed + blocklist sets
	PacketsIn  int64          `json:"packets_in"`  // sum over sets that block inbound
	PacketsOut int64          `json:"packets_out"` // sum over sets that block outbound
	AutoAllow  int64          `json:"auto_allow"`  // auto-added IPs to allowlist
	AutoBlock  int64          `json:"auto_block"`  // auto-added IPs to blocklist
	Dos        BanipDos       `json:"dos"`
}

// BanipProbe is the API view of the banIP page.
type BanipProbe struct {
	Installed      bool         `json:"installed"`
	Enabled        bool         `json:"enabled"` // ban_enabled
	Running        bool         `json:"running"`
	NftCount       bool         `json:"nft_count"`
	Applicable     bool         `json:"applicable"` // gateway with an uplink
	Version        string       `json:"version"`
	MemAvailableMB int64        `json:"mem_available_mb"` // from last_run; 0 = unknown
	Feeds          []BanipFeed  `json:"feeds"`
	Report         *BanipReport `json:"report,omitempty"`
	Allowlist      []string     `json:"allowlist"`
	Blocklist      []string     `json:"blocklist"`
}

// banipInstalled reports whether the banIP init script is present.
func banipInstalled() bool {
	_, err := os.Stat("/etc/init.d/banip")
	return err == nil
}

// banipGlobal parses `uci show banip` once (single fork, the pattern from
// #343) and returns the global section options.
func banipGlobal() map[string][]string {
	out, err := exec.Command("uci", "show", "banip").Output()
	if err != nil {
		return nil
	}
	sections := parseUCIShow(string(out), "banip")
	// The stock config declares `config banip 'global'`.
	if s, ok := sections["global"]; ok {
		return s.Options
	}
	// Fall back to whatever section carries banip options.
	for _, s := range sections {
		if len(s.Options) > 0 {
			return s.Options
		}
	}
	return nil
}

func optHas(opts map[string][]string, key, val string) bool {
	for _, v := range opts[key] {
		if v == val {
			return true
		}
	}
	return false
}

// banipFeeds builds the feed list from the global UCI options.
func banipFeeds(opts map[string][]string) []BanipFeed {
	seen := map[string]bool{}
	var feeds []BanipFeed
	add := func(name, direction string, enabled bool) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		feeds = append(feeds, BanipFeed{Name: name, Enabled: enabled, Direction: direction})
	}
	for _, f := range opts["ban_feed"] {
		d := ""
		switch {
		case optHas(opts, "ban_feedin", f):
			d = "in"
		case optHas(opts, "ban_feedout", f):
			d = "out"
		case optHas(opts, "ban_feedinout", f):
			d = "inout"
		}
		add(f, d, true)
	}
	// Feeds referenced only in direction lists count as configured but off.
	for _, key := range []string{"ban_feedin", "ban_feedout", "ban_feedinout"} {
		for _, f := range opts[key] {
			if !seen[f] {
				d := "in"
				if key == "ban_feedout" {
					d = "out"
				} else if key == "ban_feedinout" {
					d = "inout"
				}
				add(f, d, false)
			}
		}
	}
	return feeds
}

// banipReadList reads a local allow/blocklist, one entry per line, skipping
// comments and blanks.
func banipReadList(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

var (
	reBanipHeaderInt    = regexp.MustCompile(`^\s{4}blocked ([a-z- ]+?)\s*:\s*([0-9 ]+)\s*$`)
	reBanipAuto         = regexp.MustCompile(`^\s{4}auto-added IPs to (allow|block)list:\s*([0-9 ]+)\s*$`)
	reBanipSetRow       = regexp.MustCompile(`^\s{4}(\S+\.(?:v4|v6|v4MAC|v6MAC))\s*\|\s*([0-9 ]+)\s*\|\s*([^|]*)\|\s*([^|]*)\|`)
	reBanipOnPackets    = regexp.MustCompile(`ON:\s*([0-9 ]+)`)
	reBanipTimestamp    = regexp.MustCompile(`^\s{4}Timestamp:\s*(.+?)\s*$`)
	reStatusKV          = regexp.MustCompile(`^\s*\+\s*([a-z_]+)\s*:\s*(.*)$`)
	reStatusLimits      = regexp.MustCompile(`limit \(icmp/syn/udp\):\s*(\d+)/(\d+)/(\d+)`)
	reBanipVersionValue = regexp.MustCompile(`^[0-9][0-9a-zA-Z.-]*$`)
	reStatusMem         = regexp.MustCompile(`memory:\s*([0-9.]+)\s*MB`)
	reStatusElement     = regexp.MustCompile(`^([0-9 ]+)`)
)

// banipNum parses the report's space-grouped counters ("128 751").
func banipNum(s string) int64 {
	n, _ := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), 10, 64)
	return n
}

// parseBanipReportText parses `/etc/init.d/banip report` plain output.
// Split from the command so tests run without a router; the fixture is the
// real 1.5.x format from the banIP README.
func parseBanipReportText(out string) *BanipReport {
	r := &BanipReport{Sets: []BanipSetStat{}}
	for _, line := range strings.Split(out, "\n") {
		if m := reBanipTimestamp.FindStringSubmatch(line); m != nil {
			r.Timestamp = m[1]
			r.Parsed = true
			continue
		}
		if m := reBanipAuto.FindStringSubmatch(line); m != nil {
			if m[1] == "allow" {
				r.AutoAllow = banipNum(m[2])
			} else {
				r.AutoBlock = banipNum(m[2])
			}
			continue
		}
		if m := reBanipHeaderInt.FindStringSubmatch(line); m != nil {
			v := banipNum(m[2])
			switch m[1] {
			case "syn-flood packets":
				r.Dos.SynPackets = v
			case "udp-flood packets":
				r.Dos.UdpPackets = v
			case "icmp-flood packets":
				r.Dos.IcmpPackets = v
			case "invalid ct packets":
				r.Dos.InvalidCtPackets = v
			case "invalid tcp packets":
				r.Dos.InvalidTcpPackets = v
			}
			continue
		}
		m := reBanipSetRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		stat := BanipSetStat{Name: m[1], Elements: banipNum(m[2])}
		stat.LocalAllow = strings.HasPrefix(stat.Name, "allowlist.")
		stat.LocalBlock = strings.HasPrefix(stat.Name, "blocklist.")
		if p := reBanipOnPackets.FindStringSubmatch(m[3]); p != nil {
			stat.PacketsIn = banipNum(p[1])
		}
		if p := reBanipOnPackets.FindStringSubmatch(m[4]); p != nil {
			stat.PacketsOut = banipNum(p[1])
		}
		r.Sets = append(r.Sets, stat)
	}
	for _, s := range r.Sets {
		if s.LocalAllow {
			continue
		}
		r.TotalIPs += s.Elements
		r.PacketsIn += s.PacketsIn
		r.PacketsOut += s.PacketsOut
	}
	return r
}

// parseBanipStatusText extracts version, element count, DoS thresholds and
// the memory available after the last run from `/etc/init.d/banip status`.
func parseBanipStatusText(out string) (version string, elements int64, memMB int64, icmpLimit, synLimit, udpLimit int) {
	for _, line := range strings.Split(out, "\n") {
		m := reStatusKV.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch m[1] {
		case "version":
			v := strings.TrimSpace(m[2])
			if reBanipVersionValue.MatchString(v) {
				version = v
			}
		case "element_count":
			if v := reStatusElement.FindStringSubmatch(strings.TrimSpace(m[2])); v != nil {
				elements = banipNum(v[1])
			}
		case "nft_info":
			if v := reStatusLimits.FindStringSubmatch(m[2]); v != nil {
				icmpLimit, _ = strconv.Atoi(v[1])
				synLimit, _ = strconv.Atoi(v[2])
				udpLimit, _ = strconv.Atoi(v[3])
			}
		case "last_run":
			if v := reStatusMem.FindStringSubmatch(m[2]); v != nil {
				f, _ := strconv.ParseFloat(v[1], 64)
				memMB = int64(f)
			}
		}
	}
	return version, elements, memMB, icmpLimit, synLimit, udpLimit
}

// ProbeBanIP reads the full banIP state. Every read is fail-soft: a missing
// report or status is an omitted field, never an error.
func ProbeBanIP() *BanipProbe {
	p := &BanipProbe{
		Installed:  banipInstalled(),
		Applicable: hasUplink(),
		Feeds:      []BanipFeed{},
		Allowlist:  []string{},
		Blocklist:  []string{},
	}
	if !p.Installed {
		return p
	}
	opts := banipGlobal()
	p.Enabled = len(opts["ban_enabled"]) > 0 && opts["ban_enabled"][0] == "1"
	p.NftCount = len(opts["ban_nftcount"]) > 0 && opts["ban_nftcount"][0] == "1"
	p.Feeds = banipFeeds(opts)
	p.Running = executor.ServiceRunning("banip")

	if out, err := exec.Command("/etc/init.d/banip", "status").Output(); err == nil {
		v, _, mem, il, sl, ul := parseBanipStatusText(string(out))
		p.Version = v
		p.MemAvailableMB = mem
		if out, err := exec.Command("/etc/init.d/banip", "report").Output(); err == nil {
			rep := parseBanipReportText(string(out))
			rep.Dos.IcmpLimit = il
			rep.Dos.SynLimit = sl
			rep.Dos.UdpLimit = ul
			p.Report = rep
		}
	}

	p.Allowlist = banipReadList(banipAllowlistPath)
	p.Blocklist = banipReadList(banipBlocklistPath)
	return p
}

// validBanipActions are the service actions the page exposes. enable/disable
// toggle the ban_enabled master switch in UCI; start/stop/reload/restart map
// straight to the init script.
var validBanipActions = map[string]bool{
	"start": true, "stop": true, "reload": true, "restart": true,
	"enable": true, "disable": true,
}

// BanipAction runs one service action through the executor and verifies the
// resulting state, reversing the action as rollback when the healthcheck
// fails. reload/restart re-download the feeds; start/stop only restore from
// the local backups (that is banIP behaviour, surfaced in the UI note).
func BanipAction(action string) (*BanipProbe, bool, error) {
	if !validBanipActions[action] {
		return ProbeBanIP(), false, fmt.Errorf("unsupported action %q", action)
	}
	if !banipInstalled() {
		return ProbeBanIP(), false, fmt.Errorf("banip is not installed")
	}
	before := ProbeBanIP()
	var ops []executor.Op
	switch action {
	case "enable":
		snap, _ := executor.Snapshot("banip")
		ops = []executor.Op{
			{Kind: "uci_set", Args: []string{"banip.global.ban_enabled", "1"}},
			{Kind: "uci_commit", Args: []string{"banip"}},
			{Kind: "initd", Args: []string{"banip", "reload"}},
		}
		if err := executor.Apply(ops, nil); err != nil {
			if snap != "" {
				_ = executor.Restore("banip", snap)
			}
			return ProbeBanIP(), true, err
		}
		return banipActionVerify("enable", before)
	case "disable":
		snap, _ := executor.Snapshot("banip")
		ops = []executor.Op{
			{Kind: "uci_set", Args: []string{"banip.global.ban_enabled", "0"}},
			{Kind: "uci_commit", Args: []string{"banip"}},
			{Kind: "initd", Args: []string{"banip", "stop"}},
		}
		if err := executor.Apply(ops, nil); err != nil {
			if snap != "" {
				_ = executor.Restore("banip", snap)
			}
			return ProbeBanIP(), true, err
		}
		return banipActionVerify("disable", before)
	default:
		ops = []executor.Op{{Kind: "initd", Args: []string{"banip", action}}}
		if err := executor.Apply(ops, nil); err != nil {
			return ProbeBanIP(), false, err
		}
		return banipActionVerify(action, before)
	}
}

// banipActionVerify healthchecks a service action by polling the running
// state. reload/restart keep the previous running state; start must come up,
// stop must go down, enable must come up, disable must go down.
func banipActionVerify(action string, before *BanipProbe) (*BanipProbe, bool, error) {
	want := before.Running
	switch action {
	case "start", "enable":
		want = true
	case "stop", "disable":
		want = false
	}
	for i := 0; i < 10; i++ {
		p := ProbeBanIP()
		if action == "enable" {
			if p.Enabled && p.Running {
				return p, false, nil
			}
		} else if action == "disable" {
			if !p.Enabled && !p.Running {
				return p, false, nil
			}
		} else if p.Running == want {
			return p, false, nil
		}
		time.Sleep(time.Second)
	}
	probe := ProbeBanIP()
	reverse := map[string]string{"start": "stop", "stop": "start", "enable": "disable", "disable": "enable"}
	if r, ok := reverse[action]; ok {
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"banip", r}})
		if action == "enable" || action == "disable" {
			val := "0"
			if action == "disable" {
				val = "1"
			}
			_ = executor.Run(executor.Op{Kind: "uci_set", Args: []string{"banip.global.ban_enabled", val}})
			_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"banip"}})
		}
	}
	return probe, true, fmt.Errorf("healthcheck failed after %q, rolled back", action)
}

// BanipFeedsConfig is the write model for the feeds endpoint: the full
// desired feed list (enabled + direction) plus optional master switches.
type BanipFeedsConfig struct {
	Feeds    []BanipFeed `json:"feeds"`
	Enabled  *bool       `json:"enabled,omitempty"`
	NftCount *bool       `json:"nft_count,omitempty"`
}

// validFeedName guards the UCI list values.
var reBanipFeedName = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

func validBanipDirection(d string) bool {
	return d == "" || d == "in" || d == "out" || d == "inout"
}

// banipFeedOps builds the UCI ops that make the config match the desired
// feed set: the ban_feed and direction lists are rewritten wholesale
// (uci_delete + uci_add_list), which is idempotent by construction.
func banipFeedOps(cfg BanipFeedsConfig) ([]executor.Op, error) {
	var ops []executor.Op
	for _, key := range []string{"ban_feed", "ban_feedin", "ban_feedout", "ban_feedinout"} {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"banip.global." + key}})
	}
	for _, f := range cfg.Feeds {
		if !reBanipFeedName.MatchString(f.Name) {
			return nil, fmt.Errorf("invalid feed name %q", f.Name)
		}
		if !validBanipDirection(f.Direction) {
			return nil, fmt.Errorf("invalid direction %q for feed %q", f.Direction, f.Name)
		}
		if f.Enabled {
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"banip.global.ban_feed", f.Name}})
		}
		switch f.Direction {
		case "in":
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"banip.global.ban_feedin", f.Name}})
		case "out":
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"banip.global.ban_feedout", f.Name}})
		case "inout":
			ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"banip.global.ban_feedinout", f.Name}})
		}
	}
	if cfg.Enabled != nil {
		v := "0"
		if *cfg.Enabled {
			v = "1"
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"banip.global.ban_enabled", v}})
	}
	if cfg.NftCount != nil {
		v := "0"
		if *cfg.NftCount {
			v = "1"
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"banip.global.ban_nftcount", v}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"banip"}},
		executor.Op{Kind: "initd", Args: []string{"banip", "reload"}},
	)
	return ops, nil
}

// banipFeedsMatch compares the live UCI options with the desired config.
func banipFeedsMatch(opts map[string][]string, cfg BanipFeedsConfig) bool {
	wantEnabled := map[string]string{}
	for _, f := range cfg.Feeds {
		d := f.Direction
		if !f.Enabled {
			d = "off"
		}
		wantEnabled[f.Name] = d
	}
	if len(opts["ban_feed"]) != 0 || len(wantEnabled) != 0 {
		got := map[string]bool{}
		for _, f := range opts["ban_feed"] {
			got[f] = true
		}
		for name, d := range wantEnabled {
			if d != "off" && !got[name] {
				return false
			}
			if d == "off" && got[name] {
				return false
			}
		}
		for _, f := range opts["ban_feed"] {
			if _, ok := wantEnabled[f]; !ok {
				return false
			}
		}
	}
	dirList := func(key string) map[string]bool {
		m := map[string]bool{}
		for _, v := range opts[key] {
			m[v] = true
		}
		return m
	}
	in, out, inout := dirList("ban_feedin"), dirList("ban_feedout"), dirList("ban_feedinout")
	for name, d := range wantEnabled {
		if d == "off" || d == "" {
			if in[name] || out[name] || inout[name] {
				return false
			}
			continue
		}
		var m map[string]bool
		switch d {
		case "in":
			m = in
		case "out":
			m = out
		case "inout":
			m = inout
		}
		if !m[name] {
			return false
		}
	}
	if cfg.Enabled != nil {
		live := len(opts["ban_enabled"]) > 0 && opts["ban_enabled"][0] == "1"
		if live != *cfg.Enabled {
			return false
		}
	}
	if cfg.NftCount != nil {
		live := len(opts["ban_nftcount"]) > 0 && opts["ban_nftcount"][0] == "1"
		if live != *cfg.NftCount {
			return false
		}
	}
	return true
}

// SetBanipFeeds applies the desired feed configuration with snapshot,
// healthcheck (live UCI must match) and rollback.
func SetBanipFeeds(cfg BanipFeedsConfig) (*BanipProbe, bool, error) {
	if !banipInstalled() {
		return ProbeBanIP(), false, fmt.Errorf("banip is not installed")
	}
	ops, err := banipFeedOps(cfg)
	if err != nil {
		return ProbeBanIP(), false, err
	}
	snap, _ := executor.Snapshot("banip")
	if err := executor.Apply(ops, nil); err != nil {
		if snap != "" {
			_ = executor.Restore("banip", snap)
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"banip", "reload"}})
		return ProbeBanIP(), true, err
	}
	for i := 0; i < 5; i++ {
		if banipFeedsMatch(banipGlobal(), cfg) {
			return ProbeBanIP(), false, nil
		}
		time.Sleep(time.Second)
	}
	if snap != "" {
		_ = executor.Restore("banip", snap)
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"banip", "reload"}})
	return ProbeBanIP(), true, fmt.Errorf("healthcheck failed after feed change, rolled back")
}

// validBanipEntry validates one local list entry: an IPv4/IPv6 address, a
// CIDR range, a MAC address, a domain name, or a MAC followed by an IP
// (banIP's concatenation syntax).
func validBanipEntry(entry string) bool {
	fields := strings.Fields(entry)
	if len(fields) < 1 || len(fields) > 2 {
		return false
	}
	okTok := func(tok string) bool {
		if ip := net.ParseIP(tok); ip != nil {
			return true
		}
		if _, _, err := net.ParseCIDR(tok); err == nil {
			return true
		}
		if _, err := net.ParseMAC(tok); err == nil {
			return true
		}
		return reBanipDomain.MatchString(tok)
	}
	for _, tok := range fields {
		if !okTok(tok) {
			return false
		}
	}
	// Two tokens are only meaningful as MAC + IP.
	if len(fields) == 2 {
		if _, err := net.ParseMAC(fields[0]); err != nil {
			return false
		}
		if net.ParseIP(fields[1]) == nil {
			if _, _, err := net.ParseCIDR(fields[1]); err != nil {
				return false
			}
		}
	}
	return true
}

var reBanipDomain = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)

// banipListPath resolves the list name to its file. Anything else is
// rejected before it can touch the filesystem.
func banipListPath(list string) (string, error) {
	switch list {
	case "allowlist":
		return banipAllowlistPath, nil
	case "blocklist":
		return banipBlocklistPath, nil
	}
	return "", fmt.Errorf("unknown list %q", list)
}

// addBanipEntryContent is the pure planner behind the list add: it appends
// the entry (deduplicated) to the file content. Validation happens before.
func addBanipEntryContent(content, entry string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return content
		}
	}
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return entry + "\n"
	}
	return content + "\n" + entry + "\n"
}

// removeBanipEntryContent is the pure planner behind the list remove.
func removeBanipEntryContent(content, entry string) string {
	kept := []string{}
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != entry {
			kept = append(kept, line)
		}
	}
	out := strings.Join(kept, "\n")
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return strings.TrimRight(out, "\n") + "\n"
}

// writeBanipList writes a local list file with a backup and restore on
// failure. The list files are plain text (not UCI), so the executor's UCI
// snapshot does not apply; the backup lives next to the file.
func writeBanipList(path, content string) error {
	backup := path + ".netgrip-bak"
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(backup, data, 0o600); err != nil {
			return err
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".netgrip-tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func restoreBanipList(path string) {
	backup := path + ".netgrip-bak"
	data, err := os.ReadFile(backup)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
	_ = os.Remove(backup)
}

// BanipListEntries reads one local list without the full probe (the report
// command is too heavy for a per-list refresh).
func BanipListEntries(list string) ([]string, error) {
	path, err := banipListPath(list)
	if err != nil {
		return nil, err
	}
	return banipReadList(path), nil
}

// SetBanipListEntry adds (or removes, remove=true) one entry in a local
// allow/blocklist and reloads banIP so the change takes effect. The reload
// re-processes the feeds (banIP applies local lists on reload), which the
// UI states next to the action.
func SetBanipListEntry(list, entry string, remove bool) (*BanipProbe, bool, error) {
	path, err := banipListPath(list)
	if err != nil {
		return ProbeBanIP(), false, err
	}
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return ProbeBanIP(), false, fmt.Errorf("entry is required")
	}
	if !validBanipEntry(entry) {
		return ProbeBanIP(), false, fmt.Errorf("invalid entry %q: use an IP, CIDR, MAC, domain or 'MAC IP'", entry)
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return ProbeBanIP(), false, err
	}
	var next string
	if remove {
		next = removeBanipEntryContent(string(data), entry)
	} else {
		next = addBanipEntryContent(string(data), entry)
	}
	if err := writeBanipList(path, next); err != nil {
		restoreBanipList(path)
		return ProbeBanIP(), true, err
	}
	if err := executor.Run(executor.Op{Kind: "initd", Args: []string{"banip", "reload"}}); err != nil {
		restoreBanipList(path)
		return ProbeBanIP(), true, err
	}
	probe := ProbeBanIP()
	entries := probe.Blocklist
	if list == "allowlist" {
		entries = probe.Allowlist
	}
	found := false
	for _, e := range entries {
		if e == entry {
			found = true
			break
		}
	}
	if found == remove {
		restoreBanipList(path)
		return probe, true, fmt.Errorf("healthcheck failed after list change, rolled back")
	}
	return probe, false, nil
}

// BanipSearchResult is the parsed output of `/etc/init.d/banip search <ip>`.
type BanipSearchResult struct {
	IP    string   `json:"ip"`
	Found bool     `json:"found"`
	Sets  []string `json:"sets"`
}

var reBanipSearchHit = regexp.MustCompile(`IP found in Set '([^']+)'`)

// SearchBanipIP looks up one IP in the banIP sets. The IP is validated
// before it reaches the command line.
func SearchBanipIP(ip string) (*BanipSearchResult, error) {
	res := &BanipSearchResult{IP: ip, Sets: []string{}}
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return res, fmt.Errorf("invalid IP address")
	}
	if !banipInstalled() {
		return res, fmt.Errorf("banip is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/etc/init.d/banip", "search", parsed.String())
	out, err := cmd.Output()
	if err != nil {
		return res, fmt.Errorf("banip search failed: %w", err)
	}
	for _, m := range reBanipSearchHit.FindAllStringSubmatch(string(out), -1) {
		res.Sets = append(res.Sets, m[1])
	}
	res.Found = len(res.Sets) > 0
	return res, nil
}

// InstallBanip installs the banIP package after explicit confirmation from
// the UI (disk/RAM impact is stated there). Never called implicitly.
func InstallBanip(confirm bool) (*BanipProbe, error) {
	if !confirm {
		return ProbeBanIP(), fmt.Errorf("installation needs explicit confirmation")
	}
	if banipInstalled() {
		return ProbeBanIP(), nil
	}
	if err := executor.Run(executor.Op{Kind: "pkg_add", Args: []string{"banip"}}); err != nil {
		return ProbeBanIP(), fmt.Errorf("install banip: %w", err)
	}
	return ProbeBanIP(), nil
}
