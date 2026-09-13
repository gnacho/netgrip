package modules

import (
	"encoding/json"
	"fmt"
	"log"
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
	"github.com/gnacho/netgrip/internal/ubus"
)

// Scheduled parental controls (#304): per-device (MAC) time windows with
// overnight support and a "pause now" override, applied through the existing
// per-client blocking. User intent lives in /etc/netgrip/parental.json; the
// scheduler's wifi ownership ledger lives in /etc/netgrip/parental-applied.json.

var (
	parentalPath        = "/etc/netgrip/parental.json"
	parentalAppliedPath = "/etc/netgrip/parental-applied.json"
)

// ParentalRule is one per-device schedule. Start/end are "HH:MM" in the
// router's local time; end earlier than start means the window crosses
// midnight. Days use Go's weekday numbering (0=Sunday..6=Saturday).
type ParentalRule struct {
	MAC     string `json:"mac"`
	Enabled bool   `json:"enabled"`
	Days    []int  `json:"days"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Paused  bool   `json:"paused"`
}

// ParentalProbe is the API view: all rules keyed by MAC.
type ParentalProbe struct {
	Rules map[string]ParentalRule `json:"rules"`
	Ts    int64                   `json:"ts"`
}

var reHHMM = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func validParentalRule(r *ParentalRule) error {
	r.MAC = normalizeMac(r.MAC)
	if r.MAC == "" {
		return fmt.Errorf("invalid mac")
	}
	if !reHHMM.MatchString(r.Start) || !reHHMM.MatchString(r.End) {
		return fmt.Errorf("start and end must be HH:MM")
	}
	seen := map[int]bool{}
	for _, d := range r.Days {
		if d < 0 || d > 6 {
			return fmt.Errorf("days must be 0..6")
		}
		seen[d] = true
	}
	if len(seen) == 0 {
		return fmt.Errorf("at least one day required")
	}
	days := make([]int, 0, len(seen))
	for d := range seen {
		days = append(days, d)
	}
	sort.Ints(days)
	r.Days = days
	return nil
}

// hhmmToMinutes converts "HH:MM" to minutes since midnight.
func hhmmToMinutes(s string) int {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}

// dayActive reports whether the Go weekday (Sunday=0) is in the rule's days.
func (r ParentalRule) dayActive(wd time.Weekday) bool {
	for _, d := range r.Days {
		if d == int(wd) {
			return true
		}
	}
	return false
}

// parentalBlocksAt reports whether the rule blocks the device at local time t.
// Paused wins over the schedule; otherwise the window must be open today.
func parentalBlocksAt(t time.Time, r ParentalRule) bool {
	if r.Paused {
		return true
	}
	if !r.Enabled {
		return false
	}
	start := hhmmToMinutes(r.Start)
	end := hhmmToMinutes(r.End)
	cur := t.Hour()*60 + t.Minute()
	if start == end {
		// degenerate window: whole selected days
		return r.dayActive(t.Weekday())
	}
	if start < end {
		return r.dayActive(t.Weekday()) && cur >= start && cur < end
	}
	// overnight: evening of a selected day, or the morning after one
	if r.dayActive(t.Weekday()) && cur >= start {
		return true
	}
	return r.dayActive(t.AddDate(0, 0, -1).Weekday()) && cur < end
}

// parentalNextToday returns the "HH:MM" of the next block start that is still
// ahead today on a selected day, or "" when there is none (used by the pill).
func parentalNextToday(t time.Time, r ParentalRule) string {
	if r.Paused || !r.Enabled {
		return ""
	}
	start := hhmmToMinutes(r.Start)
	cur := t.Hour()*60 + t.Minute()
	if r.dayActive(t.Weekday()) && cur < start {
		return r.Start
	}
	return ""
}

// routerLocalNow returns the router's wall-clock time in local time. Go does
// not understand OpenWrt's busybox /tmp/TZ (time.Now() runs in UTC), so we ask
// the system (extending routerLocalHour to full date/time + weekday).
func routerLocalNow() (time.Time, bool) {
	out, err := exec.Command("date", "+%Y %m %d %H %M %w").Output()
	if err != nil {
		return time.Now(), false
	}
	f := strings.Fields(strings.TrimSpace(string(out)))
	if len(f) != 6 {
		return time.Now(), false
	}
	nums := make([]int, 6)
	for i, s := range f {
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Now(), false
		}
		nums[i] = n
	}
	return time.Date(nums[0], time.Month(nums[1]), nums[2], nums[3], nums[4], 0, 0, time.Local), true
}

func loadParentalRules() map[string]ParentalRule {
	out := map[string]ParentalRule{}
	if data, err := os.ReadFile(parentalPath); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func saveParentalRules(rules map[string]ParentalRule) error {
	if err := os.MkdirAll(filepath.Dir(parentalPath), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(parentalPath, data)
}

// loadParentalLedger returns the MACs the scheduler has added to the wifi
// macfilter deny list for parental reasons. The file is missing on first boot;
// a missing ledger means "not owned", so the scheduler never auto-removes a
// deny entry it cannot prove it owns (fail-safe).
func loadParentalLedger() map[string]bool {
	out := map[string]bool{}
	if data, err := os.ReadFile(parentalAppliedPath); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func saveParentalLedger(ledger map[string]bool) error {
	if err := os.MkdirAll(filepath.Dir(parentalAppliedPath), 0o750); err != nil {
		return err
	}
	if len(ledger) == 0 {
		_ = os.Remove(parentalAppliedPath)
		return nil
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(parentalAppliedPath, data)
}

// ProbeParental returns every schedule rule.
func ProbeParental() ParentalProbe {
	return ParentalProbe{Rules: loadParentalRules(), Ts: time.Now().UnixMilli()}
}

// SetParentalRule upserts one schedule and applies its block state immediately.
func SetParentalRule(req ParentalRule) (ParentalProbe, error) {
	if err := validParentalRule(&req); err != nil {
		return ProbeParental(), err
	}
	rules := loadParentalRules()
	rules[req.MAC] = req
	if err := saveParentalRules(rules); err != nil {
		return ProbeParental(), err
	}
	now, _ := routerLocalNow()
	reconcileParental(req.MAC, parentalBlocksAt(now, req))
	return ProbeParental(), nil
}

// RemoveParentalRule deletes a schedule and clears any applied block.
func RemoveParentalRule(mac string) (ParentalProbe, error) {
	mac = normalizeMac(mac)
	if mac == "" {
		return ProbeParental(), fmt.Errorf("invalid mac")
	}
	rules := loadParentalRules()
	delete(rules, mac)
	if err := saveParentalRules(rules); err != nil {
		return ProbeParental(), err
	}
	reconcileParental(mac, false)
	return ProbeParental(), nil
}

// parentalActiveNow computes, for each rule, whether it blocks right now and
// the "next block today" hint. Reads fresh state so manual edits take effect.
func parentalActiveNow() (active map[string]bool, next map[string]string) {
	now, _ := routerLocalNow()
	active = map[string]bool{}
	next = map[string]string{}
	for mac, r := range loadParentalRules() {
		mac = strings.ToLower(mac)
		if parentalBlocksAt(now, r) {
			active[mac] = true
		} else if s := parentalNextToday(now, r); s != "" {
			next[mac] = s
		}
	}
	return active, next
}

// cableParentalSet returns the MACs currently blocked by a parental firewall
// REJECT rule (netgrip_parental_* sections), distinct from manual netgrip_block_*.
func cableParentalSet() map[string]bool {
	cmdOut, _ := exec.Command("sh", "-c", "uci show firewall | grep 'src_mac='").Output()
	return parseCableParental(string(cmdOut))
}

// parseCableParental extracts MACs from firewall.netgrip_parental_<mac>.src_mac
// lines, mirroring parseCableBlocked for the parental prefix.
func parseCableParental(output string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ".netgrip_parental_") {
			continue
		}
		eq := strings.Index(line, ".src_mac=")
		if eq < 0 {
			continue
		}
		mac := strings.ToLower(strings.Trim(line[eq+len(".src_mac="):], "'\""))
		if reMac.MatchString(mac) {
			out[mac] = true
		}
	}
	return out
}

// clientTypeFor reports whether a MAC is currently a wifi or cable client
// ("" when offline). Used to pick the block mechanism without persisting the
// device type: wifi goes through the macfilter deny list, cable through a
// firewall REJECT rule.
func clientTypeFor(mac string) string {
	radios, _ := ubus.GetWirelessStatus()
	for _, radio := range radios {
		for _, iface := range radio.Interfaces {
			for _, wc := range iface.Clients {
				if strings.ToLower(wc.MAC) == mac {
					return "wifi"
				}
			}
		}
	}
	wifiPorts := map[string]bool{}
	for _, radio := range radios {
		for _, iface := range radio.Interfaces {
			wifiPorts[iface.Ifname] = true
		}
	}
	for port, macs := range bridgeFdb() {
		if wifiPorts[port] {
			continue
		}
		for _, m := range macs {
			if strings.ToLower(m) == mac {
				return "cable"
			}
		}
	}
	return ""
}

// applyParentalWifi adds or removes a MAC from every wifi-iface macfilter deny
// list, mirroring SetClientBlocked (radio reconf instead of a full reload).
func applyParentalWifi(mac string, blocked bool) error {
	snap, err := executor.Snapshot("wireless")
	if err != nil {
		return err
	}
	rollback := func() {
		_ = executor.Restore("wireless", snap)
		_ = executor.Run(executor.Op{Kind: "wifi_reconf", Args: []string{}})
	}
	sections := wifiIfaceSections()
	var ops []executor.Op
	for _, section := range sections {
		base := "wireless." + section
		if blocked {
			if uciGet(base+".macfilter") != "deny" {
				ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{base + ".macfilter", "deny"}})
			}
			present := false
			for _, m := range strings.Fields(strings.ToLower(uciGet(base + ".maclist"))) {
				if m == mac {
					present = true
				}
			}
			if !present {
				ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{base + ".maclist", mac}})
			}
		} else {
			for _, m := range strings.Fields(strings.ToLower(uciGet(base + ".maclist"))) {
				if m == mac {
					ops = append(ops, executor.Op{Kind: "uci_del_list", Args: []string{base + ".maclist", mac}})
				}
			}
		}
	}
	if len(ops) == 0 {
		return nil
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	for _, radio := range radiosForSections(sections) {
		ops = append(ops, executor.Op{Kind: "wifi_reconf", Args: []string{radio}})
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return err
	}
	return nil
}

// applyParentalCable adds or removes a parental firewall REJECT rule, mirroring
// setCableBlocked but under the netgrip_parental_ prefix so manual and parental
// cable blocks never clobber each other.
func applyParentalCable(mac string, blocked bool) error {
	if !executor.ServiceEnabled("firewall") {
		return fmt.Errorf("blocking wired clients needs the firewall (gateway)")
	}
	snap, err := executor.Snapshot("firewall")
	if err != nil {
		return err
	}
	rollback := func() {
		_ = executor.Restore("firewall", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}})
	}
	section := "netgrip_parental_" + strings.ReplaceAll(mac, ":", "")
	base := "firewall." + section
	var ops []executor.Op
	if blocked {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{base, "rule"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".name", "netgrip-parental-" + mac}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".src", "lan"}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".src_mac", mac}},
			executor.Op{Kind: "uci_set", Args: []string{base + ".target", "REJECT"}},
		)
	} else if uciSectionExists(base) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{base}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"firewall"}},
		executor.Op{Kind: "initd", Args: []string{"firewall", "reload"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return err
	}
	return nil
}

// parentalMu serialises reconciles (scheduler tick + immediate POST/DELETE
// applies) so the wifi applied-ledger read-modify-write is never interleaved.
var parentalMu sync.Mutex

// reconcileParental brings one MAC's applied block in line with desired,
// acting only on transitions. Cable blocks use the netgrip_parental_* firewall
// sections (owned by prefix); wifi blocks share the macfilter deny list and are
// guarded by the applied-ledger so a manual block is never removed.
func reconcileParental(mac string, desired bool) {
	parentalMu.Lock()
	defer parentalMu.Unlock()
	ledger := loadParentalLedger()
	if desired {
		typ := clientTypeFor(mac)
		if typ != "cable" {
			denied, avail := blockedBands()
			if !blockedEverywhere(denied[mac], avail) {
				if err := applyParentalWifi(mac, true); err != nil {
					log.Printf("netgrip: parental wifi block %s: %v", mac, err)
				} else {
					ledger[mac] = true
					_ = saveParentalLedger(ledger)
				}
			}
		}
		if typ != "wifi" && executor.ServiceEnabled("firewall") {
			if !cableParentalSet()[mac] {
				if err := applyParentalCable(mac, true); err != nil {
					log.Printf("netgrip: parental cable block %s: %v", mac, err)
				}
			}
		}
		return
	}
	if ledger[mac] {
		if err := applyParentalWifi(mac, false); err != nil {
			log.Printf("netgrip: parental wifi unblock %s: %v", mac, err)
		} else {
			delete(ledger, mac)
			_ = saveParentalLedger(ledger)
		}
	}
	if cableParentalSet()[mac] {
		if err := applyParentalCable(mac, false); err != nil {
			log.Printf("netgrip: parental cable unblock %s: %v", mac, err)
		}
	}
}

var parentalStartOnce sync.Once

// StartParentalScheduler runs the background loop that reconciles schedules
// every 30 seconds; the first tick runs immediately so state is correct after
// a reboot.
func StartParentalScheduler() {
	parentalStartOnce.Do(func() {
		log.Printf("netgrip: parental scheduler started")
		go func() {
			parentalTick()
			for {
				time.Sleep(30 * time.Second)
				parentalTick()
			}
		}()
	})
}

func parentalTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("netgrip: parental scheduler recovered: %v", r)
		}
	}()
	rules := loadParentalRules()
	if len(rules) == 0 {
		return
	}
	now, _ := routerLocalNow()
	for mac, r := range rules {
		reconcileParental(mac, parentalBlocksAt(now, r))
	}
}
