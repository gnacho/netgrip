package modules

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// PoE watchdog (issue #68): power-cycles a PoE port when the device behind
// it stops answering pings.

type PoEWatchdogConfig struct {
	Port      string `json:"port"`
	Enabled   bool   `json:"enabled"`
	Target    string `json:"target"`     // IP or hostname to ping
	Threshold int    `json:"threshold"`  // consecutive failures before a cycle
	IntervalS int    `json:"interval_s"` // seconds between checks
	CooldownS int    `json:"cooldown_s"` // seconds to wait after a cycle
}

type PoEWatchdogState struct {
	Config    PoEWatchdogConfig `json:"config"`
	Failures  int               `json:"failures"`
	LastCheck string            `json:"last_check,omitempty"`
	LastCycle string            `json:"last_cycle,omitempty"`
	Cooling   bool              `json:"cooling"`
}

type poeWatchdogRuntime struct {
	failures  int
	lastCheck time.Time
	lastCycle time.Time
}

var (
	poeWdMu      sync.Mutex
	poeWdRunning map[string]*poeWatchdogRuntime
)

var poeWdTargetRe = regexp.MustCompile(`^[a-zA-Z0-9._:-]{1,253}$`)

func poeWdDefaults(cfg *PoEWatchdogConfig) {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.IntervalS <= 0 {
		cfg.IntervalS = 30
	}
	if cfg.CooldownS <= 0 {
		cfg.CooldownS = 120
	}
}

func poeWdRuntimeMap() map[string]*poeWatchdogRuntime {
	if poeWdRunning == nil {
		poeWdRunning = map[string]*poeWatchdogRuntime{}
	}
	return poeWdRunning
}

func poeWdUCIKey(port string) string {
	return "netgrip.poe." + sanitizeUCIKey(port)
}

// watchdogTick is the decision core, kept free of hardware so it can be unit
// tested. It receives a snapshot of the runtime state and returns the action
// taken plus the updated state. cycle runs while no locks are held.
// Actions: "" (not due / cooling), "ok", "fail", "cycle".
func watchdogTick(cfg PoEWatchdogConfig, rt poeWatchdogRuntime, now time.Time, ping func(target string) bool, cycle func(port string) error) (string, poeWatchdogRuntime) {
	poeWdDefaults(&cfg)
	interval := time.Duration(cfg.IntervalS) * time.Second
	cooldown := time.Duration(cfg.CooldownS) * time.Second
	if !rt.lastCycle.IsZero() && now.Sub(rt.lastCycle) < cooldown {
		return "", rt // cooling down after a previous cycle
	}
	if !rt.lastCheck.IsZero() && now.Sub(rt.lastCheck) < interval {
		return "", rt // not due yet
	}
	rt.lastCheck = now
	if ping(cfg.Target) {
		rt.failures = 0
		return "ok", rt
	}
	rt.failures++
	if rt.failures >= cfg.Threshold {
		_ = cycle(cfg.Port)
		rt.failures = 0
		rt.lastCycle = now
		return "cycle", rt
	}
	return "fail", rt
}

func poePing(target string) bool {
	return exec.Command("ping", "-c", "1", "-W", "2", target).Run() == nil
}

// poeSetPort enables or disables PoE on a port using the same backends as
// the PoE schedule (sysfs first, poe-util as fallback). The port name is
// validated by the caller; no shell metacharacters reach this string.
func poeSetPort(port string, on bool) error {
	action := "disable"
	if on {
		action = "enable"
	}
	cmd := exec.Command("sh", "-c", "echo "+action+" > /sys/class/net/"+port+"/device/of_node/poe/status 2>/dev/null || poe-util "+action+" "+port)
	if out, err := cmd.CombinedOutput(); err != nil {
		return &poeCycleError{port: port, out: string(out), err: err}
	}
	return nil
}

type poeCycleError struct {
	port string
	out  string
	err  error
}

func (e *poeCycleError) Error() string { return "poe " + e.port + ": " + e.err.Error() + " (" + e.out + ")" }

func poeCycle(port string) error {
	if err := poeSetPort(port, false); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	return poeSetPort(port, true)
}

