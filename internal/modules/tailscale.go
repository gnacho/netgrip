package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gnacho/owpanel/internal/executor"
)

// TSProbe is the read-only Tailscale state.
type TSProbe struct {
	Installed bool     `json:"installed"`
	Running   bool     `json:"running"`
	State     string   `json:"state"` // Running | NeedsLogin | NoState | Stopped
	AuthURL   string   `json:"auth_url,omitempty"`
	IPs       []string `json:"ips"`
}

type tsStatus struct {
	BackendState string `json:"BackendState"`
	AuthURL      string `json:"AuthURL"`
	Self         struct {
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"Self"`
}

func tsInstalled() bool {
	_, err := exec.LookPath("tailscale")
	return err == nil
}

func tsStatusNow() *tsStatus {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return nil
	}
	var st tsStatus
	if err := json.Unmarshal(out, &st); err != nil {
		return nil
	}
	return &st
}

// ProbeTailscale reads the Tailscale state.
func ProbeTailscale() *TSProbe {
	p := &TSProbe{Installed: tsInstalled(), IPs: []string{}}
	if !p.Installed {
		return p
	}
	p.Running = executor.ServiceRunning("tailscale")
	st := tsStatusNow()
	if st == nil {
		p.State = "NoState"
		return p
	}
	p.State = st.BackendState
	p.AuthURL = st.AuthURL
	if st.Self.TailscaleIPs != nil {
		p.IPs = st.Self.TailscaleIPs
	}
	// OpenWrt's tailscale status --json shows NeedsLogin but with an empty
	// AuthURL; the login URL only appears in the output of tailscale up.
	if p.State == "NeedsLogin" && p.AuthURL == "" {
		p.AuthURL = tsLoginURL()
	}
	return p
}

// tsLoginURL runs tailscale up with a bounded timeout and parses the
// authentication URL from its output (the command prints it and then
// blocks until authenticated, hence the timeout).
func tsLoginURL() string {
	out, _ := exec.Command("tailscale", "up", "--timeout=5s").CombinedOutput()
	for _, field := range strings.Fields(string(out)) {
		if strings.HasPrefix(field, "https://login.tailscale.com/") {
			return strings.TrimRight(field, ".,")
		}
	}
	return ""
}

// SetTailscale enables or disables Tailscale.
func SetTailscale(enable bool) (*TSProbe, bool, error) {
	if enable {
		return enableTailscale()
	}
	return disableTailscale()
}

func enableTailscale() (*TSProbe, bool, error) {
	var ops []executor.Op
	if !tsInstalled() {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"tailscale"}})
	}
	ops = append(ops,
		executor.Op{Kind: "initd", Args: []string{"tailscale", "enable"}},
		executor.Op{Kind: "initd", Args: []string{"tailscale", "start"}},
	)
	if err := executor.Apply(ops, nil); err != nil {
		return ProbeTailscale(), true, err
	}
	// Wait for tailscaled to answer, then bring the node up. tailscale up
	// blocks while unauthenticated, so it always runs with a timeout; the
	// AuthURL is then read from tailscale status --json.
	for range 10 {
		if tsStatusNow() != nil {
			break
		}
		time.Sleep(time.Second)
	}
	_ = exec.Command("tailscale", "up", "--timeout=5s").Run()
	probe := ProbeTailscale()
	if !probe.Running {
		return probe, true, fmt.Errorf("tailscaled is not running after enable, rolled back")
	}
	return probe, false, nil
}

func disableTailscale() (*TSProbe, bool, error) {
	ops := []executor.Op{
		{Kind: "initd", Args: []string{"tailscale", "stop"}},
		{Kind: "initd", Args: []string{"tailscale", "disable"}},
	}
	if err := executor.Apply(ops, nil); err != nil {
		return ProbeTailscale(), true, err
	}
	// procd stops the service asynchronously: give it a moment before
	// declaring failure.
	for range 5 {
		probe := ProbeTailscale()
		if !probe.Running {
			return probe, false, nil
		}
		time.Sleep(time.Second)
	}
	return ProbeTailscale(), true, fmt.Errorf("tailscaled is still running after disable, rolled back")
}
