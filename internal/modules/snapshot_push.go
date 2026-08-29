package modules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	pushConfigPath = "/etc/netgrip/push-config.json"
	pushTimeout    = 30 * time.Second
)

type PushConfig struct {
	ServerURL string `json:"server_url"`
	RouterID  string `json:"router_id"`
	Token     string `json:"token"`
}

type PushResult struct {
	Ok         bool   `json:"ok"`
	SnapshotID string `json:"snapshot_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

func GetPushConfig() PushConfig {
	data, err := os.ReadFile(pushConfigPath)
	if err != nil {
		return PushConfig{}
	}
	var cfg PushConfig
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func SetPushConfig(cfg PushConfig) error {
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	os.MkdirAll("/etc/netgrip", 0755)
	return os.WriteFile(pushConfigPath, data, 0600)
}

func PushLatestSnapshot() PushResult {
	cfg := GetPushConfig()
	if cfg.ServerURL == "" || cfg.RouterID == "" {
		return PushResult{Error: "push not configured: set server URL and router ID first"}
	}

	snaps := ListSnapshots()
	if len(snaps) == 0 {
		return PushResult{Error: "no snapshots available"}
	}
	latest := snaps[len(snaps)-1]

	data, err := ExportSnapshot(latest.ID)
	if err != nil {
		return PushResult{Error: fmt.Sprintf("export snapshot: %s", err)}
	}

	configs := make([]string, len(uciConfigs))
	copy(configs, uciConfigs)
	configsStr := strings.Join(configs, ",")

	req, err := http.NewRequest("POST", cfg.ServerURL+"/api/config-backup", bytes.NewReader(data))
	if err != nil {
		return PushResult{Error: fmt.Sprintf("build request: %s", err)}
	}
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("X-Router-ID", cfg.RouterID)
	req.Header.Set("X-Snapshot-ID", latest.ID)
	req.Header.Set("X-Configs", configsStr)
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	client := &http.Client{Timeout: pushTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return PushResult{Error: fmt.Sprintf("push failed: %s", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PushResult{Error: fmt.Sprintf("server returned %d", resp.StatusCode)}
	}

	return PushResult{Ok: true, SnapshotID: latest.ID}
}