// ProbePoEWatchdogs returns the configured watchdogs with their live state.
func ProbePoEWatchdogs() []PoEWatchdogState {
	poeWdMu.Lock()
	defer poeWdMu.Unlock()
	probe := ProbePoE()
	out := make([]PoEWatchdogState, 0)
	for _, p := range probe.Ports {
		key := poeWdUCIKey(p.Name)
		if uciGet(key+".watchdog_enabled") != "1" {
			continue
		}
		cfg := PoEWatchdogConfig{
			Port:      p.Name,
			Enabled:   true,
			Target:    uciGet(key + ".watchdog_target"),
			Threshold: atoiDefault(uciGet(key+".watchdog_threshold"), 5),
			IntervalS: atoiDefault(uciGet(key+".watchdog_interval"), 30),
			CooldownS: atoiDefault(uciGet(key+".watchdog_cooldown"), 120),
		}
		poeWdDefaults(&cfg)
		st := PoEWatchdogState{Config: cfg}
		if rt := poeWdRuntimeMap()[p.Name]; rt != nil {
			st.Failures = rt.failures
			st.LastCheck = tsOrEmpty(rt.lastCheck)
			st.LastCycle = tsOrEmpty(rt.lastCycle)
			st.Cooling = !rt.lastCycle.IsZero() && time.Since(rt.lastCycle) < time.Duration(cfg.CooldownS)*time.Second
		}
		out = append(out, st)
	}
	return out
}

func tsOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}

type poeValidationError struct{ msg string }

func (e *poeValidationError) Error() string { return e.msg }

// SetPoEWatchdog persists (and enables/disables) the watchdog for one port.
func SetPoEWatchdog(cfg PoEWatchdogConfig) ([]PoEWatchdogState, error) {
	if cfg.Port == "" {
		return nil, &poeValidationError{"port required"}
	}
	if cfg.Enabled && !poeWdTargetRe.MatchString(cfg.Target) {
		return nil, &poeValidationError{"ping target must be an IP or hostname"}
	}
	poeWdDefaults(&cfg)

	ensurePoESection(cfg.Port)
	key := poeWdUCIKey(cfg.Port)
	val := "0"
	if cfg.Enabled {
		val = "1"
	}
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{key + ".watchdog_enabled", val}},
	}
	if cfg.Enabled {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{key + ".watchdog_target", cfg.Target}},
			executor.Op{Kind: "uci_set", Args: []string{key + ".watchdog_threshold", strconv.Itoa(cfg.Threshold)}},
			executor.Op{Kind: "uci_set", Args: []string{key + ".watchdog_interval", strconv.Itoa(cfg.IntervalS)}},
			executor.Op{Kind: "uci_set", Args: []string{key + ".watchdog_cooldown", strconv.Itoa(cfg.CooldownS)}},
		)
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"netgrip"}})
	if err := executor.Apply(ops, nil); err != nil {
		return ProbePoEWatchdogs(), err
	}

	poeWdMu.Lock()
	if cfg.Enabled {
		poeWdRuntimeMap()[cfg.Port] = &poeWatchdogRuntime{}
	} else {
		delete(poeWdRuntimeMap(), cfg.Port)
	}
	poeWdMu.Unlock()
	return ProbePoEWatchdogs(), nil
}

// ensurePoESection creates the netgrip.poe and per-port sections when they do
// not exist yet (same approach as SetPoESchedule).
func ensurePoESection(port string) {
	if !uciSectionExists("netgrip.poe") {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config poe 'poe'\n")
		_ = cmd.Run()
	}
	secName := sanitizeUCIKey(port)
	if !uciSectionExists("netgrip.poe." + secName) {
		cmd := exec.Command("uci", "import", "netgrip")
		cmd.Stdin = strings.NewReader("config poeport '" + secName + "'\n")
		_ = cmd.Run()
	}
}

// StartPoEWatchdog launches the background watchdog loop. Per-port scheduling
// and cooldowns live inside the tick, so one sweep every 5s is enough.
func StartPoEWatchdog() {
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for now := range t.C {
			poeWatchdogSweep(now)
		}
	}()
}

func poeWatchdogSweep(now time.Time) {
	for _, st := range ProbePoEWatchdogs() {
		cfg := st.Config
		poeWdMu.Lock()
		rt := poeWatchdogRuntime{}
		if cur := poeWdRuntimeMap()[cfg.Port]; cur != nil {
			rt = *cur
		}
		poeWdMu.Unlock()

		action, newRt := watchdogTick(cfg, rt, now, poePing, poeCycle)

		poeWdMu.Lock()
		if cur := poeWdRuntimeMap()[cfg.Port]; cur != nil || action != "" {
			poeWdRuntimeMap()[cfg.Port] = &newRt
		}
		poeWdMu.Unlock()
	}
}
