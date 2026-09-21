package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// reportFixture is the real `/etc/init.d/banip report` output from the
// banIP 1.5.x README (trimmed: two feed sets with wrapped element lists,
// the header counters and the footer).
const reportFixture = `:::
::: banIP Set Statistics
:::
    Timestamp: 2025-06-08 23:24:54
    ------------------------------
    blocked syn-flood packets  : 42
    blocked udp-flood packets  : 7
    blocked icmp-flood packets : 0
    blocked invalid ct packets : 133
    blocked invalid tcp packets: 5
    ---
    auto-added IPs to allowlist: 2
    auto-added IPs to blocklist: 13

    Set                  | Count        | Inbound (packets)     | Outbound (packets)    | Port/Protocol         | Elements (max. 50)
    ---------------------+--------------+-----------------------+-----------------------+-----------------------+------------------------
    allowlist.v4         | 1            | ON: 0                 | ON: 0                 | -                     |
    allowlist.v4MAC      | 1            | -                     | ON: 177               | -                     | 65:34:31:1f:a5:b1
    blocklist.v4         | 2            | ON: 3                 | ON: 0                 | -                     |
    cinsscore.v4         | 11498        | ON: 444               | -                     | -                     | 3.92.139.143, 5.39.61.11
                         |              |                       |                       |                       | 8, 8.137.54.171
    country.v4           | 36432        | ON: 221               | -                     | -                     | 15.236.0.0, 24.56.0.0
    country.v6           | 23665        | ON: 0                 | -                     | -                     |
    doh.v4               | 1727         | -                     | ON: 2233              | tcp, udp: 53, 80, 443 | 8.8.8.8
    doh.v6               | 1217         | -                     | ON: 0                 | tcp, udp: 53, 80, 443 |
    ---------------------+--------------+-----------------------+-----------------------+-----------------------+------------------------
    8                    | 74546        | 5 (668)               | 4 (2410)              | 2                     | 5
`

func TestParseBanipReport(t *testing.T) {
	r := parseBanipReportText(reportFixture)
	if !r.Parsed {
		t.Fatalf("report not parsed")
	}
	if r.Timestamp != "2025-06-08 23:24:54" {
		t.Errorf("timestamp = %q", r.Timestamp)
	}
	if r.Dos.SynPackets != 42 || r.Dos.UdpPackets != 7 || r.Dos.IcmpPackets != 0 {
		t.Errorf("dos counters = %+v", r.Dos)
	}
	if r.Dos.InvalidCtPackets != 133 || r.Dos.InvalidTcpPackets != 5 {
		t.Errorf("invalid counters = %+v", r.Dos)
	}
	if r.AutoAllow != 2 || r.AutoBlock != 13 {
		t.Errorf("auto allow/block = %d/%d", r.AutoAllow, r.AutoBlock)
	}
	if len(r.Sets) != 8 {
		t.Fatalf("sets = %d, want 8", len(r.Sets))
	}
	var cinsscore *BanipSetStat
	for i := range r.Sets {
		if r.Sets[i].Name == "cinsscore.v4" {
			cinsscore = &r.Sets[i]
		}
	}
	if cinsscore == nil {
		t.Fatalf("cinsscore.v4 missing")
	}
	if cinsscore.Elements != 11498 {
		t.Errorf("cinsscore elements = %d", cinsscore.Elements)
	}
	if cinsscore.PacketsIn != 444 || cinsscore.PacketsOut != 0 {
		t.Errorf("cinsscore packets = %d/%d", cinsscore.PacketsIn, cinsscore.PacketsOut)
	}
	// Totals exclude allowlist sets but include blocklist + feed sets.
	// 2 (blocklist.v4) + 11498 + 36432 + 23665 + 1727 + 1217 = 74541.
	if r.TotalIPs != 74541 {
		t.Errorf("total IPs = %d, want 74541", r.TotalIPs)
	}
	if r.PacketsIn != 444+3+221 {
		t.Errorf("packets in = %d", r.PacketsIn)
	}
	// Allowlist sets are excluded from the blocked-packet sums.
	if r.PacketsOut != 2233 {
		t.Errorf("packets out = %d", r.PacketsOut)
	}
	// allowlist sets must be flagged local.
	var allow *BanipSetStat
	for i := range r.Sets {
		if r.Sets[i].Name == "allowlist.v4" {
			allow = &r.Sets[i]
		}
	}
	if allow == nil || !allow.LocalAllow || allow.LocalBlock {
		t.Errorf("allowlist flags wrong: %+v", allow)
	}
}

