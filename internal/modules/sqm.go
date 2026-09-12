package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

const sqmSection = "netgrip"

// SQMConfig is the user-provided queue configuration. Rates in kbit/s.
// Profile is one of balanced|gaming|streaming|custom ("" defaults to balanced).
type SQMConfig struct {
	Enabled  bool   `json:"enabled"`
	Download string `json:"download"`
	Upload   string `json:"upload"`
	Profile  string `json:"profile"`
}

// SQMProbe is the read-only SQM state.
type SQMProbe struct {
	Installed bool   `json:"installed"`
	HasWan    bool   `json:"has_wan"`
	Active    bool   `json:"active"`
	Running   bool   `json:"running"`
	Interface string `json:"interface"`
	Download  string `json:"download"`
	Upload    string `json:"upload"`
	Profile   string `json:"profile"`
	Qdisc     string `json:"qdisc"`
	Script    string `json:"script"`
}

// sqmProfileKeys are the CAKE options a profile is allowed to manage. They map
// to the sqm-scripts UCI options eqdisc_opts / iqdisc_opts (raw CAKE keywords
// appended to the egress/ingress `tc ... cake` commands).
var sqmProfileKeys = []string{"eqdisc_opts", "iqdisc_opts"}

// profileSqmOpts returns the eqdisc_opts/iqdisc_opts values a profile applies.
// Semantics:
//   - balanced: both keys present with empty value -> delete both (restore the
//     script defaults, i.e. exactly today's behavior).
//   - gaming/streaming: both keys present with the profile's CAKE options.
//   - custom: nil -> leave eqdisc_opts/iqdisc_opts untouched so raw CAKE
//     settings stay in LuCI. Note: switching gaming/streaming -> custom leaves
//     the previous profile's options in place until edited in LuCI (custom
//     deliberately never overwrites them).
func profileSqmOpts(profile string) map[string]string {
	switch profile {
	case "gaming":
		return map[string]string{"eqdisc_opts": "diffserv4", "iqdisc_opts": "diffserv4"}
	case "streaming":
		return map[string]string{"eqdisc_opts": "besteffort", "iqdisc_opts": "besteffort"}
	case "custom":
		return nil
	default: // "balanced" (and unknown, defensively): delete both.
		return map[string]string{"eqdisc_opts": "", "iqdisc_opts": ""}
	}
}

// validSqmProfile reports whether p is a profile NetGrip manages.
func validSqmProfile(p string) bool {
	switch p {
	case "balanced", "gaming", "streaming", "custom":
		return true
	}
	return false
}

// normalizeProfile maps a stored profile value to a canonical profile. Empty
// and unknown values fall back to "balanced" (today's behavior).
func normalizeProfile(v string) string {
	if validSqmProfile(v) {
		return v
	}
	return "balanced"
}

// applyProfileOpts appends the profile's eqdisc_opts/iqdisc_opts operations:
// set when the profile defines a value, delete when it defines an empty value,
// and nothing for custom (nil map).
func applyProfileOpts(ops []executor.Op, base, profile string) []executor.Op {
	opts := profileSqmOpts(profile)
	if opts == nil {
		return ops // custom: leave raw CAKE options untouched
	}
	for _, key := range sqmProfileKeys {
		val, ok := opts[key]
		if !ok {
			continue
		}
		full := base + "." + key
		if val == "" {
			if uciSectionExists(full) {
				ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{full}})
			}
		} else {
			ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{full, val}})
		}
	}
	return ops
}

func sqmInstalled() bool {
	_, err := os.Stat("/etc/init.d/sqm")
	return err == nil
}

