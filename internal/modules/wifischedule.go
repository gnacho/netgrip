package modules

// WiFi schedules (#445): turn a wireless interface (SSID broadcast) off
// outside a weekly time window. The schedule lives in the netgrip UCI
// package (netgrip.wifischedule.<section>) so wireless stays clean; the
// scheduler owns wireless.@wifi-iface[N].disabled only for sections it has
// disabled itself, tracked in /etc/netgrip/wifischedule-applied.json.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// WifiSchedule is the per-interface schedule. Start/end are "HH:MM" in the
// router's local time; end earlier than start means the window crosses
// midnight. Days use Go's weekday numbering (0=Sunday..6=Saturday). Paused
// is a manual "off now" override that wins over the window.
type WifiSchedule struct {
	Section string `json:"section"`
	Enabled bool   `json:"enabled"`
	Days    []int  `json:"days"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Paused  bool   `json:"paused"`
}

// WifiScheduleState is the API view of one interface: current broadcast
// state plus its schedule and why it is off right now.
type WifiScheduleState struct {
	Section       string        `json:"section"`
	Ifname        string        `json:"ifname"`
	SSID          string        `json:"ssid"`
	Disabled      bool          `json:"disabled"`
	OffBySchedule bool          `json:"off_by_schedule"`
	Schedule      *WifiSchedule `json:"schedule,omitempty"`
}

var wifiSchedAppliedPath = "/etc/netgrip/wifischedule-applied.json"

func validWifiSchedule(s *WifiSchedule) error {
	if s.Section == "" {
		return fmt.Errorf("section is required")
	}
	if !reHHMM.MatchString(s.Start) || !reHHMM.MatchString(s.End) {
		return fmt.Errorf("start and end must be HH:MM")
	}
	seen := map[int]bool{}
	for _, d := range s.Days {
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
	s.Days = days
	return nil
}

// wifiScheduleWindowAt reports whether t is inside the schedule's weekly
// window (no Paused handling here; that is layered in wifiScheduleOffAt).
func wifiScheduleWindowAt(t time.Time, s WifiSchedule) bool {
	if !s.Enabled {
		return false
	}
	start := hhmmToMinutes(s.Start)
	end := hhmmToMinutes(s.End)
	cur := t.Hour()*60 + t.Minute()
	if start == end {
		// degenerate window: whole selected days
		return s.dayActive(t.Weekday())
	}
	if start < end {
		return s.dayActive(t.Weekday()) && cur >= start && cur < end
	}
	// overnight: evening of a selected day, or the morning after one
	if s.dayActive(t.Weekday()) && cur >= start {
		return true
	}
	return s.dayActive(t.AddDate(0, 0, -1).Weekday()) && cur < end
}

func (s WifiSchedule) dayActive(wd time.Weekday) bool {
	for _, d := range s.Days {
		if d == int(wd) {
			return true
		}
	}
	return false
}

// wifiScheduleOffAt reports whether the interface must be off at t: the
// manual pause wins over everything, then the weekly window.
func wifiScheduleOffAt(t time.Time, s WifiSchedule) bool {
	if s.Paused {
		return true
	}
	return wifiScheduleWindowAt(t, s)
}

// wifiScheduleOps builds the executor ops that set the disabled flag on the
// given sections and reconfigure their radios. Pure builder, unit-testable.
func wifiScheduleOps(sections []string, disabled bool) []executor.Op {
	val := "0"
	if disabled {
		val = "1"
	}
	var ops []executor.Op
	for _, sec := range sections {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"wireless." + sec + ".disabled", val}})
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"wireless"}})
	for _, radio := range radiosForSections(sections) {
		ops = append(ops, executor.Op{Kind: "wifi_reconf", Args: []string{radio}})
	}
	return ops
}

// wifiSchedUCIKey addresses one schedule section. UCI named sections live
// flat in the package (netgrip.<name>), so the iface name is prefixed to
// avoid colliding with other netgrip sections (main, selfupdate, poe ports).
func wifiSchedUCIKey(section string) string {
	return "netgrip.wifisec_" + sanitizeUCIKey(section)
}

func ensureWifiSchedSection(section string) {
	_ = EnsureNetgripSection("wifischedule", "wifischedule")
	secName := "wifisec_" + sanitizeUCIKey(section)
	if !uciSectionExists("netgrip." + secName) {
		cmd := exec.Command("uci", "-m", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config wifisched '" + secName + "'\n")
		_ = cmd.Run()
	}
}

func loadWifiSchedule(section string) (WifiSchedule, bool) {
	base := wifiSchedUCIKey(section)
	if !uciSectionExists(base) {
		return WifiSchedule{}, false
	}
	s := WifiSchedule{
		Section: section,
		Enabled: uciGet(base+".enabled") == "1",
		Start:   uciGet(base + ".start"),
		End:     uciGet(base + ".end"),
		Paused:  uciGet(base+".paused") == "1",
	}
	if s.Start == "" && s.End == "" {
		return WifiSchedule{}, false
	}
	for _, d := range strings.Fields(uciGet(base + ".days")) {
		if n, err := strconv.Atoi(d); err == nil && n >= 0 && n <= 6 {
			s.Days = append(s.Days, n)
		}
	}
	return s, true
}

// ProbeWifiSchedule returns the schedule state of every wireless interface.
func ProbeWifiSchedule() []WifiScheduleState {
	now, _ := routerLocalNow()
	out := make([]WifiScheduleState, 0)
	for _, sec := range wifiIfaceSections() {
		st := WifiScheduleState{
			Section:  sec,
			Ifname:   wifiIfnameForSection(sec),
			SSID:     uciGet("wireless." + sec + ".ssid"),
			Disabled: uciGet("wireless."+sec+".disabled") == "1",
		}
		if sched, ok := loadWifiSchedule(sec); ok {
			s := sched
			st.Schedule = &s
			st.OffBySchedule = wifiScheduleOffAt(now, s)
		}
		out = append(out, st)
	}
	return out
}

func wifiIfnameForSection(section string) string {
	if out := uciGet("wireless." + section + ".ifname"); out != "" {
		return out
	}
	return uciGet("wireless." + section + ".network")
}

func loadWifiSchedLedger() map[string]bool {
	out := map[string]bool{}
	if data, err := os.ReadFile(wifiSchedAppliedPath); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

func saveWifiSchedLedger(ledger map[string]bool) error {
	if err := os.MkdirAll("/etc/netgrip", 0o750); err != nil {
		return err
	}
	if len(ledger) == 0 {
		_ = os.Remove(wifiSchedAppliedPath)
		return nil
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		return err
	}
	return writeFileAtomic(wifiSchedAppliedPath, data)
}

var wifiSchedMu sync.Mutex

// SetWifiSchedule persists one schedule and applies its effect immediately
// (disable inside the window / paused, enable otherwise).
func SetWifiSchedule(req WifiSchedule) ([]WifiScheduleState, bool, error) {
	InvalidateKey("wireless")
	InvalidateKey("ucishow|wireless")
	if err := validWifiSchedule(&req); err != nil {
		return ProbeWifiSchedule(), false, err
	}
	if !uciSectionExists("wireless." + req.Section) {
		return ProbeWifiSchedule(), false, fmt.Errorf("unknown wireless section: %q", req.Section)
	}

	ensureWifiSchedSection(req.Section)
	key := wifiSchedUCIKey(req.Section)
	val := "0"
	if req.Enabled {
		val = "1"
	}
	paused := "0"
	if req.Paused {
		paused = "1"
	}
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{key + ".enabled", val}},
		{Kind: "uci_set", Args: []string{key + ".start", req.Start}},
		{Kind: "uci_set", Args: []string{key + ".end", req.End}},
		{Kind: "uci_set", Args: []string{key + ".paused", paused}},
		{Kind: "uci_delete", Args: []string{key + ".days"}},
	}
	for _, d := range req.Days {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{key + ".days", strconv.Itoa(d)}})
	}
	if err := applyNetgripConfig(ops); err != nil {
		return ProbeWifiSchedule(), true, err
	}

	now, _ := routerLocalNow()
	rolledBack := reconcileWifiSchedule(req.Section, wifiScheduleOffAt(now, req))
	return ProbeWifiSchedule(), rolledBack, nil
}

// reconcileWifiSchedule brings one interface's disabled flag in line with
// the schedule. Transitions act only when needed; the ledger makes the
// scheduler re-enable only interfaces it disabled itself, so a manual
// disable from the WiFi editor is never clobbered.
func reconcileWifiSchedule(section string, off bool) (rolledBack bool) {
	wifiSchedMu.Lock()
	defer wifiSchedMu.Unlock()

	current := uciGet("wireless."+section+".disabled") == "1"
	ledger := loadWifiSchedLedger()
	if off && !current {
		snap, err := executor.Snapshot("wireless")
		if err != nil {
			log.Printf("netgrip: wifi schedule snapshot %s: %v", section, err)
			return false
		}
		ops := wifiScheduleOps([]string{section}, true)
		if err := executor.Apply(ops, nil); err != nil {
			_ = executor.Restore("wireless", snap)
			_ = executor.Run(executor.Op{Kind: "wifi_reconf", Args: []string{}})
			log.Printf("netgrip: wifi schedule disable %s: %v", section, err)
			return true
		}
		ledger[section] = true
		_ = saveWifiSchedLedger(ledger)
		return false
	}
	if !off && current && ledger[section] {
		snap, err := executor.Snapshot("wireless")
		if err != nil {
			log.Printf("netgrip: wifi schedule snapshot %s: %v", section, err)
			return false
		}
		rollback := func() {
			_ = executor.Restore("wireless", snap)
			_ = executor.Run(executor.Op{Kind: "wifi_reconf", Args: []string{}})
		}
		ops := wifiScheduleOps([]string{section}, false)
		if err := executor.Apply(ops, nil); err != nil {
			rollback()
			log.Printf("netgrip: wifi schedule enable %s: %v", section, err)
			return true
		}
		// Healthcheck: a re-enabled interface must come back up, else the
		// rollback restores the snapshot and we keep the ledger entry.
		if !waitIfaceUp(section) {
			rollback()
			log.Printf("netgrip: wifi schedule healthcheck failed for %s, rolled back", section)
			return true
		}
		delete(ledger, section)
		_ = saveWifiSchedLedger(ledger)
	}
	return false
}

// waitIfaceUp polls the interface up to ~16s after a reconf; coming back up
// takes a moment, so an immediate read could wrongly trigger a rollback.
func waitIfaceUp(section string) bool {
	for i := 0; i < 8; i++ {
		if uciGet("wireless."+section+".disabled") != "1" && ifaceLive(section) {
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func ifaceLive(section string) bool {
	ui, err := ProbeWifiUI()
	if err != nil {
		return false
	}
	for _, u := range ui {
		if u.Section == section {
			return !u.Disabled
		}
	}
	return false
}

var wifiSchedStartOnce sync.Once

// StartWifiScheduleScheduler runs the background loop that reconciles the
// WiFi schedules every 30 seconds; the first tick runs immediately so state
// is correct after a reboot.
func StartWifiScheduleScheduler() {
	wifiSchedStartOnce.Do(func() {
		log.Printf("netgrip: wifi schedule scheduler started")
		go func() {
			wifiSchedTick()
			for {
				time.Sleep(30 * time.Second)
				wifiSchedTick()
			}
		}()
	})
}

func wifiSchedTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("netgrip: wifi schedule scheduler recovered: %v", r)
		}
	}()
	now, _ := routerLocalNow()
	for _, sec := range wifiIfaceSections() {
		sched, ok := loadWifiSchedule(sec)
		if !ok {
			continue
		}
		reconcileWifiSchedule(sec, wifiScheduleOffAt(now, sched))
	}
	// Self-heal: a schedule deleted from UCI while its window was active
	// (manual cleanup, factory edit) leaves the interface off with no rule
	// to ever bring it back. The ledger proves we are the ones who disabled
	// it, so re-enable once and drop the entry.
	for sec := range loadWifiSchedLedger() {
		if _, ok := loadWifiSchedule(sec); !ok {
			reconcileWifiSchedule(sec, false)
		}
	}
}
