package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	banipAllowlistPath   = "/etc/banip/banip.allowlist"
	banipBlocklistPath   = "/etc/banip/banip.blocklist"
	banipFeedsPath       = "/etc/banip/banip.feeds"
	banipCustomFeedsPath = "/etc/banip/banip.custom.feeds"
	// banipCatalogCap bounds the catalog in case the feeds file is huge.
	banipCatalogCap = 200
)

// BanipFeed is one configured blocklist feed. Direction is "in", "out" or
// "inout" when overridden via ban_feedin/ban_feedout/ban_feedinout; "" means
// the feed default from the banIP feed table applies.
type BanipFeed struct {
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Direction string `json:"direction"`
	// InCatalog is true when the feed exists in the local catalog
	// (/etc/banip/banip.feeds or banip.custom.feeds).
	InCatalog bool `json:"in_catalog"`
	// Chain and IPv6 are the catalog defaults, attached to configured
	// feeds too so the UI can explain them without a second lookup.
	Chain string `json:"chain,omitempty"`
	IPv6  bool   `json:"ipv6,omitempty"`
	// LastDownloadFailed is true when the most recent download attempt of
	// this feed failed and its sets are still empty in the report. banIP
	// only logs failures (a successful run leaves no per-feed trace), so
	// the marker combines the syslog failure with the empty report set to
	// avoid accusing a feed that already recovered on a later run.
	LastDownloadFailed bool `json:"last_download_failed,omitempty"`
}

// BanipCatalogFeed is one feed available in the local catalog. The url_*
// and rule_* fields are never exposed: they are download internals of
// banIP, not something the panel edits.
type BanipCatalogFeed struct {
	Name   string `json:"name"`
	Descr  string `json:"descr"`
	Chain  string `json:"chain"` // default direction: "in", "out" or "inout"; "" = unspecified
	IPv6   bool   `json:"ipv6"`  // has a url_6 source
	Custom bool   `json:"custom"`
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
	Installed      bool               `json:"installed"`
	Enabled        bool               `json:"enabled"` // ban_enabled
	Running        bool               `json:"running"`
	NftCount       bool               `json:"nft_count"`
	Applicable     bool               `json:"applicable"` // gateway with an uplink
	Version        string             `json:"version"`
	MemAvailableMB int64              `json:"mem_available_mb"` // from last_run; 0 = unknown
	Feeds          []BanipFeed        `json:"feeds"`
	Catalog        []BanipCatalogFeed `json:"catalog"` // available feeds not configured in UCI
	Report         *BanipReport       `json:"report,omitempty"`
	Allowlist      []string           `json:"allowlist"`
	Blocklist      []string           `json:"blocklist"`
}

// banipCache is the in-memory cache for GET /api/banip. The full probe
// forks several commands and takes seconds on a router; the UI loads it on
// page open and refresh, so a very short TTL removes back-to-back double
// loads without ever serving stale state after a mutation: every write
// path invalidates the cache.
var banipCache = struct {
	sync.Mutex
	probe        *BanipProbe
	at           time.Time
	status       *BanipStatus
	statusAt     time.Time
	revalidating bool
	gen          uint64
}{}

const banipCacheTTL = 3 * time.Second

// banipRevalidateAfter is the age at which serving a cached probe also
// triggers a background refresh. Serving never blocks on age: the probe
// takes seconds on a router and its numbers move slowly, so a menu entry
// always gets the cached copy; the background recompute (bounded by this
// threshold) keeps it converging to live state.
const banipRevalidateAfter = 60 * time.Second

// banipStatusTTL is shorter than the probe TTL: the status feeds the
// Services card and the progressive page paint, where 2s of staleness is
// invisible but every saved fork is felt.
const banipStatusTTL = 2 * time.Second

// ProbeBanIPCached returns the cached probe. Fresh entries (3s) are served
// as is; older entries are ALSO served instantly - the probe takes seconds
// on a router and its numbers move slowly, so blocking a menu entry on it is
// worse than showing slightly stale data. Any entry older than
// banipRevalidateAfter triggers a background refresh (singleflight), so data
// converges while the UI stays instant. Only an EMPTY cache (first read
// after start or after a mutation invalidated it) computes synchronously.
// Mutations invalidate via banipInvalidate.
func ProbeBanIPCached() *BanipProbe {
	banipCache.Lock()
	p, at, gen := banipCache.probe, banipCache.at, banipCache.gen
	age := time.Since(at)
	switch {
	case p != nil && age < banipCacheTTL:
		banipCache.Unlock()
		return p
	case p != nil:
		if age >= banipRevalidateAfter && !banipCache.revalidating {
			banipCache.revalidating = true
			go banipRevalidate(gen)
		}
		banipCache.Unlock()
		return p
	default:
		banipCache.Unlock()
		np := ProbeBanIP()
		banipCache.Lock()
		if gen == banipCache.gen {
			banipCache.probe, banipCache.at = np, time.Now()
		}
		banipCache.Unlock()
		return np
	}
}

