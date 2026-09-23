// mqtt_ops.go: prueba de conexión del broker y op de orquestación
// mqtt.configure (#409), que NetPulse envía a los routers NetGrip para
// configurarles el MQTT sin tocarlos a mano.
package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gonzalop/mq"
)

// mqttProbe comprueba que el broker responde y acepta las credenciales: hace
// un CONNECT y desconecta. Sirve para el botón "probar conexión" de la tarjeta
// y para verificar un mqtt.configure antes de darlo por bueno.
func mqttProbe(ctx context.Context, cfg MQTTConfig, node string) error {
	addr := fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)
	opts := []mq.Option{
		mq.WithProtocolVersion(mq.ProtocolV311),
		mq.WithClientID("netgrip-probe-" + node),
		mq.WithConnectTimeout(5 * time.Second),
	}
	if cfg.User != "" {
		opts = append(opts, mq.WithCredentials(cfg.User, cfg.Pass))
	}
	pctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	client, err := mq.DialContext(pctx, addr, opts...)
	if err != nil {
		return err
	}
	client.Disconnect(pctx)
	return nil
}

// MQTTTestConnection expone la sonda para la API: prueba los valores del
// formulario sin guardarlos.
func MQTTTestConnection(cfg MQTTConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("host is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return mqttProbe(ctx, cfg, mqttNodeID(cfg))
}

// mqttOpEnvPath es la ruta del env que escribe el op; inyectable en tests.
var mqttOpEnvPath = mqttEnvFile

// applyMQTTConfigureOp aplica un mqtt.configure venido de NetPulse: persiste
// /etc/netgrip/mqtt.env y lo aplica en caliente. Si el broker nuevo no
// responde, restaura la configuración anterior y devuelve error.
func applyMQTTConfigureOp(op executor.Op) error {
	// Args: enabled host port user pass nodeId interval
	if len(op.Args) != 7 {
		return fmt.Errorf("mqtt.configure needs 7 args, got %d", len(op.Args))
	}
	enabled := op.Args[0] == "1" || strings.EqualFold(op.Args[0], "true")
	port := mqttDefaultPort
	if op.Args[2] != "" {
		p, err := strconv.Atoi(op.Args[2])
		if err != nil || p <= 0 || p > 65535 {
			return fmt.Errorf("invalid port %q", op.Args[2])
		}
		port = p
	}
	interval, err := strconv.Atoi(op.Args[6])
	if err != nil || interval <= 0 {
		interval = mqttDefaultInterval
	}
	next := MQTTConfig{
		Enabled:  enabled,
		Host:     op.Args[1],
		Port:     port,
		User:     op.Args[3],
		Pass:     op.Args[4],
		NodeID:   op.Args[5],
		Interval: interval,
	}

	prev, prevErr := ReadMQTTConfig(mqttOpEnvPath)
	if err := setMQTTConfigAt(mqttOpEnvPath, next); err != nil {
		return fmt.Errorf("write mqtt config: %w", err)
	}
	if !next.Enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := mqttProbe(ctx, next, mqttNodeID(next)); err != nil {
		if prevErr == nil {
			if rerr := setMQTTConfigAt(mqttOpEnvPath, prev); rerr != nil {
				return fmt.Errorf("broker unreachable (%v) and rollback failed: %w", err, rerr)
			}
		}
		return fmt.Errorf("broker unreachable with the new settings: %w", err)
	}
	return nil
}
