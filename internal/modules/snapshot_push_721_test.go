package modules

import (
	"os"
	"path/filepath"
	"testing"
)

// #721: sin push-config guardado, los datos salen del agente NetPulse.
func TestPushConfigFallsBackToNetPulseAgent(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "agent-config")
	content := "NETPULSE_SERVER=https://netpulse.example.com\nNETPULSE_SLUG=rt1\nNETPULSE_TOKEN=tok123\nNETPULSE_ENABLED=1\n"
	if err := os.WriteFile(agentFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	oldConfigPath := pushConfigPath
	oldEnv := pushNetPulseEnvPath
	t.Cleanup(func() { pushConfigPath = oldConfigPath; pushNetPulseEnvPath = oldEnv })
	pushConfigPath = filepath.Join(dir, "push-config.json")
	pushNetPulseEnvPath = func() string { return agentFile }

	cfg := GetPushConfig()
	if cfg.ServerURL != "https://netpulse.example.com" || cfg.RouterID != "rt1" || cfg.Token != "tok123" {
		t.Fatalf("expected values from the agent, got %+v", cfg)
	}
}

// Lo guardado a mano manda sobre el agente.
func TestPushConfigSavedValuesWin(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "agent-config")
	_ = os.WriteFile(agentFile, []byte("NETPULSE_SERVER=https://agent.example.com\nNETPULSE_SLUG=agent\n"), 0600)

	path := filepath.Join(dir, "push-config.json")
	if err := os.WriteFile(path, []byte(`{"server_url":"https://mine.example.com","router_id":"mine","token":"x"}`), 0600); err != nil {
		t.Fatal(err)
	}

	oldConfigPath := pushConfigPath
	oldEnv := pushNetPulseEnvPath
	t.Cleanup(func() { pushConfigPath = oldConfigPath; pushNetPulseEnvPath = oldEnv })
	pushConfigPath = path
	pushNetPulseEnvPath = func() string { return agentFile }

	cfg := GetPushConfig()
	if cfg.ServerURL != "https://mine.example.com" || cfg.RouterID != "mine" {
		t.Fatalf("saved config should win, got %+v", cfg)
	}
}
