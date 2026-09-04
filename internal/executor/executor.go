package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Op is a single allowlisted operation. Nothing outside these kinds can run.
type Op struct {
	Kind string   `json:"kind"`
	Args []string `json:"args"`
}

// UnmarshalJSON accepts `args` both as the positional []string (internal form)
// and as the object NetPulse sends ({config,section,option,value,...}); the
// object form is converted into the internal key/value args per kind (#235).
func (o *Op) UnmarshalJSON(b []byte) error {
	var raw struct {
		Kind string          `json:"kind"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	o.Kind = raw.Kind
	args, err := decodeOpArgs(raw.Kind, raw.Args)
	if err != nil {
		return err
	}
	o.Args = args
	return nil
}

// decodeOpArgs turns the raw `args` JSON into the internal []string form.
func decodeOpArgs(kind string, raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// Positional form: keep as-is.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	// Object form: convert {config,section,option,value} / {service,action}
	// into the internal positional args.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("args must be an array or object")
	}
	get := func(k string) string {
		if v, ok := m[k]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				return s
			}
			return strings.TrimSpace(string(v))
		}
		return ""
	}
	switch kind {
	case "uci_set", "uci_add_list", "uci_del_list":
		key := uciKeyFromObject(m)
		return []string{key, get("value")}, nil
	case "uci_delete":
		return []string{uciKeyFromObject(m)}, nil
	case "uci_commit":
		return []string{get("config")}, nil
	case "service":
		name := get("service")
		if name == "" {
			name = get("name")
		}
		return []string{name, get("action")}, nil
	case "install", "apk_install":
		if p := get("package"); p != "" {
			return []string{p}, nil
		}
		return []string{get("name")}, nil
	default:
		// Generic: collect the object values in a stable key order.
		keys := []string{"config", "section", "option", "value", "service", "action"}
		var out []string
		for _, k := range keys {
			if v := get(k); v != "" {
				out = append(out, v)
			}
		}
		return out, nil
	}
}

func uciKeyFromObject(m map[string]json.RawMessage) string {
	get := func(k string) string {
		if v, ok := m[k]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				return s
			}
		}
		return ""
	}
	key := get("config")
	if s := get("section"); s != "" {
		key += "." + s
	}
	if s := get("option"); s != "" {
		key += "." + s
	}
	return key
}

var (
	// UCI keys look like config.section.option. Sections may be named
	// ("ddns.netgrip") or indexed ("network.@device[0]", the form printed
	// by `uci show` for anonymous sections and accepted by the CLI).
	reUCIKey    = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.(@[a-zA-Z0-9_-]+\[\d+\]|[a-zA-Z0-9_@:-]+))+$`)
	reUCIConfig = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	reService   = regexp.MustCompile(`^[a-z0-9_-]+$`)
	rePkg       = regexp.MustCompile(`^[a-z0-9][a-z0-9+_.-]*$`)
	reBadChars  = regexp.MustCompile("[;\n\r`]")
)

var initdActions = map[string]bool{
	"enable": true, "disable": true, "start": true,
	"stop": true, "restart": true, "reload": true,
}

// Validate checks an op against the allowlist before it can run.
func Validate(op Op) error {
	switch op.Kind {
	case "uci_set":
		// Args: <config.section.option> <value>
		if len(op.Args) != 2 || !reUCIKey.MatchString(op.Args[0]) || reBadChars.MatchString(op.Args[1]) {
			return fmt.Errorf("invalid uci_set args: %v", op.Args)
		}
	case "uci_add_list", "uci_del_list":
		// Args: <config.section.option> <value>
		if len(op.Args) != 2 || !reUCIKey.MatchString(op.Args[0]) || reBadChars.MatchString(op.Args[1]) {
			return fmt.Errorf("invalid %s args: %v", op.Kind, op.Args)
		}
	case "uci_delete":
		if len(op.Args) != 1 || !reUCIKey.MatchString(op.Args[0]) {
			return fmt.Errorf("invalid uci_delete args: %v", op.Args)
		}
	case "uci_commit":
		if len(op.Args) != 1 || !reUCIConfig.MatchString(op.Args[0]) {
			return fmt.Errorf("invalid uci_commit args: %v", op.Args)
		}
	case "pkg_add", "apk_upgrade":
		// Args: package names; apk on 25.12+, opkg on older releases.
		// NOTE: `apk add <pkg>` does NOT upgrade an installed package in
		// apk v3 (it keeps the installed version, verified on 25.12.5);
		// upgrades need `apk upgrade <pkg>`.
		if len(op.Args) == 0 {
			return fmt.Errorf("%s needs at least one package", op.Kind)
		}
		for _, pkg := range op.Args {
			if !rePkg.MatchString(pkg) {
				return fmt.Errorf("invalid package name: %q", pkg)
			}
		}
	case "pkg_del":
		// Args: package names to uninstall (apk del / opkg remove).
		if len(op.Args) == 0 {
			return fmt.Errorf("pkg_del needs at least one package")
		}
		for _, pkg := range op.Args {
			if !rePkg.MatchString(pkg) {
				return fmt.Errorf("invalid package name: %q", pkg)
			}
		}
	case "wifi_reload", "wifi_reconf":
		// Args: optional radio name (empty = all radios).
		if len(op.Args) > 1 || (len(op.Args) == 1 && !reService.MatchString(op.Args[0])) {
			return fmt.Errorf("invalid %s args: %v", op.Kind, op.Args)
		}
	case "ifup", "ifdown":
		if len(op.Args) != 1 || !reService.MatchString(op.Args[0]) {
			return fmt.Errorf("invalid %s args: %v", op.Kind, op.Args)
		}
	case "ip_link":
		// Args: <iface> <up|down>
		if len(op.Args) != 2 || !reService.MatchString(op.Args[0]) || (op.Args[1] != "up" && op.Args[1] != "down") {
			return fmt.Errorf("invalid ip_link args: %v", op.Args)
		}
	case "initd":
		if len(op.Args) != 2 || !reService.MatchString(op.Args[0]) || !initdActions[op.Args[1]] {
			return fmt.Errorf("invalid initd args: %v", op.Args)
		}
	default:
		return fmt.Errorf("op kind not allowlisted: %q", op.Kind)
	}
	return nil
}

