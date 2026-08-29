package modules

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type NlbwmonProbe struct {
	Installed        bool `json:"installed"`
	Running          bool `json:"running"`
	Generations      int  `json:"generations"`
	CommitInterval   int  `json:"commit_interval"`
	PreallocDays     int  `json:"prealloc_days"`
	ProtocolDatabase bool `json:"protocol_database"`
}

func ProbeNlbwmon() *NlbwmonProbe {
	p := &NlbwmonProbe{
		Installed: executor.ServiceEnabled("nlbwmon") || exec.Command("which", "nlbwmon").Run() == nil,
		Running:   executor.ServiceRunning("nlbwmon"),
	}
	if !p.Installed {
		return p
	}
	if v, err := strconv.Atoi(uciGet("nlbwmon.core.database_generations")); err == nil {
		p.Generations = v
	}
	if v, err := strconv.Atoi(uciGet("nlbwmon.core.commit_interval")); err == nil {
		p.CommitInterval = v
	}
	if v, err := strconv.Atoi(uciGet("nlbwmon.core.database_prealloc_days")); err == nil {
		p.PreallocDays = v
	}
	p.ProtocolDatabase = uciGet("nlbwmon.core.protocol_database") == "1"
	return p
}

type NlbwmonConfig struct {
	Enabled        *bool `json:"enabled,omitempty"`
	Generations    *int  `json:"generations,omitempty"`
	CommitInterval *int  `json:"commit_interval,omitempty"`
	PreallocDays   *int  `json:"prealloc_days,omitempty"`
}

func SetNlbwmon(cfg NlbwmonConfig) (*NlbwmonProbe, bool, error) {
	freshInstall := false
	if !ProbeNlbwmon().Installed {
		if cfg.Enabled == nil || !*cfg.Enabled {
			return nil, false, fmt.Errorf("nlbwmon is not installed")
		}
		if err := executor.Run(executor.Op{Kind: "pkg_add", Args: []string{"nlbwmon"}}); err != nil {
			return nil, false, fmt.Errorf("install nlbwmon: %w", err)
		}
		freshInstall = true
	}
	snap, err := executor.Snapshot("nlbwmon")
	if err != nil {
		return nil, false, fmt.Errorf("snapshot nlbwmon: %w", err)
	}

	var ops []executor.Op
	if cfg.Enabled != nil {
		action := "enable"
		if !*cfg.Enabled {
			action = "disable"
		}
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"nlbwmon", action}})
		if freshInstall && *cfg.Enabled {
			ops = append(ops, executor.Op{Kind: "initd", Args: []string{"nlbwmon", "start"}})
		}
	}
	if cfg.Generations != nil {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"nlbwmon.core.database_generations", strconv.Itoa(*cfg.Generations)}})
	}
	if cfg.CommitInterval != nil {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"nlbwmon.core.commit_interval", strconv.Itoa(*cfg.CommitInterval)}})
	}
	if cfg.PreallocDays != nil {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"nlbwmon.core.database_prealloc_days", strconv.Itoa(*cfg.PreallocDays)}})
	}

	if len(ops) > 0 {
		hasUciSet := false
		for _, op := range ops {
			if op.Kind == "uci_set" {
				hasUciSet = true
				break
			}
		}
		if hasUciSet {
			ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"nlbwmon"}})
			ops = append(ops, executor.Op{Kind: "initd", Args: []string{"nlbwmon", "restart"}})
		}
	}

	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("nlbwmon", snap)
		executor.Run(executor.Op{Kind: "initd", Args: []string{"nlbwmon", "restart"}})
		return ProbeNlbwmon(), true, err
	}
	return ProbeNlbwmon(), false, nil
}

func NlbwmonTopHosts(n int) string {
	out, err := exec.Command("nlbw", "-c", "show", "-g", "local_addr", "-o", "total_bytes", "-l", strconv.Itoa(n)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