// banipRevalidate recomputes the probe in the background after a stale
// entry was served. A mutation bumping the generation (banipInvalidate)
// discards the result: the next read recomputes from live state.
func banipRevalidate(gen uint64) {
	np := ProbeBanIP()
	banipCache.Lock()
	defer banipCache.Unlock()
	banipCache.revalidating = false
	if gen == banipCache.gen {
		banipCache.probe, banipCache.at = np, time.Now()
	}
}

// banipInvalidate drops the cached probe and status. Write paths call this
// (via defer) so the next read recomputes from the router.
func banipInvalidate() {
	banipCache.Lock()
	banipCache.probe = nil
	banipCache.status = nil
	banipCache.revalidating = false
	banipCache.gen++
	banipCache.Unlock()
}

// ProbeBanipStatusCached is the cached variant of ProbeBanipStatus for
// GET /api/banip/status; mutations invalidate it via banipInvalidate.
func ProbeBanipStatusCached() *BanipStatus {
	banipCache.Lock()
	s, at := banipCache.status, banipCache.statusAt
	banipCache.Unlock()
	if s != nil && time.Since(at) < banipStatusTTL {
		return s
	}
	s = ProbeBanipStatus()
	banipCache.Lock()
	banipCache.status, banipCache.statusAt = s, time.Now()
	banipCache.Unlock()
	return s
}