func TestParseBanipStatus(t *testing.T) {
	out := `::: banIP runtime information
  + status            : active (nft: ✔, monitor: ✔)
  + version           : 1.5.6-r7
  + element_count     : 128 751 (chains: 7, sets: 19, rules: 47)
  + nft_info          : ver: 1.1.1-r1, priority: -100, policy: performance, loglevel: warn, expiry: 2h, limit (icmp/syn/udp): 25/10/100
  + run_info          : base: /tmp/banIP
  + run_flags         : auto: ✔, proto (4/6): ✔/✔, count: ✔, dedup: ✔
  + last_run          : mode: restart, 2025-06-08 21:11:21, duration: 0m 22s, memory: 1310.16 MB available
  + system_info       : cores: 4
`
	version, elements, mem, lastRun, il, sl, ul := parseBanipStatusText(out)
	if version != "1.5.6-r7" {
		t.Errorf("version = %q", version)
	}
	if elements != 128751 {
		t.Errorf("elements = %d", elements)
	}
	if mem != 1310 {
		t.Errorf("mem = %d", mem)
	}
	if lastRun != "2025-06-08 21:11:21" {
		t.Errorf("lastRun = %q", lastRun)
	}
	if il != 25 || sl != 10 || ul != 100 {
		t.Errorf("limits = %d/%d/%d", il, sl, ul)
	}
}

func TestBanipFeeds(t *testing.T) {
	opts := map[string][]string{
		"ban_enabled":   {"1"},
		"ban_feed":      {"cinsscore", "doh", "country"},
		"ban_feedout":   {"doh"},
		"ban_feedinout": {"country"},
		"ban_feedin":    {"turris"},
		"ban_nftcount":  {"1"},
	}
	feeds := banipFeeds(opts)
	byName := map[string]BanipFeed{}
	for _, f := range feeds {
		byName[f.Name] = f
	}
	if len(feeds) != 4 {
		t.Fatalf("feeds = %d, want 4: %+v", len(feeds), feeds)
	}
	if !byName["cinsscore"].Enabled || byName["cinsscore"].Direction != "" {
		t.Errorf("cinsscore = %+v", byName["cinsscore"])
	}
	if !byName["doh"].Enabled || byName["doh"].Direction != "out" {
		t.Errorf("doh = %+v", byName["doh"])
	}
	if !byName["country"].Enabled || byName["country"].Direction != "inout" {
		t.Errorf("country = %+v", byName["country"])
	}
	if byName["turris"].Enabled || byName["turris"].Direction != "in" {
		t.Errorf("turris = %+v", byName["turris"])
	}
}

func TestBanipFeedOps(t *testing.T) {
	enabled, nftcount := true, true
	cfg := BanipFeedsConfig{
		Feeds: []BanipFeed{
			{Name: "cinsscore", Enabled: true},
			{Name: "doh", Enabled: true, Direction: "out"},
			{Name: "turris", Enabled: false, Direction: "in"},
		},
		Enabled:  &enabled,
		NftCount: &nftcount,
	}
	ops, err := banipFeedOps(cfg)
	if err != nil {
		t.Fatalf("ops error: %v", err)
	}
	var addList []string
	for _, op := range ops {
		if op.Kind != "uci_add_list" {
			continue
		}
		addList = append(addList, op.Args[0]+"="+op.Args[1])
	}
	want := []string{
		"banip.global.ban_feed=cinsscore",
		"banip.global.ban_feed=doh",
		"banip.global.ban_feedout=doh",
		"banip.global.ban_feedin=turris",
	}
	got := strings.Join(addList, ",")
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing op %q in %q", w, got)
		}
	}
	if strings.Contains(got, "ban_feed=turris") {
		t.Errorf("disabled feed ended up in ban_feed: %q", got)
	}
	// The last two ops must commit and reload.
	last := ops[len(ops)-2:]
	if last[0].Kind != "uci_commit" || last[1].Kind != "initd" || last[1].Args[1] != "reload" {
		t.Errorf("tail ops = %+v", last)
	}
}

