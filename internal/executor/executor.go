package executor

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Op is a single allowlisted operation. Nothing outside these kinds can run.
type Op struct {
	Kind string   `json:"kind"`
	Args []string `json:"args"`
}

var (
	reUCIKey    = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-zA-Z0-9_@:-]+)+$`)
	reUCIConfig = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	reService   = regexp.MustCompile(`^[a-z0-9_-]+$`)
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
	case "uci_delete":
		if len(op.Args) != 1 || !reUCIKey.MatchString(op.Args[0]) {
			return fmt.Errorf("invalid uci_delete args: %v", op.Args)
		}
	case "uci_commit":
		if len(op.Args) != 1 || !reUCIConfig.MatchString(op.Args[0]) {
			return fmt.Errorf("invalid uci_commit args: %v", op.Args)
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
	case "uci_delete":
		cmd = exec.Command("uci", "delete", op.Args[0])
	case "uci_commit":
		cmd = exec.Command("uci", "commit", op.Args[0])
	case "initd":
		cmd = exec.Command("/etc/init.d/"+op.Args[0], op.Args[1])
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", op.Kind, op.Args, err, strings.TrimSpace(string(out)))
	}
	return nil
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
func Restore(config, content string) error {
	if !reUCIConfig.MatchString(config) {
		return fmt.Errorf("invalid config name: %q", config)
	}
	cmd := exec.Command("uci", "import", "-f", config)
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