// wanDevice resolves the L3 device of the wan interface ("" on dumb APs).
func wanDevice() string {
	out, err := exec.Command("sh", "-c", "ubus call network.interface.wan status 2>/dev/null | grep l3_device | cut -d'\"' -f4").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sqmCakePresent(dev string) bool {
	if dev == "" {
		return false
	}
	out, err := exec.Command("tc", "qdisc", "show", "dev", dev).Output()
	return err == nil && strings.Contains(string(out), "cake")
}

// ProbeSQM reads the SQM state.
func ProbeSQM() *SQMProbe {
	p := &SQMProbe{Installed: sqmInstalled()}
	dev := wanDevice()
	p.HasWan = dev != ""
	base := "sqm." + sqmSection
	if !uciSectionExists(base) {
		return p
	}
	p.Interface = uciGet(base + ".interface")
	p.Download = uciGet(base + ".download")
	p.Upload = uciGet(base + ".upload")
	p.Profile = normalizeProfile(uciGet(base + ".profile"))
	p.Qdisc = uciGet(base + ".qdisc")
	p.Script = uciGet(base + ".script")
	p.Active = uciGet(base+".enabled") == "1"
	if p.Interface != "" {
		p.Running = sqmCakePresent(p.Interface)
	}
	return p
}

// SetSQM applies the SQM configuration with snapshot, healthcheck and rollback.
func SetSQM(cfg SQMConfig) (*SQMProbe, bool, error) {
	probe := ProbeSQM()
	if cfg.Enabled && !probe.HasWan {
		return probe, false, fmt.Errorf("no WAN interface on this router (dumb AP): SQM does not apply")
	}
	snapSqm := ""
	if _, err := os.Stat("/etc/config/sqm"); err == nil {
		if s, err := executor.Snapshot("sqm"); err == nil {
			snapSqm = s
		}
	}
	rollback := func() {
		if snapSqm != "" {
			_ = executor.Restore("sqm", snapSqm)
		} else {
			_ = executor.Run(executor.Op{Kind: "uci_delete", Args: []string{"sqm." + sqmSection}})
			_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"sqm"}})
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"sqm", "stop"}})
	}

	ops, err := sqmOps(cfg, probe)
	if err != nil {
		return probe, false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeSQM(), true, err
	}

	dev := wanDevice()
	ok := func() bool {
		for range 5 {
			p := ProbeSQM()
			if cfg.Enabled {
				if p.Active && p.Running {
					return true
				}
			} else if !p.Active && !sqmCakePresent(dev) {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	if !ok() {
		rollback()
		return ProbeSQM(), true, fmt.Errorf("healthcheck failed after apply (enabled=%v), rolled back", cfg.Enabled)
	}
	return ProbeSQM(), false, nil
}

func sqmOps(cfg SQMConfig, probe *SQMProbe) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	base := "sqm." + sqmSection

	if !cfg.Enabled {
		if uciSectionExists(base) {
			set(base+".enabled", "0")
			ops = append(ops,
				executor.Op{Kind: "uci_commit", Args: []string{"sqm"}},
				executor.Op{Kind: "initd", Args: []string{"sqm", "stop"}},
			)
		}
		return ops, nil
	}

	dl, errDl := strconv.Atoi(cfg.Download)
	ul, errUl := strconv.Atoi(cfg.Upload)
	if errDl != nil || errUl != nil || dl <= 0 || ul <= 0 {
		return nil, fmt.Errorf("download and upload rates (kbit/s) are required and must be positive")
	}
	if !sqmInstalled() {
		ops = append(ops, executor.Op{Kind: "pkg_add", Args: []string{"sqm-scripts"}})
	}
	dev := wanDevice()
	set(base, "queue")
	set(base+".interface", dev)
	set(base+".qdisc", "cake")
	set(base+".script", "piece_of_cake.qos")
	set(base+".download", cfg.Download)
	set(base+".upload", cfg.Upload)
	set(base+".enabled", "1")
	profile := cfg.Profile
	if profile == "" {
		profile = "balanced"
	}
	if !validSqmProfile(profile) {
		return nil, fmt.Errorf("unknown SQM profile %q", profile)
	}
	set(base+".profile", profile)
	ops = applyProfileOpts(ops, base, profile)
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"sqm"}},
		executor.Op{Kind: "initd", Args: []string{"sqm", "enable"}},
		executor.Op{Kind: "initd", Args: []string{"sqm", "start"}},
	)
	return ops, nil
}