func TestBanipFeedOpsRejectsGarbage(t *testing.T) {
	if _, err := banipFeedOps(BanipFeedsConfig{Feeds: []BanipFeed{{Name: "bad;name", Enabled: true}}}); err == nil {
		t.Errorf("invalid feed name accepted")
	}
	if _, err := banipFeedOps(BanipFeedsConfig{Feeds: []BanipFeed{{Name: "ok", Direction: "sideways"}}}); err == nil {
		t.Errorf("invalid direction accepted")
	}
}

func TestBanipFeedsMatch(t *testing.T) {
	tru := true
	cfg := BanipFeedsConfig{
		Feeds:   []BanipFeed{{Name: "doh", Enabled: true, Direction: "out"}},
		Enabled: &tru,
	}
	live := map[string][]string{
		"ban_enabled": {"1"},
		"ban_feed":    {"doh"},
		"ban_feedout": {"doh"},
	}
	if !banipFeedsMatch(live, cfg) {
		t.Errorf("identical config did not match")
	}
	live["ban_feed"] = []string{"doh", "extra"}
	if banipFeedsMatch(live, cfg) {
		t.Errorf("extra live feed matched")
	}
	delete(live, "ban_feedout")
	if banipFeedsMatch(live, cfg) {
		t.Errorf("missing direction override matched")
	}
}

func TestValidBanipEntry(t *testing.T) {
	valid := []string{
		"192.168.1.10", "2001:db8::1", "10.0.0.0/8", "2001:db8::/32",
		"66:34:31:1f:a5:b1", "example.com", "sub.dominio.es",
		"66:34:31:1f:a5:b1 192.168.1.10",
	}
	for _, e := range valid {
		if !validBanipEntry(e) {
			t.Errorf("rejected valid entry %q", e)
		}
	}
	invalid := []string{
		"", "hello world", "1.2.3.4/33", "10.0.0.0/8; rm -rf /", "-", "#comment",
		"192.168.1.1 192.168.1.2", "not_a_domain", "a b c",
	}
	for _, e := range invalid {
		if validBanipEntry(e) {
			t.Errorf("accepted invalid entry %q", e)
		}
	}
}

func TestBanipListContentPlanners(t *testing.T) {
	base := "# comment\n192.168.1.5\nexample.com\n"
	if got := addBanipEntryContent(base, "example.com"); got != base {
		t.Errorf("dedupe changed content: %q", got)
	}
	got := addBanipEntryContent(base, "10.0.0.7")
	if !strings.HasSuffix(got, "10.0.0.7\n") || !strings.HasPrefix(got, "# comment") {
		t.Errorf("add result = %q", got)
	}
	if got := addBanipEntryContent("", "10.0.0.7"); got != "10.0.0.7\n" {
		t.Errorf("add to empty = %q", got)
	}
	rem := removeBanipEntryContent(base, "192.168.1.5")
	if strings.Contains(rem, "192.168.1.5") || !strings.Contains(rem, "example.com") {
		t.Errorf("remove result = %q", rem)
	}
	if got := removeBanipEntryContent("10.0.0.7\n", "10.0.0.7"); got != "" {
		t.Errorf("remove last = %q", got)
	}
}

const catalogFixture = `{
	"cinsscore": {
		"url_4": "https://cinsscore.com/list/ci-badguys.txt",
		"url_6": "https://cinsscore.com/list/ci-badguys-v6.txt",
		"rule_4": "/^127\\./{next}/ {printf \"%s\\n\",$1}",
		"chain": "in",
		"descr": "CINS Score malicious IPs"
	},
	"doh": {
		"url_4": "https://raw.githubusercontent.com/dibdot/DoH-IP-blocklists/master/doh-ipv4.txt",
		"url_6": "https://raw.githubusercontent.com/dibdot/DoH-IP-blocklists/master/doh-ipv6.txt",
		"rule_4": "{printf \"%s\\n\",$1}",
		"chain": "out",
		"descr": "Public DoH-Provider"
	},
	"nocat": {
		"url_4": "https://example.com/list.txt",
		"descr": "feed without an explicit chain"
	},
	"broken": {
		"descr": "no url_4, must be skipped"
	}
}`