// Run executes a validated op.
func Run(op Op) error {
	if err := Validate(op); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch op.Kind {
	case "uci_set":
		cmd = exec.Command("uci", "set", op.Args[0]+"="+op.Args[1])
	case "uci_add_list":
		cmd = exec.Command("uci", "add_list", op.Args[0]+"="+op.Args[1])
	case "uci_del_list":
		cmd = exec.Command("uci", "del_list", op.Args[0]+"="+op.Args[1])
	case "uci_delete":
		cmd = exec.Command("uci", "delete", op.Args[0])
	case "uci_commit":
		cmd = exec.Command("uci", "commit", op.Args[0])
	case "pkg_add":
		if _, err := exec.LookPath("apk"); err == nil {
			cmd = exec.Command("apk", append([]string{"add"}, op.Args...)...)
		} else {
			// opkg needs the feed indexes in /var/opkg-lists; a freshly
			// flashed router has none and `opkg install` fails with
			// "Unknown package". Self-heal: refresh once when missing.
			if opkgListsMissingDir(opkgListsDir) {
				if out, err := exec.Command("opkg", "update").CombinedOutput(); err != nil {
					return fmt.Errorf("pkg_add [opkg update]: %w (%s)", err, strings.TrimSpace(string(out)))
				}
			}
			cmd = exec.Command("opkg", append([]string{"install"}, op.Args...)...)
		}
	case "pkg_del":
		if _, err := exec.LookPath("apk"); err == nil {
			cmd = exec.Command("apk", append([]string{"del"}, op.Args...)...)
		} else {
			cmd = exec.Command("opkg", append([]string{"remove"}, op.Args...)...)
		}
	case "apk_upgrade":
		cmd = exec.Command("apk", append([]string{"upgrade"}, op.Args...)...)
	case "wifi_reload":
		cmd = exec.Command("wifi", append([]string{"reload"}, op.Args...)...)
	case "wifi_reconf":
		cmd = exec.Command("wifi", append([]string{"reconf"}, op.Args...)...)
	case "ifup", "ifdown":
		cmd = exec.Command(op.Kind, op.Args[0])
	case "ip_link":
		cmd = exec.Command("ip", "link", "set", op.Args[0], op.Args[1])
	case "initd":
		cmd = exec.Command("/etc/init.d/"+op.Args[0], op.Args[1])
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", op.Kind, op.Args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// opkgListsDir is where opkg caches the feed indexes.
const opkgListsDir = "/var/opkg-lists"

// opkgListsMissingDir reports whether dir holds no feed indexes, meaning
// `opkg install` would fail with "Unknown package" until `opkg update` runs.
// A missing dir counts as missing lists.
func opkgListsMissingDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

// Apply runs ops in order; on failure it runs rollbackOps (best effort,
// reverse order) and reports the original error.
func Apply(ops []Op, rollbackOps []Op) error {
	for i, op := range ops {
		if err := Run(op); err != nil {
			for j := len(rollbackOps) - 1; j >= 0; j-- {
				_ = Run(rollbackOps[j])
			}
			return fmt.Errorf("op %d (%s) failed, rollback attempted: %w", i, op.Kind, err)
		}
	}
	return nil
}

// Snapshot exports a UCI config for later restore.
func Snapshot(config string) (string, error) {
	if !reUCIConfig.MatchString(config) {
		return "", fmt.Errorf("invalid config name: %q", config)
	}
	out, err := exec.Command("uci", "export", config).Output()
	if err != nil {
		return "", fmt.Errorf("uci export %s: %w", config, err)
	}
	return string(out), nil
}

// Restore imports a previously exported UCI config and commits it.
// Note: `uci import <config>` reads from stdin and REPLACES the package
// (merge only happens with the -m flag). The -f global option means
// "use <file> as input", NOT "full package": `uci import -f network`
// is invalid and made restores silently fail (verified on 25.12.5).
func Restore(config, content string) error {
	if !reUCIConfig.MatchString(config) {
		return fmt.Errorf("invalid config name: %q", config)
	}
	cmd := exec.Command("uci", "import", config)
	cmd.Stdin = bytes.NewBufferString(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uci import %s: %w (%s)", config, err, strings.TrimSpace(string(out)))
	}
	return Run(Op{Kind: "uci_commit", Args: []string{config}})
}

// ServiceEnabled reports whether an init.d service is enabled.
func ServiceEnabled(name string) bool {
	if !reService.MatchString(name) {
		return false
	}
	return exec.Command("/etc/init.d/"+name, "enabled").Run() == nil
}

// ServiceRunning reports whether an init.d service is running.
func ServiceRunning(name string) bool {
	if !reService.MatchString(name) {
		return false
	}
	return exec.Command("/etc/init.d/"+name, "running").Run() == nil
}
