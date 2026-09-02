package modules

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// NetifydProbe is the read-only state of the netifyd DPI engine.
type NetifydProbe struct {
	Installed bool           `json:"installed"`
	Enabled   bool           `json:"enabled"`
	Running   bool           `json:"running"`
	Applicable bool          `json:"applicable"`
	LowEnd    bool           `json:"low_end"`
	Apps      []NetifydApp   `json:"apps"`
}

var (
	netifydMu     sync.Mutex
	netifydClient *netifydSocketClient
	netifydTable  = newNetifydTable(defaultMaxApps, defaultMaxFlows)

	// Injected for tests.
	netifydInstalledFn = netifydInstalled
	netifydEnabledFn   = netifydEnabled
	netifydRunningFn   = netifydRunning
	lowEndDeviceFn     = lowEndDevice
	socketExistsFn     = socketExists
)

func netifydInstalled() bool {
	return pkgInstalled("netifyd")
}

func netifydEnabled() bool {
	return executor.ServiceEnabled("netifyd")
}

func netifydRunning() bool {
	return executor.ServiceRunning("netifyd")
}

// lowEndDevice reports routers that should not run netifyd because of RAM or CPU.
func lowEndDevice() bool {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		// Totalram is bytes; 128 MiB threshold.
		if si.Totalram/1024/1024 < 128 {
			return true
		}
	}
	return runtime.NumCPU() <= 1
}

// ProbeNetifyd returns the current netifyd state and the live app table.
func ProbeNetifyd() *NetifydProbe {
	p := &NetifydProbe{
		Installed:  netifydInstalledFn(),
		Enabled:    netifydEnabledFn(),
		Running:    netifydRunningFn(),
		LowEnd:     lowEndDeviceFn(),
		Apps:       netifydTable.Apps(),
	}
	p.Applicable = p.Installed || !p.LowEnd
	return p
}

// NetifydApps returns just the live application table (for /api/dpi/apps).
func NetifydApps() []NetifydApp {
	return netifydTable.Apps()
}

// NetifydTimeline returns the aggregated timeline for /api/dpi/timeline.
func NetifydTimeline() NetifydTimelineResponse {
	return netifydTable.Timeline()
}

// StartNetifydClient starts the persistent socket client if it is not running.
// It is idempotent.
func StartNetifydClient() {
	netifydMu.Lock()
	defer netifydMu.Unlock()
	if netifydClient != nil {
		return
	}
	c := newNetifydSocketClient(netifydSocketPath, netifydTable)
	c.Start()
	netifydClient = c
}

// StopNetifydClient stops the socket client and clears the live table.
func StopNetifydClient() {
	netifydMu.Lock()
	defer netifydMu.Unlock()
	if netifydClient == nil {
		return
	}
	netifydClient.Stop()
	netifydClient = nil
	netifydTable.Reset()
}

// SetNetifyd enables or disables netifyd with healthcheck and rollback.
func SetNetifyd(enabled bool) (*NetifydProbe, bool, error) {
	probe := ProbeNetifyd()
	if enabled && probe.LowEnd {
		return probe, false, fmt.Errorf("netifyd is disabled on devices with <128 MB RAM or single-core CPUs")
	}
	if !probe.Installed && !enabled {
		return probe, false, nil
	}

	var ops []executor.Op
	if !probe.Installed && enabled {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"netifyd"}})
	}

	action := "stop"
	enable := "disable"
	if enabled {
		action = "start"
		enable = "enable"
	}
	ops = append(ops,
		executor.Op{Kind: "initd", Args: []string{"netifyd", enable}},
		executor.Op{Kind: "initd", Args: []string{"netifyd", action}},
	)

	if err := executor.Apply(ops, nil); err != nil {
		rollbackNetifyd(probe.Installed, enabled)
		return ProbeNetifyd(), true, err
	}

	if enabled {
		StartNetifydClient()
	} else {
		StopNetifydClient()
	}

	if !waitNetifydState(enabled) {
		rollbackNetifyd(probe.Installed, enabled)
		return ProbeNetifyd(), true, fmt.Errorf("netifyd healthcheck failed after apply (enabled=%v), rolled back", enabled)
	}

	return ProbeNetifyd(), false, nil
}

func rollbackNetifyd(wasInstalled, wantedEnabled bool) {
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"netifyd", "stop"}})
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"netifyd", "disable"}})
	if wantedEnabled && !wasInstalled {
		// We installed it during this call; remove it to leave the router clean.
		_ = executor.Run(executor.Op{Kind: "pkg_del", Args: []string{"netifyd"}})
	}
	StopNetifydClient()
}

func waitNetifydState(enabled bool) bool {
	for range 5 {
		p := ProbeNetifyd()
		if enabled {
			if p.Running && socketExistsFn() {
				return true
			}
		} else if !p.Running {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func socketExists() bool {
	_, err := os.Stat(netifydSocketPath)
	return err == nil
}

// netifydContext is used by tests to inject fake dependencies.
type netifydContext struct {
	installed    func() bool
	enabled      func() bool
	running      func() bool
	lowEnd       func() bool
	socketExists func() bool
}

func defaultNetifydContext() *netifydContext {
	return &netifydContext{
		installed:    netifydInstalled,
		enabled:      netifydEnabled,
		running:      netifydRunning,
		lowEnd:       lowEndDevice,
		socketExists: socketExists,
	}
}

func init() {
	// If netifyd is already running when NetGrip starts, connect immediately.
	if netifydRunningFn() {
		StartNetifydClient()
	}
}