func TestParseBanipCatalog(t *testing.T) {
	feeds := parseBanipCatalog([]byte(catalogFixture), false, banipCatalogCap)
	if len(feeds) != 3 {
		t.Fatalf("feeds = %d, want 3 (broken skipped): %+v", len(feeds), feeds)
	}
	byName := map[string]BanipCatalogFeed{}
	for _, f := range feeds {
		byName[f.Name] = f
		if f.Descr == "" {
			t.Errorf("feed %s lost its description", f.Name)
		}
	}
	c := byName["cinsscore"]
	if c.Chain != "in" || !c.IPv6 || c.Custom {
		t.Errorf("cinsscore = %+v", c)
	}
	d := byName["doh"]
	if d.Chain != "out" || !d.IPv6 {
		t.Errorf("doh = %+v", d)
	}
	n := byName["nocat"]
	if n.Chain != "" || n.IPv6 {
		t.Errorf("nocat = %+v", n)
	}
	// custom flag propagation
	custom := parseBanipCatalog([]byte(catalogFixture), true, banipCatalogCap)
	if len(custom) == 0 || !custom[0].Custom {
		t.Errorf("custom flag not set: %+v", custom)
	}
	// empty and broken input are empty states, not errors
	for _, bad := range []string{"", "not json", "[]", "{}", "null"} {
		if got := parseBanipCatalog([]byte(bad), false, banipCatalogCap); len(got) != 0 {
			t.Errorf("input %q yielded %d feeds", bad, len(got))
		}
	}
	// cap is honored
	if got := parseBanipCatalog([]byte(catalogFixture), false, 2); len(got) != 2 {
		t.Errorf("cap not honored: %d", len(got))
	}
}

func TestBanipMergeCatalog(t *testing.T) {
	configured := []BanipFeed{
		{Name: "cinsscore", Enabled: true},
		{Name: "manual", Enabled: true}, // not in catalog (hand-added in UCI)
	}
	catalog := []BanipCatalogFeed{
		{Name: "cinsscore", Descr: "CINS", Chain: "in", IPv6: true},
		{Name: "doh", Descr: "DoH", Chain: "out"},
	}
	feeds, available := banipMergeCatalog(configured, catalog)
	if len(available) != 1 || available[0].Name != "doh" {
		t.Errorf("available = %+v", available)
	}
	if !feeds[0].InCatalog {
		t.Errorf("configured catalog feed not marked: %+v", feeds[0])
	}
	if feeds[0].Chain != "in" || !feeds[0].IPv6 {
		t.Errorf("catalog defaults not attached to configured feed: %+v", feeds[0])
	}
	if feeds[1].InCatalog {
		t.Errorf("manual feed wrongly marked: %+v", feeds[1])
	}
	// no duplicates: cinsscore must not appear in available
	for _, a := range available {
		if a.Name == "cinsscore" {
			t.Errorf("duplicate of configured feed in catalog list")
		}
	}
}

func TestParseBanipDownloadFailures(t *testing.T) {
	log := `Sep 20 10:14:58 rt-lab user.info banIP-1.5.6-r7[28527]: download for feed 'debl.v4' failed, rc: 4
Sep 20 10:15:01 rt-lab user.info banIP-1.5.6-r7[28527]: download for feed 'turris.v6' failed, rc: 4
Sep 20 10:15:02 rt-lab user.info banIP-1.5.6-r7[28527]: download for feed 'doh.v4' failed, rc: 4
Sep 20 10:15:03 rt-lab user.info banIP-1.5.6-r7[28527]: start banIP processing
Sep 20 10:15:04 rt-lab user.info banIP-1.5.6-r7[28527]: download for feed 'hagezi_v6.v4MAC' failed, rc: 4
`
	failed := parseBanipDownloadFailures(log)
	for _, name := range []string{"debl", "turris", "doh", "hagezi_v6"} {
		if !failed[name] {
			t.Errorf("expected %q flagged as failed, got %v", name, failed)
		}
	}
	for name := range failed {
		switch name {
		case "debl", "turris", "doh", "hagezi_v6":
		default:
			t.Errorf("unexpected failed feed %q", name)
		}
	}
	if len(parseBanipDownloadFailures("")) != 0 {
		t.Errorf("empty log must yield no failures")
	}
}

