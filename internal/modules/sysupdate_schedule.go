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

// Actualización del sistema programada (#640): el firmware (owut/ASU) se
// comprueba y se aplica solo dentro de una ventana, igual que el self-update
// de NetGrip pero con dos diferencias que mandan:
//
//   - Un upgrade de firmware REINICIA el router. Por eso la ventana por
//     defecto es de madrugada y el intervalo semanal.
//   - Nunca se aplica a ciegas: si owut dice que hay paquetes que se
//     perderían y no hay forma de conservarlos (safe_with_reinstall), se
//     salta y se deja anotado el motivo. NetGrip sobrevive porque sus
//     ficheros van en /etc/sysupgrade.conf (EnsureSurvival).
//
// La configuración vive en UCI (netgrip.sysupdate.*) y el estado en
// /etc/netgrip/sysupdate-state.json.

const sysUpdateStatePath = "/etc/netgrip/sysupdate-state.json"

// SysUpdateConfig is the user-facing schedule for the firmware update.
type SysUpdateConfig struct {
	Enabled     bool `json:"enabled"`
	IntervalH   int  `json:"intervalHours"`
	WindowStart int  `json:"windowStart"`
	WindowEnd   int  `json:"windowEnd"`
}

// SysUpdateState is what the scheduler remembers between runs.
type SysUpdateState struct {
	LastCheck   int64  `json:"lastCheck"`
	LastResult  string `json:"lastResult"` // up-to-date | applied <ver> | blocked: ... | error: ...
	LastApply   int64  `json:"lastApply"`
	LastVersion string `json:"lastVersion"`
}

// SysUpdateSchedule is the API view.
type SysUpdateSchedule struct {
	Config SysUpdateConfig `json:"config"`
	State  SysUpdateState  `json:"state"`
}

