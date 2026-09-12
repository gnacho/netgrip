package modules

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Scheduled self-update (#297): standalone installs have no NetPulse server
// ordering upgrades, so the panel checks GitHub releases on its own inside a
// configurable window (night by default) and applies the verified upgrade
// path: arch-aware asset selection, ELF check and the atomic swap.
// Config lives in UCI (netgrip.selfupdate.*) and survives upgrades; runtime
// state lives in /etc/netgrip/selfupdate-state.json.

const selfUpdateStatePath = "/etc/netgrip/selfupdate-state.json"

// SelfUpdateConfig is the user-facing schedule configuration.
type SelfUpdateConfig struct {
	Enabled     bool `json:"enabled"`
	IntervalH   int  `json:"intervalHours"` // hours between checks, default 168
	WindowStart int  `json:"windowStart"`   // local hour, inclusive (default 3)
	WindowEnd   int  `json:"windowEnd"`     // local hour, exclusive (default 5, wraps midnight)
}

// SelfUpdateScheduleState is the persisted progress of the scheduler.
type SelfUpdateScheduleState struct {
	LastCheck   int64  `json:"lastCheck"`   // unix seconds
	LastResult  string `json:"lastResult"`  // "up-to-date" | "applied <ver>" | "error: ..."
	LastApply   int64  `json:"lastApply"`   // unix seconds
	LastVersion string `json:"lastVersion"` // release applied
}

func (c SelfUpdateConfig) normalized() SelfUpdateConfig {
	if c.IntervalH <= 0 {
		c.IntervalH = 168
	}
	if c.IntervalH > 24*30 {
		c.IntervalH = 24 * 30
	}
	if c.WindowStart < 0 || c.WindowStart > 23 {
		c.WindowStart = 3
	}
	if c.WindowEnd < 0 || c.WindowEnd > 24 || c.WindowEnd == c.WindowStart {
		c.WindowEnd = c.WindowStart + 2
		if c.WindowEnd > 24 {
			c.WindowEnd -= 24
		}
	}
	return c
}

// inWindow reports whether the given local hour falls inside the check
// window. A wrapping window (start > end) crosses midnight.
func (c SelfUpdateConfig) inWindow(hour int) bool {
	if c.WindowStart == c.WindowEnd {
		return true
	}
	if c.WindowStart < c.WindowEnd {
		return hour >= c.WindowStart && hour < c.WindowEnd
	}
	return hour >= c.WindowStart || hour < c.WindowEnd
}

func selfUpdateUCIGet(opt string) string {
	out, err := exec.Command("uci", "-q", "get", "netgrip.selfupdate."+opt).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadSelfUpdateConfig loads the schedule from UCI with safe defaults:
// enabled, weekly, 03:00-05:00 local.
func ReadSelfUpdateConfig() SelfUpdateConfig {
	c := SelfUpdateConfig{Enabled: true, IntervalH: 168, WindowStart: 3, WindowEnd: 5}
	if v := selfUpdateUCIGet("enabled"); v != "" {
		c.Enabled = v == "1"
	}
	if n, err := strconv.Atoi(selfUpdateUCIGet("interval")); err == nil && n > 0 {
		c.IntervalH = n
	}
	if n, err := strconv.Atoi(selfUpdateUCIGet("window_start")); err == nil {
		c.WindowStart = n
	}
	if n, err := strconv.Atoi(selfUpdateUCIGet("window_end")); err == nil {
		c.WindowEnd = n
	}
	return c.normalized()
}

// WriteSelfUpdateConfig persists the schedule to UCI.
func WriteSelfUpdateConfig(c SelfUpdateConfig) (SelfUpdateConfig, error) {
	c = c.normalized()
	sets := []string{
		"netgrip.selfupdate=selfupdate",
		"netgrip.selfupdate.enabled=" + map[bool]string{true: "1", false: "0"}[c.Enabled],
		"netgrip.selfupdate.interval=" + strconv.Itoa(c.IntervalH),
		"netgrip.selfupdate.window_start=" + strconv.Itoa(c.WindowStart),
		"netgrip.selfupdate.window_end=" + strconv.Itoa(c.WindowEnd),
	}
	for _, s := range sets {
		if out, err := exec.Command("uci", "set", s).CombinedOutput(); err != nil {
			return c, errors.New("uci set " + s + ": " + strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("uci", "commit", "netgrip").CombinedOutput(); err != nil {
		return c, errors.New("uci commit: " + strings.TrimSpace(string(out)))
	}
	return c, nil
}

var (
	selfUpdateSchedMu    sync.Mutex
	selfUpdateSchedCache *SelfUpdateScheduleState
)

func selfUpdateSchedState() SelfUpdateScheduleState {
	selfUpdateSchedMu.Lock()
	defer selfUpdateSchedMu.Unlock()
	if selfUpdateSchedCache == nil {
		st := SelfUpdateScheduleState{}
		if b, err := os.ReadFile(selfUpdateStatePath); err == nil {
			_ = json.Unmarshal(b, &st)
		}
		selfUpdateSchedCache = &st
	}
	return *selfUpdateSchedCache
}

func selfUpdateSaveState(st SelfUpdateScheduleState) {
	selfUpdateSchedMu.Lock()
	defer selfUpdateSchedMu.Unlock()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	tmp := selfUpdateStatePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return
	}
	_ = os.Rename(tmp, selfUpdateStatePath)
	selfUpdateSchedCache = &st
}

// SelfUpdateScheduleStateOf returns a copy of the persisted scheduler state.
func SelfUpdateScheduleStateOf() SelfUpdateScheduleState { return selfUpdateSchedState() }

// SelfUpdateScheduleStatus is the API view: config plus scheduler progress.
type SelfUpdateScheduleStatus struct {
	Config SelfUpdateConfig        `json:"config"`
	State  SelfUpdateScheduleState `json:"state"`
	Status SelfUpdateStatus        `json:"status"`
}

// routerLocalHour returns the hour the router itself considers "now".
// Go does not understand OpenWrt's busybox /tmp/TZ, so time.Now() runs in
// UTC while date, cron and LuCI show local time: ask the system instead.
func routerLocalHour() int {
	if out, err := exec.Command("date", "+%H").Output(); err == nil {
		if h, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			return h
		}
	}
	return time.Now().Hour()
}

// StartSelfUpdateScheduler runs the background loop: every 30 minutes it
// checks whether the window is open and the interval has elapsed, then
// performs (with jitter) a release check and applies the update when one is
// available. Dev builds never self-update.
func StartSelfUpdateScheduler(version string) {
	log.Printf("netgrip: scheduled self-update started (version %s)", version)
	go func() {
		for {
			scheduledSelfUpdateTick(version)
			time.Sleep(30 * time.Minute)
		}
	}()
}

func scheduledSelfUpdateTick(version string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("netgrip: scheduled self-update recovered: %v", r)
		}
	}()
	if version == "" || version == "dev" {
		return
	}
	cfg := ReadSelfUpdateConfig()
	if !cfg.Enabled || !cfg.inWindow(routerLocalHour()) {
		return
	}
	st := selfUpdateSchedState()
	now := time.Now().Unix()
	if st.LastCheck > 0 && now-st.LastCheck < int64(cfg.IntervalH)*3600 {
		return
	}
	if u := GetSelfUpdateStatus(); u.Phase == "downloading" || u.Phase == "installing" {
		return
	}
	// Jitter so a fleet of routers does not stampede GitHub on release day.
	jitter := time.Duration(rand.Intn(10*60)) * time.Second
	log.Printf("netgrip: scheduled self-update check in %s (window %02d-%02d)", jitter, cfg.WindowStart, cfg.WindowEnd)
	time.Sleep(jitter)

	check := CheckSelfUpdate(version)
	st.LastCheck = time.Now().Unix()
	switch {
	case check == nil || (!check.Available && check.Latest == ""):
		st.LastResult = "error: check failed"
	case check.Available:
		if err := StartSelfUpdate(version); err != nil {
			st.LastResult = "error: " + err.Error()
		} else {
			st.LastResult = "applied " + strings.TrimPrefix(check.Latest, "v")
			st.LastApply = time.Now().Unix()
			st.LastVersion = check.Latest
		}
	default:
		st.LastResult = "up-to-date"
	}
	selfUpdateSaveState(st)
	if strings.HasPrefix(st.LastResult, "applied") || strings.HasPrefix(st.LastResult, "error") {
		log.Printf("netgrip: scheduled self-update: %s", st.LastResult)
	}
}
