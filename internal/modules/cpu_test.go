package modules

import (
	"testing"
	"time"
)

func TestReadSoftnetDecodesHexColumns(t *testing.T) {
	// The real shape: one hex row per core, column 2 dropped, column 3
	// squeezed. A core that is fine reads zero; one that cannot keep up
	// shows millions.
	const doc = "0c8c75c8 00000000 000000c9 00000000\n" +
		"2a2016ec 003135bf 000757ac 00000000\n"
	rows := parseSoftnet(doc)
	if len(rows) != 2 {
		t.Fatalf("rows: %+v", rows)
	}
	if rows[0].dropped != 0 || rows[0].squeezed != 0xc9 {
		t.Errorf("cpu0: %+v", rows[0])
	}
	if rows[1].dropped != 0x3135bf || rows[1].squeezed != 0x757ac {
		t.Errorf("cpu1: %+v", rows[1])
	}
}

func TestParseCPUTimesSkipsTheAggregateLine(t *testing.T) {
	const doc = `cpu  100 0 100 800 0 0 0 0 0 0
cpu0 10 0 10 80 0 0 0 0 0 0
cpu1 40 0 40 20 0 0 0 0 0 0
intr 12345
`
	cores := parseCPUTimes(doc)
	if len(cores) != 2 {
		t.Fatalf("cores: %+v", cores)
	}
	// cpu0: total 100, idle 80 -> 20% busy once compared against zero.
	if cores[0].total != 100 || cores[0].idle != 80 {
		t.Errorf("cpu0: %+v", cores[0])
	}
	// idle counts iowait too: both mean the core is not working.
	if cores[1].total != 100 || cores[1].idle != 20 {
		t.Errorf("cpu1: %+v", cores[1])
	}
}

// The first sample has nothing to compare against. Reporting a percentage
// from boot-time totals would describe the average since boot, which is not
// what a live card is claiming to show.
func TestProbeCPUWarmsUpBeforeReporting(t *testing.T) {
	cpuState.mu.Lock()
	cpuState.at, cpuState.cores, cpuState.softnet, cpuState.procs = timeZero(), nil, nil, nil
	cpuState.mu.Unlock()

	first := ProbeCPU()
	if !first.Warming {
		t.Fatal("the first poll must report warming")
	}
	if len(first.Cores) != 0 || first.UsagePct != 0 {
		t.Fatalf("no percentages on the first poll: %+v", first)
	}
	// Load and governor do not need two samples, so they come through.
	if len(first.Load) == 0 {
		t.Error("load average should be there from the first poll")
	}

	second := ProbeCPU()
	if second.Warming {
		t.Fatal("the second poll has a delta to work with")
	}
	if len(second.Cores) == 0 {
		t.Fatal("expected per-core figures")
	}
	for _, c := range second.Cores {
		if c.UsagePct < 0 || c.UsagePct > 100 {
			t.Errorf("core %d out of range: %v", c.Idx, c.UsagePct)
		}
	}
	if second.Busiest < second.UsagePct {
		t.Errorf("the busiest core cannot be below the average: %+v", second)
	}
}

func TestParseProcStatHandlesCommandWithSpacesAndRSS(t *testing.T) {
	// Fields 14/15 (utime/stime) carry the load; field 24 is RSS in pages.
	// The command may contain spaces and parentheses of its own.
	const line = "6390 (netgrip -listen 0.0.0.0) S 1 6390 6390 0 -1 4194304 12345 0 0 0 150 75 0 0 20 0 12 0 42 0 14080 24064 0 0 0 0 0 0"
	name, ticks, rss, ok := parseProcStat(line, 4096)
	if !ok {
		t.Fatal("should parse")
	}
	if name != "netgrip -listen 0.0.0.0" {
		t.Errorf("name: %q", name)
	}
	if ticks != 225 { // utime 150 + stime 75
		t.Errorf("ticks: %v", ticks)
	}
	if rss != 14080*4096 {
		t.Errorf("rss: %v", rss)
	}

	if _, _, _, ok := parseProcStat("garbage", 4096); ok {
		t.Error("garbage must not parse")
	}
}

func timeZero() time.Time { return time.Time{} }