func (c SysUpdateConfig) normalized() SysUpdateConfig {
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

func (c SysUpdateConfig) inWindow(hour int) bool {
	if c.WindowStart == c.WindowEnd {
		return true
	}
	if c.WindowStart < c.WindowEnd {
		return hour >= c.WindowStart && hour < c.WindowEnd
	}
	return hour >= c.WindowStart || hour < c.WindowEnd
}

func sysUpdateUCIGet(opt string) string {
	out, err := exec.Command("uci", "-q", "get", "netgrip.sysupdate."+opt).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ReadSysUpdateConfig loads the schedule with safe defaults: disabled until
// the user asks for it, weekly, 03:00-05:00 local.
func ReadSysUpdateConfig() SysUpdateConfig {
	c := SysUpdateConfig{Enabled: false, IntervalH: 168, WindowStart: 3, WindowEnd: 5}
	if v := sysUpdateUCIGet("enabled"); v != "" {
		c.Enabled = v == "1"
	}
	if n, err := strconv.Atoi(sysUpdateUCIGet("interval")); err == nil && n > 0 {
		c.IntervalH = n
	}
	if n, err := strconv.Atoi(sysUpdateUCIGet("window_start")); err == nil {
		c.WindowStart = n
	}
	if n, err := strconv.Atoi(sysUpdateUCIGet("window_end")); err == nil {
		c.WindowEnd = n
	}
	return c.normalized()
}

// WriteSysUpdateConfig persists the schedule in UCI.
func WriteSysUpdateConfig(c SysUpdateConfig) (SysUpdateConfig, error) {
	c = c.normalized()
	sets := []string{
		"netgrip.sysupdate=sysupdate",
		"netgrip.sysupdate.enabled=" + map[bool]string{true: "1", false: "0"}[c.Enabled],
		"netgrip.sysupdate.interval=" + strconv.Itoa(c.IntervalH),
		"netgrip.sysupdate.window_start=" + strconv.Itoa(c.WindowStart),
		"netgrip.sysupdate.window_end=" + strconv.Itoa(c.WindowEnd),
	}
	for _, s := range sets {
		if out, err := exec.Command("uci", "set", s).CombinedOutput(); err != nil {
			return c, errors.New("uci set " + s + ": " + strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("uci", "commit", "netgrip").CombinedOutput(); err != nil {
		return c, errors.New("uci commit netgrip: " + strings.TrimSpace(string(out)))
	}
	return c, nil
}

var sysUpdateMu sync.Mutex

func sysUpdateSchedState() SysUpdateState {
	sysUpdateMu.Lock()
	defer sysUpdateMu.Unlock()
	var st SysUpdateState
	if data, err := os.ReadFile(sysUpdateStatePath); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

// saveSysUpdateState writes the scheduler state atomically-ish (small file,
// tmpfs or overlay; a partial write is only a lost "last check").
func saveSysUpdateState(st SysUpdateState) {
	sysUpdateMu.Lock()
	defer sysUpdateMu.Unlock()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	tmp := sysUpdateStatePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, sysUpdateStatePath)
}

// sysUpdateAction decides what to do with a check result. A firmware upgrade
// reboots the router, so the scheduler only applies it when owut says it is
// safe, or when the only blocker is packages that can be preserved with -r
// (netgrip among them). Anything else is reported, never forced.
func sysUpdateAction(c *UpdateCheck) (action string, withReinstall bool) {
	if c == nil {
		return "error", false
	}
	if !c.Available || c.SameVersion {
		return "up-to-date", false
	}
	if c.SafeToProceed {
		return "apply", false
	}
	if c.SafeWithReinstall {
		return "apply", true
	}
	return "blocked", false
}

// SysUpdateScheduleOf returns config + state for the API.
func SysUpdateScheduleOf() SysUpdateSchedule {
	return SysUpdateSchedule{Config: ReadSysUpdateConfig(), State: sysUpdateSchedState()}
}

// StartSysUpdateScheduler runs the loop: every 20 minutes it checks whether
// the window is open and the interval elapsed, then checks for a firmware
// update and applies it when it is safe.
func StartSysUpdateScheduler() {
	log.Printf("netgrip: scheduled system update started")
	go func() {
		for {
			scheduledSysUpdateTick()
			time.Sleep(20 * time.Minute)
		}
	}()
}

func scheduledSysUpdateTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("netgrip: scheduled system update recovered: %v", r)
		}
	}()
	cfg := ReadSysUpdateConfig()
	if !cfg.Enabled || !cfg.inWindow(routerLocalHour()) {
		return
	}
	st := sysUpdateSchedState()
	now := time.Now().Unix()
	if st.LastCheck > 0 && now-st.LastCheck < int64(cfg.IntervalH)*3600 {
		return
	}

	// Jitter para que una flota no pida imagen a la vez.
	time.Sleep(time.Duration(rand.Intn(8*60)) * time.Second)

	check := CheckUpdate()
	st.LastCheck = time.Now().Unix()
	action, withReinstall := sysUpdateAction(check)
	switch action {
	case "error":
		st.LastResult = "error: check failed"
	case "up-to-date":
		st.LastResult = "up-to-date"
	case "blocked":
		// Faltan paquetes que no se pueden conservar: un upgrade dejaría el
		// router sin algo que el usuario instaló. Se avisa y no se toca.
		st.LastResult = "blocked: missing packages"
	case "apply":
		// withReinstall solo cuando hace falta conservar paquetes propios
		// (netgrip entre ellos): EnsureSurvival deja sus ficheros en
		// /etc/sysupgrade.conf, así que el panel vuelve tras el reinicio.
		if err := StartUpgrade(withReinstall); err != nil {
			st.LastResult = "error: " + err.Error()
		} else {
			st.LastResult = "applied " + strings.TrimPrefix(check.VersionTo, "v")
			st.LastApply = time.Now().Unix()
			st.LastVersion = check.VersionTo
			log.Printf("netgrip: scheduled system update to %s started (reboot follows)", check.VersionTo)
		}
	}
	saveSysUpdateState(st)
}
