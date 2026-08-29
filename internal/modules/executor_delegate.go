package modules

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gnacho/netgrip/internal/executor"
)

const executorTokenPath = "/etc/netgrip/executor-token"

var (
	executorTokenOnce sync.Once
	executorToken     string
)

func getExecutorToken() string {
	executorTokenOnce.Do(func() {
		data, err := os.ReadFile(executorTokenPath)
		if err == nil {
			executorToken = strings.TrimSpace(string(data))
			return
		}
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return
		}
		executorToken = hex.EncodeToString(buf)
		os.MkdirAll("/etc/netgrip", 0755)
		os.WriteFile(executorTokenPath, []byte(executorToken), 0600)
	})
	return executorToken
}

func ValidateExecutorToken(token string) bool {
	expected := getExecutorToken()
	if expected == "" {
		return false
	}
	return token == expected
}

func GetExecutorToken() string {
	return getExecutorToken()
}

type ExecutorRequest struct {
	Ops []executor.Op `json:"ops"`
}

type ExecutorResponse struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func runExecutorOp(op executor.Op) error {
	switch op.Kind {
	case "uci_set", "uci_add_list", "uci_del_list", "uci_delete", "uci_commit":
		return executor.Run(op)
	case "service":
		if len(op.Args) < 2 {
			return fmt.Errorf("service requires 2 args")
		}
		action := op.Args[1]
		if action == "enable" || action == "disable" || action == "start" || action == "stop" || action == "restart" || action == "reload" {
			return executor.Run(executor.Op{Kind: "initd", Args: []string{op.Args[0], action}})
		}
		return fmt.Errorf("unsupported service action: %s", action)
	case "install", "apk_install":
		return executor.Run(executor.Op{Kind: "pkg_add", Args: op.Args})
	default:
		return fmt.Errorf("op kind %q not supported by NetGrip executor", op.Kind)
	}
}

func ExecuteOps(req ExecutorRequest) ExecutorResponse {
	for i, op := range req.Ops {
		if err := runExecutorOp(op); err != nil {
			return ExecutorResponse{
				Ok:    false,
				Error: fmt.Sprintf("op %d (%s): %s", i, op.Kind, err),
			}
		}
	}
	return ExecutorResponse{Ok: true}
}
