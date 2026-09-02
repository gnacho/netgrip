package modules

import (
	"testing"
	"time"
)

func wdCfg() PoEWatchdogConfig {
	return PoEWatchdogConfig{
		Port:      "lan2",
		Enabled:   true,
		Target:    "192.0.2.10",
		Threshold: 3,
		IntervalS: 30,
		CooldownS: 120,
	}
}

func TestWatchdogTickFirstCheckPings(t *testing.T) {
	cfg := wdCfg()
	pinged := false
	action, rt := watchdogTick(cfg, poeWatchdogRuntime{}, time.Now(),
		func(target string) bool { pinged = true; return true }, nil)
	if !pinged {
		t.Fatal("first tick must ping")
	}
	if action != "ok" {
		t.Fatalf("want ok, got %q", action)
	}
	if !rt.lastCheck.IsZero() == false && rt.failures != 0 {
		t.Fatalf("unexpected state %+v", rt)
	}
}

func TestWatchdogTickNotDueYet(t *testing.T) {
	cfg := wdCfg()
	now := time.Now()
	rt := poeWatchdogRuntime{lastCheck: now}
	action, _ := watchdogTick(cfg, rt, now.Add(10*time.Second), nil, nil)
	if action != "" {
		t.Fatalf("second check before interval must be skipped, got %q", action)
	}
}

func TestWatchdogTickCyclesAfterThreshold(t *testing.T) {
	cfg := wdCfg()
	now := time.Now()
	rt := poeWatchdogRuntime{}
	cycles := 0
	fail := func(string) bool { return false }
	cycle := func(port string) error { cycles++; return nil }

	var action string
	for i := 1; i <= 3; i++ {
		action, rt = watchdogTick(cfg, rt, now.Add(time.Duration(i)*time.Minute), fail, cycle)
	}
	if action != "cycle" || cycles != 1 {
		t.Fatalf("want one cycle after 3 failures, got action=%q cycles=%d", action, cycles)
	}
	if rt.failures != 0 || rt.lastCycle.IsZero() {
		t.Fatalf("state must reset after cycle, got %+v", rt)
	}
}

func TestWatchdogTickCooldownBlocksChecks(t *testing.T) {
	cfg := wdCfg()
	now := time.Now()
	rt := poeWatchdogRuntime{lastCycle: now}
	pinged := false
	action, _ := watchdogTick(cfg, rt, now.Add(60*time.Second),
		func(string) bool { pinged = true; return true }, nil)
	if action != "" || pinged {
		t.Fatalf("cooldown must block checks, got action=%q pinged=%v", action, pinged)
	}
}

func TestWatchdogTickSuccessResetsFailures(t *testing.T) {
	cfg := wdCfg()
	now := time.Now()
	rt := poeWatchdogRuntime{failures: 2}
	action, rt := watchdogTick(cfg, rt, now.Add(time.Minute),
		func(string) bool { return true }, nil)
	if action != "ok" || rt.failures != 0 {
		t.Fatalf("success must reset failures, got action=%q failures=%d", action, rt.failures)
	}
}

func TestWatchdogTickDefaultsApplied(t *testing.T) {
	cfg := PoEWatchdogConfig{Port: "lan1", Enabled: true, Target: "10.0.0.5"}
	poeWdDefaults(&cfg)
	if cfg.Threshold != 5 || cfg.IntervalS != 30 || cfg.CooldownS != 120 {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}
