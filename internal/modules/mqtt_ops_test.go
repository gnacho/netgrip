package modules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gnacho/netgrip/internal/executor"
)

func TestApplyMQTTConfigureOp(t *testing.T) {
	dir := t.TempDir()
	old := mqttOpEnvPath
	defer func() { mqttOpEnvPath = old }()
	mqttOpEnvPath = filepath.Join(dir, "mqtt.env")

	// Configuración previa: activado.
	if err := os.WriteFile(mqttOpEnvPath, []byte("MQTT_ENABLED=1\nMQTT_HOST=10.0.0.10\nMQTT_PORT=1883\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Args incompletos.
	if err := applyMQTTConfigureOp(executor.Op{Kind: "mqtt.configure", Args: []string{"1", "h"}}); err == nil {
		t.Fatal("must fail with incomplete args")
	}
	// Puerto inválido.
	if err := applyMQTTConfigureOp(executor.Op{Kind: "mqtt.configure", Args: []string{"1", "h", "x", "", "", "", "60"}}); err == nil {
		t.Fatal("must fail with an invalid port")
	}

	// Desactivado: aplica sin sondear el broker.
	if err := applyMQTTConfigureOp(executor.Op{Kind: "mqtt.configure", Args: []string{"0", "", "", "", "", "", "60"}}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	cfg, err := ReadMQTTConfig(mqttOpEnvPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("must end up disabled")
	}

	// Activado con un broker inalcanzable: falla y revierte a lo anterior.
	if err := applyMQTTConfigureOp(executor.Op{Kind: "mqtt.configure", Args: []string{"1", "127.0.0.1", "1", "", "", "", "60"}}); err == nil {
		t.Fatal("must fail with an unreachable broker")
	}
	cfg, err = ReadMQTTConfig(mqttOpEnvPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("must have rolled back to the previous (disabled) config")
	}
}