// runningFromPidfile reports whether the pid recorded in pidfile names a
// live process under procDir whose comm mentions banip (guards against pid
// reuse). Missing or malformed pidfiles answer false so the caller falls
// back to the init.d fork. Pure on its arguments so tests run off-router.
func runningFromPidfile(pidfile, procDir string) bool {
	data, err := os.ReadFile(pidfile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	if _, err := os.Stat(filepath.Join(procDir, strconv.Itoa(pid))); err != nil {
		return false
	}
	if comm, err := os.ReadFile(filepath.Join(procDir, strconv.Itoa(pid), "comm")); err == nil {
		return strings.Contains(strings.ToLower(string(comm)), "banip")
	}
	// No comm available (off-router tests): the live pid is enough.
	return true
}

// BanipStatus is the lightweight status used by the Services overview card:
// no report, no lists, no catalog, just the cheap forks.
type BanipStatus struct {
	Installed  bool `json:"installed"`
	Enabled    bool `json:"enabled"`
	Running    bool `json:"running"`
	Applicable bool `json:"applicable"`
}

// ProbeBanipStatus reads only installed/enabled/running. Cheap on purpose:
// the Services page renders several cards at once. The running check reads
// the procd pidfile first (no fork) and only falls back to the init.d
// command when there is no pidfile.
func ProbeBanipStatus() *BanipStatus {
	s := &BanipStatus{Installed: banipInstalled(), Applicable: hasUplink()}
	if !s.Installed {
		return s
	}
	s.Enabled = uciGet("banip.global.ban_enabled") == "1"
	if runningFromPidfile("/var/run/banip.pid", "/proc") {
		s.Running = true
	} else {
		s.Running = executor.ServiceRunning("banip")
	}
	return s
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

// parseBanipCatalog parses a /etc/banip/banip.feeds document (flat JSON
// object keyed by feed name). Only the display fields are kept; url_* and
// rule_* are download internals and are dropped. A feed without url_4 is
// not usable and is skipped. Broken or empty input yields an empty list
// (the catalog is a nice-to-have, never an error).
func parseBanipCatalog(data []byte, custom bool, max int) []BanipCatalogFeed {
	var raw map[string]struct {
		URL4  string `json:"url_4"`
		URL6  string `json:"url_6"`
		Chain string `json:"chain"`
		Descr string `json:"descr"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		return []BanipCatalogFeed{}
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	out := []BanipCatalogFeed{}
	for _, name := range names {
		if len(out) >= max {
			break
		}
		f := raw[name]
		if f.URL4 == "" {
			continue
		}
		chain := f.Chain
		if chain != "in" && chain != "out" && chain != "inout" {
			chain = ""
		}
		out = append(out, BanipCatalogFeed{
			Name:   name,
			Descr:  f.Descr,
			Chain:  chain,
			IPv6:   f.URL6 != "",
			Custom: custom,
		})
	}
	return out
}

// banipCatalog reads the stock and custom feed catalogs. Custom feeds win
// on name collisions (they are the user's own definitions).
func banipCatalog() []BanipCatalogFeed {
	out := []BanipCatalogFeed{}
	seen := map[string]bool{}
	for _, src := range []struct {
		path   string
		custom bool
	}{
		{banipFeedsPath, false},
		{banipCustomFeedsPath, true},
	} {
		data, err := os.ReadFile(src.path)
		if err != nil {
			continue
		}
		for _, f := range parseBanipCatalog(data, src.custom, banipCatalogCap-len(out)) {
			if seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			out = append(out, f)
		}
	}
	return out
}

// parseBanipDownloadFailures returns the set of feed names (without the
// .v4/.v6 suffix) whose download failed, according to recent syslog lines.
// banIP 1.5.x logs a line per failed download and nothing on success, so
// the failure set alone cannot tell a broken feed from one that recovered
// on a later run; the caller cross-checks with the report.
func parseBanipDownloadFailures(out string) map[string]bool {
	failed := map[string]bool{}
	for _, m := range reBanipDownloadFail.FindAllStringSubmatch(out, -1) {
		failed[m[1]] = true
	}
	return failed
}

// banipDownloadLog reads the recent syslog lines mentioning banIP. ubox
// logread's -e regex is case-sensitive against the message text, so the
// filter must be the exact tag "banIP" (a lowercase "banip" matches nothing
// and would silently suppress every failure marker). It falls back to the
// full `logread` output; the parser re-greps either way. Missing logread or
// an empty log is an empty result, never an error: the marker is a hint,
// not a verdict.
func banipDownloadLog() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "logread", "-e", "banIP").Output(); err == nil && len(out) > 0 {
		return string(out)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	out, err := exec.CommandContext(ctx2, "logread").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// banipMarkDownloadFailures flags configured feeds whose download failed
// and whose sets are still empty in the report. A feed with elements in any
// of its sets already recovered (banIP only logs failures, never successes,
// so an old failure line may outlive the fix); without a parsed report the
// signal is not actionable and nothing is marked.
func banipMarkDownloadFailures(feeds []BanipFeed, failed map[string]bool, report *BanipReport) {
	if len(failed) == 0 || report == nil || !report.Parsed {
		return
	}
	elems := map[string]int64{}
	for _, s := range report.Sets {
		name := s.Name
		for _, suf := range []string{".v4MAC", ".v6MAC", ".v4", ".v6"} {
			if strings.HasSuffix(name, suf) {
				name = strings.TrimSuffix(name, suf)
				break
			}
		}
		elems[name] += s.Elements
	}
	for i := range feeds {
		if failed[feeds[i].Name] && elems[feeds[i].Name] == 0 {
			feeds[i].LastDownloadFailed = true
		}
	}
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
	// reBanipDownloadFail matches the banIP syslog failure lines, e.g.
	// user.info banIP-1.5.6-r7[28527]: download for feed 'debl.v4' failed, rc: 4
	reBanipDownloadFail = regexp.MustCompile(`download for feed '([a-z0-9_-]+)\.(?:v4|v6|v4MAC|v6MAC)' failed`)
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

// banipMergeCatalog splits the catalog into configured (marks in_catalog
// on the matching feed) and available entries. Pure so tests can cover the
// no-duplicates rule.
func banipMergeCatalog(feeds []BanipFeed, catalog []BanipCatalogFeed) ([]BanipFeed, []BanipCatalogFeed) {
	configured := map[string]bool{}
	for _, f := range feeds {
		configured[f.Name] = true
	}
	available := []BanipCatalogFeed{}
	for _, cf := range catalog {
		if configured[cf.Name] {
			for i := range feeds {
				if feeds[i].Name == cf.Name {
					feeds[i].InCatalog = true
					feeds[i].Chain = cf.Chain
					feeds[i].IPv6 = cf.IPv6
				}
			}
			continue
		}
		available = append(available, cf)
	}
	return feeds, available
}

// ProbeBanIP reads the full banIP state. Every read is fail-soft: a missing
// report or status is an omitted field, never an error.
func ProbeBanIP() *BanipProbe {
	p := &BanipProbe{
		Installed:  banipInstalled(),
		Applicable: hasUplink(),
		Feeds:      []BanipFeed{},
		Catalog:    []BanipCatalogFeed{},
		Allowlist:  []string{},
		Blocklist:  []string{},
	}
	if !p.Installed {
		return p
	}
	// The independent reads fork (uci show, init.d running/status/report,
	// the two list files, logread): run them concurrently. status and report used
	// to be sequential with report gated on status; both are read-only and
	// the DoS thresholds from status are merged into the report afterwards,
	// so they can run in parallel and degrade independently (fail-soft).
	var (
		wg                   sync.WaitGroup
		opts                 map[string][]string
		running              bool
		statusOut, reportOut string
		statusOK, reportOK   bool
		allowlist, blocklist []string
		downloadLog          string
	)
	wg.Add(7)
	go func() { defer wg.Done(); opts = banipGlobal() }()
	go func() {
		defer wg.Done()
		if runningFromPidfile("/var/run/banip.pid", "/proc") {
			running = true
		} else {
			running = executor.ServiceRunning("banip")
		}
	}()
	go func() {
		defer wg.Done()
		if out, err := exec.Command("/etc/init.d/banip", "status").Output(); err == nil {
			statusOut, statusOK = string(out), true
		}
	}()
	go func() {
		defer wg.Done()
		if out, err := exec.Command("/etc/init.d/banip", "report").Output(); err == nil {
			reportOut, reportOK = string(out), true
		}
	}()
	go func() { defer wg.Done(); allowlist = banipReadList(banipAllowlistPath) }()
	go func() { defer wg.Done(); blocklist = banipReadList(banipBlocklistPath) }()
	go func() { defer wg.Done(); downloadLog = banipDownloadLog() }()
	wg.Wait()

	p.Enabled = len(opts["ban_enabled"]) > 0 && opts["ban_enabled"][0] == "1"
	p.NftCount = len(opts["ban_nftcount"]) > 0 && opts["ban_nftcount"][0] == "1"
	p.Feeds = banipFeeds(opts)
	p.Running = running

	// Catalog: available feeds that are not configured in UCI. Configured
	// feeds get in_catalog so the UI can show which ones come from the
	// stock/custom catalogs.
	p.Feeds, p.Catalog = banipMergeCatalog(p.Feeds, banipCatalog())

	if statusOK {
		v, _, mem, il, sl, ul := parseBanipStatusText(statusOut)
		p.Version = v
		p.MemAvailableMB = mem
		if reportOK {
			rep := parseBanipReportText(reportOut)
			rep.Dos.IcmpLimit = il
			rep.Dos.SynLimit = sl
			rep.Dos.UdpLimit = ul
			p.Report = rep
		}
	} else if reportOK {
		// status failed but report answered: show it without thresholds.
		p.Report = parseBanipReportText(reportOut)
	}

	p.Allowlist = allowlist
	p.Blocklist = blocklist
	banipMarkDownloadFailures(p.Feeds, parseBanipDownloadFailures(downloadLog), p.Report)
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
	defer banipInvalidate()
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
		// banIP's start_service refuses to run without the rc.d autostart
		// symlink (`/etc/init.d/banip enabled` check): toggling only the UCI
		// master switch leaves the service unable to start, so enable both
		// and start in one batch.
		ops = []executor.Op{
			{Kind: "uci_set", Args: []string{"banip.global.ban_enabled", "1"}},
			{Kind: "uci_commit", Args: []string{"banip"}},
			{Kind: "initd", Args: []string{"banip", "enable"}},
			{Kind: "initd", Args: []string{"banip", "start"}},
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
			{Kind: "initd", Args: []string{"banip", "disable"}},
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
// healthcheck (live UCI must match) and rollback. Feed names are validated
// against the local catalog plus whatever is already configured in UCI, so
// typos are rejected before any write.
func SetBanipFeeds(cfg BanipFeedsConfig) (*BanipProbe, bool, error) {
	defer banipInvalidate()
	if !banipInstalled() {
		return ProbeBanIP(), false, fmt.Errorf("banip is not installed")
	}
	known := map[string]bool{}
	for _, cf := range banipCatalog() {
		known[cf.Name] = true
	}
	for _, f := range banipFeeds(banipGlobal()) {
		known[f.Name] = true
	}
	for _, f := range cfg.Feeds {
		if !known[f.Name] {
			return ProbeBanIP(), false, fmt.Errorf("unknown feed %q: it is not in the banIP catalog or the current config", f.Name)
		}
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
	defer banipInvalidate()
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
	defer banipInvalidate()
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

// UninstallBanip removes the banIP packages after explicit confirmation.
// The service is stopped and its rc.d autostart removed in the same batch
// before the packages, so no half-removed state is left behind if a step
// fails. The UCI config (/etc/config/banip) and the local lists stay on
// disk, as opkg keeps conffiles: the UI says so in the confirmation.
func UninstallBanip(confirm bool) (*BanipProbe, error) {
	defer banipInvalidate()
	if !confirm {
		return ProbeBanIP(), fmt.Errorf("uninstall needs explicit confirmation")
	}
	if !banipInstalled() {
		return ProbeBanIP(), nil
	}
	var ops []executor.Op
	if executor.ServiceRunning("banip") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"banip", "stop"}})
	}
	if executor.ServiceEnabled("banip") {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"banip", "disable"}})
	}
	ops = append(ops, executor.Op{Kind: "pkg_del", Args: []string{"banip"}})
	if pkgInstalled("luci-app-banip") {
		ops = append(ops, executor.Op{Kind: "pkg_del", Args: []string{"luci-app-banip"}})
	}
	if err := executor.Apply(ops, nil); err != nil {
		return ProbeBanIP(), fmt.Errorf("uninstall banip: %w", err)
	}
	return ProbeBanIP(), nil
}
