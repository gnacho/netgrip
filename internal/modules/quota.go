package modules

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Per-client data quotas (#308): a daily or monthly traffic cap per MAC with
// a notify or throttle action on exceed. User intent lives in
// /etc/netgrip/quotas.json; the persisted counters/period markers live in
// /etc/netgrip/quota_state.json. Accounting reads nftables counters from a
// NetGrip-owned table (inet netgrip_quota) keyed by client IP, mirroring how
// nftqos.go builds and applies its own table.

var (
	quotaConfigFile = "/etc/netgrip/quotas.json"
	quotaStateFile  = "/etc/netgrip/quota_state.json"
	quotaRulesFile  = "/etc/netgrip/quota_accounting.nft"
	quotaTable      = "netgrip_quota"
)

// Quota is one per-device data cap. Limit is in bytes; ThrottleKbps is the
// hard cap applied when the throttle action fires.
type Quota struct {
	MAC          string `json:"mac"`
	IP           string `json:"ip"`
	Period       string `json:"period"`        // daily | monthly
	Limit        int64  `json:"limit"`         // bytes
	Action       string `json:"action"`        // notify | throttle
	ThrottleKbps int    `json:"throttle_kbps"` // kbps (throttle action only)
}

// QuotaUsage is the read-only consumption view for one client.
type QuotaUsage struct {
	Used      int64  `json:"used"`
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
	Period    string `json:"period"`
	Throttled bool   `json:"throttled"`
	Exceeded  bool   `json:"exceeded"`
}

// QuotaProbe is the API view: config + usage, keyed by MAC.
type QuotaProbe struct {
	Applicable bool                  `json:"applicable"`
	Quotas     map[string]Quota      `json:"quotas"`
	Usage      map[string]QuotaUsage `json:"usage"`
	Ts         int64                 `json:"ts"`
}

// quotaStateEntry persists one client's accounting state. LastRx/LastTx are
// the raw nftables counter values at the last fold (upload/download); a value
// of -1 means "no baseline yet". Used accumulates only the current period.
type quotaStateEntry struct {
	Period    string `json:"period"`
	PeriodKey string `json:"period_key"`
	IP        string `json:"ip"`
	Used      int64  `json:"used"`
	LastRx    int64  `json:"last_rx"`
	LastTx    int64  `json:"last_tx"`
	Notified  bool   `json:"notified"`
	Throttled bool   `json:"throttled"`
	LastSeen  int64  `json:"last_seen"`
}

func validQuota(q *Quota) error {
	q.MAC = normalizeMac(q.MAC)
	if q.MAC == "" {
		return fmt.Errorf("invalid mac")
	}
	if !reIPv4.MatchString(q.IP) {
		return fmt.Errorf("invalid ip")
	}
	if q.Period != "daily" && q.Period != "monthly" {
		return fmt.Errorf("period must be daily or monthly")
	}
	if q.Limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	if q.Action != "notify" && q.Action != "throttle" {
		return fmt.Errorf("action must be notify or throttle")
	}
	if q.Action == "throttle" && q.ThrottleKbps <= 0 {
		return fmt.Errorf("throttle_kbps must be positive")
	}
	return nil
}

// quotaPeriodKey returns the stable bucket label for a period at time t in
// the router's local time: "2006-01-02" (daily) or "2006-01" (monthly).
func quotaPeriodKey(t time.Time, period string) string {
	if period == "monthly" {
		return t.Format("2006-01")
	}
	return t.Format("2006-01-02")
}

func quotaExceeded(used, limit int64) bool {
	return used >= limit
}

func quotaRemaining(used, limit int64) int64 {
	if used >= limit {
		return 0
	}
	return limit - used
}

// shouldNotifyOnce reports whether a quota-exceeded notification should fire:
// the cap is crossed and no notification has been sent yet this period.
func shouldNotifyOnce(exceeded, notified bool) bool {
	return exceeded && !notified
}

func loadQuotaConfig() map[string]Quota {
	out := map[string]Quota{}
	if data, err := os.ReadFile(quotaConfigFile); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func saveQuotaConfig(quotas map[string]Quota) error {
	if err := os.MkdirAll(filepath.Dir(quotaConfigFile), 0o750); err != nil {
		return err
	}
	if len(quotas) == 0 {
		_ = os.Remove(quotaConfigFile)
		return nil
	}
	data, err := json.MarshalIndent(quotas, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(quotaConfigFile, data)
}

func loadQuotaState() map[string]*quotaStateEntry {
	out := map[string]*quotaStateEntry{}
	if data, err := os.ReadFile(quotaStateFile); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func saveQuotaState(state map[string]*quotaStateEntry) error {
	if err := os.MkdirAll(filepath.Dir(quotaStateFile), 0o750); err != nil {
		return err
	}
	if len(state) == 0 {
		_ = os.Remove(quotaStateFile)
		return nil
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(quotaStateFile, data)
}

// quotaCounter holds the raw nftables counters for one IP.
type quotaCounter struct {
	upload   int64
	download int64
}

// buildQuotaRuleset generates the nftables table for accounting (upload and
// download counters) and throttling (hard cap via limit rate). Accounting
// chains run at priority -150 (after the throttle chains at -200) so the
// counters measure the traffic that actually passes.
func buildQuotaRuleset(quotas map[string]Quota, throttle map[string]bool) string {
	entries := make([]Quota, 0, len(quotas))
	for _, q := range quotas {
		entries = append(entries, q)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].IP < entries[j].IP })

	var b strings.Builder
	b.WriteString(fmt.Sprintf("table inet %s {\n", quotaTable))

	b.WriteString("  chain upload {\n")
	b.WriteString("    type filter hook prerouting priority -150; policy accept;\n")
	for _, q := range entries {
		b.WriteString(fmt.Sprintf("    ip saddr %s counter\n", q.IP))
	}
	b.WriteString("  }\n")

	b.WriteString("  chain download {\n")
	b.WriteString("    type filter hook postrouting priority -150; policy accept;\n")
	for _, q := range entries {
		b.WriteString(fmt.Sprintf("    ip daddr %s counter\n", q.IP))
	}
	b.WriteString("  }\n")

	b.WriteString("  chain throttle_upload {\n")
	b.WriteString("    type filter hook prerouting priority -200; policy accept;\n")
	for _, q := range entries {
		if throttle[q.MAC] && q.Action == "throttle" && q.ThrottleKbps > 0 {
			b.WriteString(fmt.Sprintf("    ip saddr %s limit rate over %d kbytes/second drop\n", q.IP, throttleRate(q.ThrottleKbps)))
		}
	}
	b.WriteString("  }\n")

	b.WriteString("  chain throttle_download {\n")
	b.WriteString("    type filter hook postrouting priority -200; policy accept;\n")
	for _, q := range entries {
		if throttle[q.MAC] && q.Action == "throttle" && q.ThrottleKbps > 0 {
			b.WriteString(fmt.Sprintf("    ip daddr %s limit rate over %d kbytes/second drop\n", q.IP, throttleRate(q.ThrottleKbps)))
		}
	}
	b.WriteString("  }\n")

	b.WriteString("}\n")
	return b.String()
}

// throttleRate converts kbps to the kbytes/second token nftables expects,
// matching nftqos.go's decimal convention (Mbps * 125). A floor of 1 avoids
// a degenerate zero-rate drop that would block everything.
func throttleRate(kbps int) int {
	rate := kbps / 8
	if rate < 1 {
		return 1
	}
	return rate
}

// applyQuotaTable replaces the netgrip_quota table with the given ruleset.
// It only ever touches the quota table, never netgrip_qos or any other
// NetGrip/firewall table.
func applyQuotaTable(rules string) error {
	if exec.Command("nft", "list", "table", "inet", quotaTable).Run() == nil {
		if out, err := exec.Command("nft", "delete", "table", "inet", quotaTable).CombinedOutput(); err != nil {
			return fmt.Errorf("nft delete table %s: %w (%s)", quotaTable, err, strings.TrimSpace(string(out)))
		}
	}
	if err := writeFileAtomic(quotaRulesFile, []byte(rules)); err != nil {
		return err
	}
	out, err := exec.Command("nft", "-f", quotaRulesFile).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft -f %s: %w (%s)", quotaRulesFile, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func dropQuotaTable() {
	if exec.Command("nft", "list", "table", "inet", quotaTable).Run() == nil {
		_ = exec.Command("nft", "delete", "table", "inet", quotaTable).Run()
	}
}

// nftListJSON is the subset of `nft -j list` needed to read counter bytes per
// rule chain.
type nftListJSON struct {
	Nftables []struct {
		Rule *struct {
			Chain string `json:"chain"`
			Expr  []struct {
				Match *struct {
					Left struct {
						Payload struct {
							Field string `json:"field"`
						} `json:"payload"`
					} `json:"left"`
					Right string `json:"right"`
				} `json:"match"`
				Counter *struct {
					Bytes int64 `json:"bytes"`
				} `json:"counter"`
			} `json:"expr"`
		} `json:"rule"`
	} `json:"nftables"`
}

// readQuotaCounters reads the per-IP upload/download counter bytes from the
// netgrip_quota table. It returns an error if nft is unavailable or the JSON
// cannot be parsed (the caller treats that as a fail-open accounting error).
func readQuotaCounters() (map[string]quotaCounter, error) {
	out := map[string]quotaCounter{}
	raw, err := exec.Command("nft", "-j", "list", "table", "inet", quotaTable).Output()
	if err != nil {
		return nil, err
	}
	var parsed nftListJSON
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	for _, item := range parsed.Nftables {
		if item.Rule == nil {
			continue
		}
		if item.Rule.Chain != "upload" && item.Rule.Chain != "download" {
			continue
		}
		var ip string
		var bytes int64
		for _, e := range item.Rule.Expr {
			if e.Match != nil && (e.Match.Left.Payload.Field == "saddr" || e.Match.Left.Payload.Field == "daddr") {
				ip = e.Match.Right
			}
			if e.Counter != nil {
				bytes = e.Counter.Bytes
			}
		}
		if ip == "" {
			continue
		}
		c := out[ip]
		if item.Rule.Chain == "upload" {
			c.upload = bytes
		} else {
			c.download = bytes
		}
		out[ip] = c
	}
	return out, nil
}

// foldQuotaCounters adds the traffic since the last fold into each entry's
// Used total. A raw counter that went backwards (table rebuilt, reboot)
// is treated as a fresh baseline so the small remainder is added and nothing
// is double counted. It reports whether any Used value changed.
func foldQuotaCounters(quotas map[string]Quota, state map[string]*quotaStateEntry) (bool, error) {
	counters, err := readQuotaCounters()
	if err != nil {
		return false, err
	}
	changed := false
	for mac, q := range quotas {
		e := state[mac]
		if e == nil || q.IP == "" {
			continue
		}
		c := counters[q.IP]
		if e.LastRx < 0 || e.LastTx < 0 {
			e.LastRx, e.LastTx = c.upload, c.download
			continue
		}
		var du, dd int64
		if c.upload >= e.LastRx {
			du = c.upload - e.LastRx
		} else {
			du = c.upload
		}
		if c.download >= e.LastTx {
			dd = c.download - e.LastTx
		} else {
			dd = c.download
		}
		if du != 0 || dd != 0 {
			e.Used += du + dd
			changed = true
		}
		e.LastRx, e.LastTx = c.upload, c.download
	}
	return changed, nil
}

// rebuildQuotaTable folds any pending traffic, applies the ruleset (with the
// current throttle set from state) and resets the raw-counter baselines to
// zero, since a rebuild restarts every counter. Caller must hold quotaMu.
func rebuildQuotaTable(quotas map[string]Quota, state map[string]*quotaStateEntry) error {
	if _, err := foldQuotaCounters(quotas, state); err != nil {
		log.Printf("netgrip: quota accounting before rebuild: %v", err)
	}
	throttle := map[string]bool{}
	for mac, e := range state {
		if e.Throttled {
			throttle[mac] = true
		}
	}
	if err := applyQuotaTable(buildQuotaRuleset(quotas, throttle)); err != nil {
		return err
	}
	for _, e := range state {
		e.LastRx, e.LastTx = 0, 0
	}
	return saveQuotaState(state)
}

// quotaMu serialises state mutations (scheduler tick + immediate upsert/delete)
// so the fold-read-modify-write sequence is never interleaved.
var quotaMu sync.Mutex

// ProbeQuota returns the current quota config and usage.
func ProbeQuota() *QuotaProbe {
	quotas := loadQuotaConfig()
	state := loadQuotaState()
	return &QuotaProbe{
		Applicable: hasWan(),
		Quotas:     quotas,
		Usage:      quotaUsageFor(quotas, state),
		Ts:         time.Now().UnixMilli(),
	}
}

func quotaUsageFor(quotas map[string]Quota, state map[string]*quotaStateEntry) map[string]QuotaUsage {
	out := map[string]QuotaUsage{}
	for mac, q := range quotas {
		var used int64
		var throttled bool
		if e := state[mac]; e != nil {
			used = e.Used
			throttled = e.Throttled
		}
		out[mac] = QuotaUsage{
			Used:      used,
			Limit:     q.Limit,
			Remaining: quotaRemaining(used, q.Limit),
			Period:    q.Period,
			Throttled: throttled,
			Exceeded:  quotaExceeded(used, q.Limit),
		}
	}
	return out
}

// QuotaUsageForClients exposes the current per-MAC usage for the clients
// payload. It never fails: a missing or unreadable state simply yields no
// quota fields for that client.
func QuotaUsageForClients() map[string]QuotaUsage {
	return quotaUsageFor(loadQuotaConfig(), loadQuotaState())
}

// SetQuota upserts one quota and rebuilds the accounting/throttle ruleset.
func SetQuota(q Quota) (*QuotaProbe, error) {
	invalidateClients()
	if err := validQuota(&q); err != nil {
		return ProbeQuota(), err
	}
	if !hasWan() {
		return ProbeQuota(), fmt.Errorf("per-client quotas only apply to the gateway router")
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()
	quotas := loadQuotaConfig()
	quotas[q.MAC] = q
	if err := saveQuotaConfig(quotas); err != nil {
		return ProbeQuota(), err
	}
	state := loadQuotaState()
	if err := rebuildQuotaTable(quotas, state); err != nil {
		log.Printf("netgrip: quota apply: %v", err)
		return ProbeQuota(), err
	}
	return ProbeQuota(), nil
}

// RemoveQuota deletes one quota and rebuilds the ruleset.
func RemoveQuota(mac string) (*QuotaProbe, error) {
	invalidateClients()
	mac = normalizeMac(mac)
	if mac == "" {
		return ProbeQuota(), fmt.Errorf("invalid mac")
	}
	if !hasWan() {
		return ProbeQuota(), fmt.Errorf("per-client quotas only apply to the gateway router")
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()
	quotas := loadQuotaConfig()
	if _, ok := quotas[mac]; !ok {
		return ProbeQuota(), nil
	}
	delete(quotas, mac)
	if err := saveQuotaConfig(quotas); err != nil {
		return ProbeQuota(), err
	}
	state := loadQuotaState()
	delete(state, mac)
	if err := rebuildQuotaTable(quotas, state); err != nil {
		log.Printf("netgrip: quota apply: %v", err)
		return ProbeQuota(), err
	}
	return ProbeQuota(), nil
}

// ApplyQuotaAtBoot rebuilds the accounting/throttle table from persisted
// config + state on startup. It deliberately leaves the persisted raw-counter
// baselines untouched so the first fold detects the post-reboot reset and only
// adds the new traffic.
func ApplyQuotaAtBoot() {
	quotas := loadQuotaConfig()
	if len(quotas) == 0 {
		return
	}
	if !hasWan() {
		return
	}
	state := loadQuotaState()
	throttle := map[string]bool{}
	for mac, e := range state {
		if e.Throttled {
			throttle[mac] = true
		}
	}
	if err := applyQuotaTable(buildQuotaRuleset(quotas, throttle)); err != nil {
		log.Printf("netgrip: quota boot apply: %v", err)
	}
}

func init() {
	ApplyQuotaAtBoot()
}

var (
	quotaStartOnce sync.Once
	quotaLastSave  time.Time
)

// StartQuotaScheduler runs the enforcement loop every 60 seconds; the first
// tick runs immediately so state is correct after a reboot.
func StartQuotaScheduler() {
	quotaStartOnce.Do(func() {
		log.Printf("netgrip: quota scheduler started")
		go func() {
			quotaTick()
			for {
				time.Sleep(60 * time.Second)
				quotaTick()
			}
		}()
	})
}

func quotaTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("netgrip: quota scheduler recovered: %v", r)
		}
	}()
	if !hasWan() {
		return
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()

	quotas := loadQuotaConfig()
	if len(quotas) == 0 {
		dropQuotaTable()
		return
	}
	state := loadQuotaState()
	now, _ := routerLocalNow()
	changed := false

	for mac, q := range quotas {
		e := state[mac]
		if e == nil {
			e = &quotaStateEntry{LastRx: -1, LastTx: -1}
			state[mac] = e
			changed = true
		}
		key := quotaPeriodKey(now, q.Period)
		if e.PeriodKey != key {
			e.PeriodKey = key
			e.Used = 0
			e.Notified = false
			changed = true
		}
		e.Period = q.Period
		if e.IP != q.IP {
			e.IP = q.IP
			changed = true
		}
		e.LastSeen = now.Unix()
	}

	folded, err := foldQuotaCounters(quotas, state)
	if err != nil {
		// Fail-open: an accounting error must never block traffic. Skip
		// enforcement this tick and warn loudly.
		log.Printf("netgrip: quota accounting: %v", err)
		return
	}
	if folded {
		changed = true
	}

	for mac, q := range quotas {
		e := state[mac]
		if q.Action == "notify" && shouldNotifyOnce(quotaExceeded(e.Used, q.Limit), e.Notified) {
			notifyQuotaExceeded(q, e)
			e.Notified = true
			changed = true
		}
	}

	prevThrottle := map[string]bool{}
	for mac, e := range state {
		prevThrottle[mac] = e.Throttled
	}
	throttleChanged := false
	for mac, q := range quotas {
		e := state[mac]
		desired := q.Action == "throttle" && quotaExceeded(e.Used, q.Limit)
		if e.Throttled != desired {
			e.Throttled = desired
			throttleChanged = true
		}
	}

	if throttleChanged {
		if err := rebuildQuotaTable(quotas, state); err != nil {
			log.Printf("netgrip: quota throttle apply: %v", err)
			for mac := range state {
				state[mac].Throttled = prevThrottle[mac]
			}
			_ = saveQuotaState(state)
		}
	} else if changed || time.Since(quotaLastSave) >= 5*time.Minute {
		if err := saveQuotaState(state); err == nil {
			quotaLastSave = time.Now()
		}
	}
}

func notifyQuotaExceeded(q Quota, e *quotaStateEntry) {
	name := q.MAC
	if m := clientMeta(); m[q.MAC].Name != "" {
		name = m[q.MAC].Name
	}
	period := "daily"
	if q.Period == "monthly" {
		period = "monthly"
	}
	notifyAll("NetGrip",
		fmt.Sprintf("📊 <b>NetGrip</b>\n%s reached its %s data quota: %s of %s used",
			htmlEsc(name), period, humanBytes(e.Used), humanBytes(q.Limit)),
		fmt.Sprintf("📊 NetGrip\n%s reached its %s data quota: %s of %s used",
			name, period, humanBytes(e.Used), humanBytes(q.Limit)),
		false)
}

func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.0f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