func TestBanipMarkDownloadFailures(t *testing.T) {
	report := &BanipReport{
		Parsed: true,
		Sets: []BanipSetStat{
			{Name: "debl.v4", Elements: 0},
			{Name: "turris.v4", Elements: 4553},
			{Name: "doh.v6MAC", Elements: 0},
		},
	}
	feeds := []BanipFeed{
		{Name: "debl", Enabled: true},      // failed + empty set -> marked
		{Name: "turris", Enabled: true},    // failed but recovered -> not marked
		{Name: "doh", Enabled: true},       // failed, MAC-only set empty -> marked
		{Name: "cinsscore", Enabled: true}, // no failure -> not marked
	}
	banipMarkDownloadFailures(feeds, map[string]bool{"debl": true, "turris": true, "doh": true}, report)
	if !feeds[0].LastDownloadFailed {
		t.Errorf("debl should be marked: %+v", feeds[0])
	}
	if feeds[1].LastDownloadFailed {
		t.Errorf("turris recovered (set has elements), must not be marked: %+v", feeds[1])
	}
	if !feeds[2].LastDownloadFailed {
		t.Errorf("doh should be marked (MAC set empty): %+v", feeds[2])
	}
	if feeds[3].LastDownloadFailed {
		t.Errorf("cinsscore has no failure line, must not be marked: %+v", feeds[3])
	}
	// No failures, no report, or unparsed report: never mark.
	other := []BanipFeed{{Name: "debl", Enabled: true}}
	banipMarkDownloadFailures(other, nil, report)
	if other[0].LastDownloadFailed {
		t.Errorf("no failures must never mark")
	}
	banipMarkDownloadFailures(other, map[string]bool{"debl": true}, nil)
	if other[0].LastDownloadFailed {
		t.Errorf("missing report must never mark")
	}
	banipMarkDownloadFailures(other, map[string]bool{"debl": true}, &BanipReport{Parsed: false})
	if other[0].LastDownloadFailed {
		t.Errorf("unparsed report must never mark")
	}
}

func TestBanipCacheInvalidate(t *testing.T) {
	banipInvalidate()
	// Seed a fake fresh probe: the cached read must come back without forks.
	banipCache.Lock()
	banipCache.probe = &BanipProbe{Installed: true, Version: "test"}
	banipCache.at = time.Now()
	banipCache.Unlock()
	if got := ProbeBanIPCached(); !got.Installed || got.Version != "test" {
		t.Errorf("cached probe not returned: %+v", got)
	}
	banipInvalidate()
	banipCache.Lock()
	p := banipCache.probe
	banipCache.Unlock()
	if p != nil {
		t.Errorf("invalidate kept the probe")
	}
}

