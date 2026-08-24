package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/owpanel/internal/executor"
)

const ddnsSection = "owpanel"

// DDNSConfig is the user-provided service configuration. Password is
// write-only: it is never returned by the probe.
type DDNSConfig struct {
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"service_name"`
	Domain      string `json:"domain"`
	LookupHost  string `json:"lookup_host"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
}

// DDNSProbe is the read-only DDNS state.
type DDNSProbe struct {
	Installed    bool   `json:"installed"`
	Active       bool   `json:"active"`
	Running      bool   `json:"running"`
	ServiceName  string `json:"service_name"`
	Domain       string `json:"domain"`
	LookupHost   string `json:"lookup_host"`
	Username     string `json:"username"`
	RegisteredIP string `json:"registered_ip"`
	LastUpdate   string `json:"last_update"`
}

func ddnsInstalled() bool {
	_, err := os.Stat("/etc/init.d/ddns")
	return err == nil
}

func ddnsUpdaterRunning() bool {
	out, err := exec.Command("pgrep", "-f", "dynamic_dns_updater.sh.*"+ddnsSection).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// ddnsStopUpdater kills our service's updater process (best effort) using
// the pid file ddns-scripts maintains. Per-service stop avoids the
// init.d reload that would restart every other configured service.
func ddnsStopUpdater() {
	data, err := os.ReadFile("/tmp/run/ddns/" + ddnsSection + ".pid")
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
}

// ProbeDDNS reads the DDNS state. Never includes the password.
func ProbeDDNS() *DDNSProbe {
	p := &DDNSProbe{Installed: ddnsInstalled()}
	base := "ddns." + ddnsSection
	if !uciSectionExists(base) {
		return p
	}
	p.ServiceName = uciGet(base + ".service_name")
	p.Domain = uciGet(base + ".domain")
	p.LookupHost = uciGet(base + ".lookup_host")
	p.Username = uciGet(base + ".username")
	p.Active = uciGet(base+".enabled") == "1"
	p.Running = ddnsUpdaterRunning()
	// ddns-scripts keeps per-service state in /tmp/run/ddns/<service>.dat
	if data, err := os.ReadFile("/tmp/run/ddns/" + ddnsSection + ".dat"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "registered_ip=") {
				p.RegisteredIP = strings.Trim(strings.TrimPrefix(line, "registered_ip="), "'\"")
			}
		}
		if st, err := os.Stat("/tmp/run/ddns/" + ddnsSection + ".dat"); err == nil {
			p.LastUpdate = st.ModTime().Format(time.RFC3339)
		}
	}
	return p
}

// SetDDNS applies the DDNS configuration and enabled state with snapshot,
// healthcheck and rollback.
func SetDDNS(cfg DDNSConfig) (*DDNSProbe, bool, error) {
	hadConfig := uciSectionExists("ddns." + ddnsSection)
	snapDdns := ""
	if _, err := os.Stat("/etc/config/ddns"); err == nil {
		if s, err := executor.Snapshot("ddns"); err == nil {
			snapDdns = s
		}
	}

	rollback := func() {
		if snapDdns != "" {
			_ = executor.Restore("ddns", snapDdns)
		} else {
			_ = executor.Run(executor.Op{Kind: "uci_delete", Args: []string{"ddns." + ddnsSection}})
			_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"ddns"}})
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"ddns", "reload"}})
	}

	ops, err := ddnsOps(cfg, hadConfig)
	if err != nil {
		return ProbeDDNS(), false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeDDNS(), true, err
	}
	if !cfg.Enabled {
		// init.d reload would restart other people's services; just kill
		// our own updater pid instead.
		ddnsStopUpdater()
	}

	ok := func() bool {
		for range 5 {
			probe := ProbeDDNS()
			if cfg.Enabled {
				if probe.Active && probe.Running {
					return true
				}
			} else if !probe.Active && !probe.Running {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	if !ok() {
		rollback()
		return ProbeDDNS(), true, fmt.Errorf("healthcheck failed after apply (enabled=%v), rolled back", cfg.Enabled)
	}
	return ProbeDDNS(), false, nil
}

func ddnsOps(cfg DDNSConfig, hadConfig bool) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	base := "ddns." + ddnsSection

	if !cfg.Enabled {
		if hadConfig {
			set(base+".enabled", "0")
			ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"ddns"}})
		}
		return ops, nil
	}

	if cfg.ServiceName == "" || cfg.Domain == "" {
		return nil, fmt.Errorf("service_name and domain are required")
	}
	if !ddnsInstalled() {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"ddns-scripts"}})
	}
	set(base, "service")
	set(base+".service_name", cfg.ServiceName)
	set(base+".domain", cfg.Domain)
	set(base+".username", cfg.Username)
	if cfg.Password != "" {
		set(base+".password", cfg.Password)
	}
	if cfg.LookupHost != "" {
		set(base+".lookup_host", cfg.LookupHost)
	}
	// Source of the public IP: the wan interface when the router has one,
	// else a web checkip service (dumb APs sit behind another router).
	if uciSectionExists("network.wan") {
		set(base+".ip_source", "network")
		set(base+".ip_network", "wan")
	} else {
		set(base+".ip_source", "web")
		set(base+".ip_url", "http://checkip.dyndns.org")
	}
	set(base+".enabled", "1")
	// The init script spawns one procd instance per enabled service.
	// `ddns start` only starts instances that are not running yet, so
	// other people's services are never restarted (rate-limit safe).
	// NOTE: dynamic_dns_updater.sh -- start runs in the FOREGROUND and
	// blocks the caller; never exec it directly (learned the hard way).
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"ddns"}},
		executor.Op{Kind: "initd", Args: []string{"ddns", "enable"}},
		executor.Op{Kind: "initd", Args: []string{"ddns", "start"}},
	)
	return ops, nil
}
