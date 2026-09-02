package modules

import (
	"fmt"
	"os"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// MDNSProbe is the read-only mDNS reflector state.
type MDNSProbe struct {
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	Running   bool   `json:"running"`
	Domain    string `json:"domain"` // hostname.local
}

func mdnsInstalled() bool {
	_, err := os.Stat("/etc/init.d/umdns")
	return err == nil
}

func mdnsHostname() string {
	h := uciGet("system.@system[0].hostname")
	if h == "" {
		h = "openwrt"
	}
	return h + ".local"
}

// ProbeMDNS reads the mDNS reflector state.
func ProbeMDNS() *MDNSProbe {
	p := &MDNSProbe{Installed: mdnsInstalled()}
	if !p.Installed {
		return p
	}
	p.Enabled = executor.ServiceEnabled("umdns")
	p.Running = executor.ServiceRunning("umdns")
	p.Domain = mdnsHostname()
	return p
}

// SetMDNS applies the mDNS reflector enable/disable with snapshot, reload and
// rollback.
func SetMDNS(enabled bool) (*MDNSProbe, bool, error) {
	probe := ProbeMDNS()
	if !probe.Installed && !enabled {
		return probe, false, nil
	}

	snap := ""
	if _, err := os.Stat("/etc/config/umdns"); err == nil {
		if s, err := executor.Snapshot("umdns"); err == nil {
			snap = s
		}
	}

	rollback := func() {
		if snap != "" {
			_ = executor.Restore("umdns", snap)
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"umdns", "restart"}})
	}

	var ops []executor.Op
	if !probe.Installed {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"umdns"}})
	}
	action := "stop"
	enable := "disable"
	if enabled {
		action = "start"
		enable = "enable"
	}
	ops = append(ops,
		executor.Op{Kind: "initd", Args: []string{"umdns", enable}},
		executor.Op{Kind: "initd", Args: []string{"umdns", action}},
	)

	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeMDNS(), true, err
	}

	ok := func() bool {
		for range 5 {
			p := ProbeMDNS()
			if enabled {
				if p.Enabled && p.Running {
					return true
				}
			} else if !p.Enabled && !p.Running {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	if !ok() {
		rollback()
		return ProbeMDNS(), true, fmt.Errorf("mDNS healthcheck failed after apply (enabled=%v), rolled back", enabled)
	}
	return ProbeMDNS(), false, nil
}