func TestBanipWarmupPopulatesCaches(t *testing.T) {
	banipInvalidate()
	defer banipInvalidate()
	old := banipWarmupDelay
	banipWarmupDelay = 0
	defer func() { banipWarmupDelay = old }()
	StartBanipWarmup()
	deadline := time.Now().Add(5 * time.Second)
	for {
		banipCache.Lock()
		hasStatus, hasProbe := banipCache.status != nil, banipCache.probe != nil
		banipCache.Unlock()
		if hasStatus && hasProbe {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("warmup did not populate the caches (status=%v probe=%v)", hasStatus, hasProbe)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunningFromPidfile(t *testing.T) {
	dir := t.TempDir()
	// Fake /proc so the test is hermetic: the pid dir existing means the
	// process is alive, comm carries the name check.
	proc := filepath.Join(dir, "proc")
	if err := os.MkdirAll(proc, 0o755); err != nil {
		t.Fatal(err)
	}
	pidDir := func(pid string, comm string) {
		d := filepath.Join(proc, pid)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "comm"), []byte(comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writePidfile := func(name, pid string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(pid+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Missing pidfile -> false (caller falls back to the init.d fork).
	if runningFromPidfile(filepath.Join(dir, "nope.pid"), proc) {
		t.Errorf("missing pidfile reported running")
	}
	// Malformed pidfile -> false.
	if runningFromPidfile(writePidfile("bad.pid", "garbage"), proc) {
		t.Errorf("garbage pidfile reported running")
	}
	// Dead pid -> false: no directory under the fake proc.
	if runningFromPidfile(writePidfile("dead.pid", "424242"), proc) {
		t.Errorf("dead pid reported running")
	}
	// Live banIP service pid -> true.
	pidDir("100", "banip-service.s")
	if !runningFromPidfile(writePidfile("live.pid", "100"), proc) {
		t.Errorf("live banip pid not reported running")
	}
	// Live pid with a foreign comm -> false (pid reuse guard).
	pidDir("200", "systemd")
	if runningFromPidfile(writePidfile("foreign.pid", "200"), proc) {
		t.Errorf("foreign live pid reported running")
	}
	// Live pid without a readable comm -> true (best effort, no comm).
	if err := os.MkdirAll(filepath.Join(proc, "300"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !runningFromPidfile(writePidfile("nocomm.pid", "300"), proc) {
		t.Errorf("live pid without comm not reported running")
	}
}

func TestBanipRamWarning(t *testing.T) {
	none := map[string]bool{}
	dismissed := map[string]bool{"2025-06-08 21:11:21": true}

	// Low free RAM with a known run -> warning, visible.
	w := banipRamWarning(130, "2025-06-08 21:11:21", none)
	if w == nil || w.FreeMB != 130 || w.LastRun != "2025-06-08 21:11:21" || w.Dismissed {
		t.Fatalf("warning = %+v", w)
	}
	// Same run dismissed -> still reported, flagged dismissed (the UI hides
	// it but the state stays honest for a later new run).
	w = banipRamWarning(130, "2025-06-08 21:11:21", dismissed)
	if w == nil || !w.Dismissed {
		t.Fatalf("dismissed warning = %+v", w)
	}
	// A new run (new timestamp) shows again even after a dismiss.
	w = banipRamWarning(130, "2025-06-09 08:00:00", dismissed)
	if w == nil || w.Dismissed {
		t.Fatalf("new-run warning = %+v", w)
	}
	// Enough RAM, unknown RAM and missing timestamp -> no warning at all.
	if w := banipRamWarning(512, "2025-06-08 21:11:21", none); w != nil {
		t.Errorf("warning with enough RAM = %+v", w)
	}
	if w := banipRamWarning(0, "2025-06-08 21:11:21", none); w != nil {
		t.Errorf("warning with unknown RAM = %+v", w)
	}
	if w := banipRamWarning(130, "", none); w != nil {
		t.Errorf("warning without run timestamp = %+v", w)
	}
}

func TestBanipDismissedFileRoundTrip(t *testing.T) {
	old := banipDismissedPath
	banipDismissedPath = filepath.Join(t.TempDir(), "banip_dismissed.json")
	defer func() { banipDismissedPath = old }()

	if got := loadBanipDismissed(); len(got) != 0 {
		t.Fatalf("missing file = %v", got)
	}
	if err := saveBanipDismissed(map[string]bool{"b-run": true, "a-run": true}); err != nil {
		t.Fatal(err)
	}
	got := loadBanipDismissed()
	if !got["a-run"] || !got["b-run"] || len(got) != 2 {
		t.Fatalf("round trip = %v", got)
	}
	// Malformed file reads as empty (fail-soft).
	if err := os.WriteFile(banipDismissedPath, []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadBanipDismissed(); len(got) != 0 {
		t.Fatalf("malformed file = %v", got)
	}
	// Empty list removes the file, like saveQuotaConfig.
	if err := saveBanipDismissed(map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(banipDismissedPath); !os.IsNotExist(err) {
		t.Fatalf("file not removed: %v", err)
	}
}
