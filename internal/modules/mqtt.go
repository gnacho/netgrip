// mqtt.go: MQTT integration (#398). NetGrip can publish its own state and
// accept a few commands over MQTT with Home Assistant MQTT Discovery, so a
// NetGrip-only installation can be driven from Home Assistant without a
// NetPulse server.
//
// The integration is disabled by default and configured through
// /etc/netgrip/mqtt.env (KEY=VALUE, mode 0600), the same pattern as the
// embedded NetPulse agent.
package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gonzalop/mq"
)

const (
	mqttEnvFile = "/etc/netgrip/mqtt.env"

	mqttDefaultPort     = 1883
	mqttDefaultInterval = 60 // seconds between state publishes
	mqttInitialBackoff  = 5 * time.Second
	mqttMaxBackoff      = 2 * time.Minute
	mqttPublishTimeout  = 10 * time.Second
)

// MQTTConfig is the persisted configuration (mqtt.env).
type MQTTConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Pass     string `json:"pass"`
	NodeID   string `json:"node_id"`
	Interval int    `json:"interval"` // seconds between state publishes
}

// MQTTInfo is the read-only view for the API. The password is never returned.
type MQTTInfo struct {
	Enabled     bool      `json:"enabled"`
	Configured  bool      `json:"configured"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	User        string    `json:"user"`
	NodeID      string    `json:"node_id"`
	Interval    int       `json:"interval"`
	Connected   bool      `json:"connected"`
	LastError   string    `json:"last_error,omitempty"`
	LastPublish time.Time `json:"last_publish,omitempty"`
	Version     string    `json:"version,omitempty"`
}

var (
	mqttMu         sync.Mutex
	mqttBaseCtx    context.Context
	mqttBaseCancel context.CancelFunc
	mqttCancel     context.CancelFunc
	mqttStarted    bool
	mqttVersion    string
	mqttConnected  bool
	mqttLastErr    string
	mqttLastPub    time.Time
)

// StartMQTT starts the integration (call once from main). It is inert unless
// MQTT_ENABLED=1 with a host in mqtt.env.
func StartMQTT(version string) {
	mqttMu.Lock()
	if mqttStarted {
		mqttMu.Unlock()
		return
	}
	mqttStarted = true
	mqttVersion = version
	mqttBaseCtx, mqttBaseCancel = context.WithCancel(context.Background())
	mqttMu.Unlock()
	applyMQTT()
}

// StopMQTT cancels the connection loop (tests and reconfiguration).
func StopMQTT() {
	mqttMu.Lock()
	defer mqttMu.Unlock()
	if mqttCancel != nil {
		mqttCancel()
		mqttCancel = nil
	}
	if mqttBaseCancel != nil {
		mqttBaseCancel()
		mqttBaseCancel = nil
	}
	mqttConnected = false
}

// ReadMQTTConfig parses mqtt.env. Missing values fall back to defaults.
func ReadMQTTConfig(path string) (MQTTConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MQTTConfig{}, err
	}
	kv := parseEnvFile(string(data))
	cfg := MQTTConfig{
		Enabled:  kv["MQTT_ENABLED"] == "1",
		Host:     kv["MQTT_HOST"],
		Port:     mqttDefaultPort,
		User:     kv["MQTT_USER"],
		Pass:     kv["MQTT_PASS"],
		NodeID:   kv["MQTT_NODE_ID"],
		Interval: mqttDefaultInterval,
	}
	if v := kv["MQTT_PORT"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Port = n
		}
	}
	if v := kv["MQTT_INTERVAL"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Interval = n
		}
	}
	return cfg, nil
}

func writeMQTTEnv(path string, cfg MQTTConfig) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	enabled := "0"
	if cfg.Enabled {
		enabled = "1"
	}
	var b strings.Builder
	b.WriteString("# managed by netgrip; MQTT integration config\n")
	b.WriteString("MQTT_ENABLED=" + enabled + "\n")
	b.WriteString("MQTT_HOST=" + cfg.Host + "\n")
	if cfg.Port > 0 {
		b.WriteString("MQTT_PORT=" + strconv.Itoa(cfg.Port) + "\n")
	}
	if cfg.User != "" {
		b.WriteString("MQTT_USER=" + cfg.User + "\n")
	}
	if cfg.Pass != "" {
		b.WriteString("MQTT_PASS=" + cfg.Pass + "\n")
	}
	if cfg.NodeID != "" {
		b.WriteString("MQTT_NODE_ID=" + cfg.NodeID + "\n")
	}
	if cfg.Interval > 0 {
		b.WriteString("MQTT_INTERVAL=" + strconv.Itoa(cfg.Interval) + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

// SetMQTTConfig persists the config (empty password keeps the current one) and
// (re)starts the connection loop without restarting the netgrip process.
func SetMQTTConfig(cfg MQTTConfig) error {
	return setMQTTConfigAt(mqttEnvFile, cfg)
}

func setMQTTConfigAt(path string, cfg MQTTConfig) error {
	if cfg.Pass == "" {
		if old, err := ReadMQTTConfig(path); err == nil {
			cfg.Pass = old.Pass
		}
	}
	if cfg.Enabled && strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("host is required to enable MQTT")
	}
	if err := writeMQTTEnv(path, cfg); err != nil {
		return err
	}
	applyMQTT()
	return nil
}

// MQTTInfoNow composes the API view from the env file plus runtime state.
func MQTTInfoNow() MQTTInfo {
	cfg, err := ReadMQTTConfig(mqttEnvFile)
	if err != nil {
		cfg = MQTTConfig{Interval: mqttDefaultInterval, Port: mqttDefaultPort}
	}
	mqttMu.Lock()
	info := MQTTInfo{
		Connected:   mqttConnected,
		LastError:   mqttLastErr,
		LastPublish: mqttLastPub,
		Version:     mqttVersion,
	}
	mqttMu.Unlock()
	info.Enabled = cfg.Enabled
	info.Host = cfg.Host
	info.Port = cfg.Port
	info.User = cfg.User
	info.NodeID = mqttNodeID(cfg)
	info.Interval = cfg.Interval
	info.Configured = strings.TrimSpace(cfg.Host) != ""
	return info
}

// applyMQTT reads the config and (re)starts the connection loop. Always call
// it after changing the config: it cancels the running loop first.
func applyMQTT() {
	mqttMu.Lock()
	if mqttCancel != nil {
		mqttCancel()
		mqttCancel = nil
	}
	mqttConnected = false
	mqttLastErr = ""
	base := mqttBaseCtx
	version := mqttVersion
	mqttMu.Unlock()
	if base == nil {
		base = context.Background()
	}

	cfg, err := ReadMQTTConfig(mqttEnvFile)
	if err != nil {
		cfg = MQTTConfig{}
	}
	if !cfg.Enabled {
		return
	}
	if strings.TrimSpace(cfg.Host) == "" {
		log.Printf("mqtt: enabled but MQTT_HOST is empty; not starting")
		return
	}
	node := mqttNodeID(cfg)
	ctx, cancel := context.WithCancel(base)
	mqttMu.Lock()
	mqttCancel = cancel
	mqttMu.Unlock()
	go runMQTT(ctx, cfg, node, version)
}

// runMQTT keeps a connection to the broker, retrying with backoff until ctx is
// cancelled. The MQTT library handles reconnection once the first Dial
// succeeds, so this loop only covers the initial connect.
func runMQTT(ctx context.Context, cfg MQTTConfig, node, version string) {
	backoff := mqttInitialBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		client, err := mqttDial(ctx, cfg, node, version)
		if err != nil {
			log.Printf("mqtt: connect to %s:%d failed: %v", cfg.Host, cfg.Port, err)
			setMQTTError(err)
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > mqttMaxBackoff {
				backoff = mqttMaxBackoff
			}
			continue
		}
		backoff = mqttInitialBackoff
		log.Printf("mqtt: connected to %s:%d as node %q", cfg.Host, cfg.Port, node)

		// #398 / gonzalop/mq v0.9.10 workaround: WithSubscription only
		// registers the handler locally and the initial SUBSCRIBE is never
		// sent with CleanSession=true (it is only sent on reconnect). Do the
		// first subscription explicitly; reconnects go through the library.
		if tok := client.Subscribe(ctx, commandFilter(node), mq.AtLeastOnce, func(c *mq.Client, msg mq.Message) {
			go handleMQTTCommand(c, node, version, msg)
		}); tok != nil {
			if err := tok.Wait(ctx); err != nil && ctx.Err() == nil {
				log.Printf("mqtt: subscribe %s: %v", commandFilter(node), err)
			}
		}

		publishAvailability(client, node)
		publishDiscovery(client, node, version)
		publishState(client, node, version)

		stateLoop(ctx, client, node, version, time.Duration(cfg.Interval)*time.Second)

		// Say goodbye cleanly: the retained availability must not stay
		// "online" after an intentional stop (a graceful DISCONNECT
		// suppresses the Will).
		publishAvailabilityPayload(client, node, "offline")
		_ = client.Disconnect(context.Background())
		setMQTTDisconnected()
		return
	}
}

func mqttDial(ctx context.Context, cfg MQTTConfig, node, version string) (*mq.Client, error) {
	addr := fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)
	opts := []mq.Option{
		mq.WithProtocolVersion(mq.ProtocolV311),
		mq.WithClientID("netgrip-" + node),
		mq.WithKeepAlive(60 * time.Second),
		mq.WithAutoReconnect(true),
		mq.WithReconnectBackoff(2*time.Second, mqttMaxBackoff, true),
		mq.WithConnectTimeout(15 * time.Second),
		mq.WithWill(availabilityTopic(node), []byte("offline"), 1, true),
		mq.WithOnConnect(func(c *mq.Client) {
			setMQTTConnected()
			// Runs on the initial connect and on every reconnect; keeps
			// discovery and state fresh after the broker comes back.
			publishAvailability(c, node)
			publishDiscovery(c, node, version)
			publishState(c, node, version)
		}),
		mq.WithOnConnectionLost(func(_ *mq.Client, err error) {
			log.Printf("mqtt: connection lost: %v", err)
			setMQTTError(err)
			setMQTTDisconnected()
		}),
		// Registers the handler locally so the library re-subscribes on every
		// reconnect (the initial SUBSCRIBE is issued explicitly after Dial,
		// see the note there).
		mq.WithSubscription(commandFilter(node), func(c *mq.Client, msg mq.Message) {
			go handleMQTTCommand(c, node, version, msg)
		}),
	}
	if cfg.User != "" {
		opts = append(opts, mq.WithCredentials(cfg.User, cfg.Pass))
	}
	dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return mq.DialContext(dctx, addr, opts...)
}

// stateLoop publishes the retained state every interval until ctx ends.
func stateLoop(ctx context.Context, client *mq.Client, node, version string, interval time.Duration) {
	if interval <= 0 {
		interval = mqttDefaultInterval * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if client.IsConnected() {
				publishState(client, node, version)
			}
		}
	}
}

// handleMQTTCommand runs one command on the "command/#" subscription.
func handleMQTTCommand(client *mq.Client, node, version string, msg mq.Message) {
	action := strings.TrimPrefix(msg.Topic, "netgrip/"+node+"/command/")
	payload := strings.ToUpper(strings.TrimSpace(string(msg.Payload)))
	log.Printf("mqtt: command %q payload %q", action, payload)

	var err error
	switch action {
	case "guest_wifi":
		err = mqttGuestWifi(payload)
	case "banip":
		err = mqttBanip(payload)
	case "ipv6":
		err = mqttIPv6(payload)
	case "sqm":
		err = mqttSQM(payload)
	case "reboot":
		err = RebootRouter()
	default:
		err = fmt.Errorf("unsupported command %q", action)
	}
	publishResult(client, node, action, err)
	if client.IsConnected() {
		publishState(client, node, version)
	}
}

func mqttGuestWifi(payload string) error {
	on, err := parseOnOff(payload)
	if err != nil {
		return err
	}
	cur := ProbeGuest()
	// Idempotent: a switch that is already in the requested state must not
	// re-apply the config (a guest apply reloads both radios and runs a
	// healthcheck of up to 75s).
	if on == cur.Active {
		return nil
	}
	if !on {
		_, _, err := SetGuest(GuestConfig{Enabled: false})
		return err
	}
	if !uciSectionExists("network.guest") {
		return errors.New("guest wifi is not configured; set it up in the panel first")
	}
	// Re-enable the existing network: pass the current SSID so the update path
	// flips disabled=0 without touching the rest of the config.
	_, _, err = SetGuest(GuestConfig{Enabled: true, SSID: cur.SSID})
	return err
}

func mqttBanip(payload string) error {
	switch payload {
	case "RELOAD":
		_, _, err := BanipAction("reload")
		return err
	case "ON":
		if ProbeBanipStatusCached().Enabled {
			return nil
		}
		_, _, err := BanipAction("enable")
		return err
	case "OFF":
		if !ProbeBanipStatusCached().Enabled {
			return nil
		}
		_, _, err := BanipAction("disable")
		return err
	}
	return fmt.Errorf("banip expects ON, OFF or RELOAD, got %q", payload)
}

func mqttIPv6(payload string) error {
	on, err := parseOnOff(payload)
	if err != nil {
		return err
	}
	st := ProbeIPv6()
	if on && st.State == "enabled" || !on && st.State == "disabled" {
		return nil
	}
	_, _, err = SetIPv6(on)
	return err
}

func mqttSQM(payload string) error {
	on, err := parseOnOff(payload)
	if err != nil {
		return err
	}
	p := ProbeSQM()
	if on == p.Active {
		return nil
	}
	if !on {
		_, _, err := SetSQM(SQMConfig{Enabled: false})
		return err
	}
	if p.Download == "" || p.Upload == "" {
		return errors.New("sqm is not configured; set it up in the panel first")
	}
	_, _, err = SetSQM(SQMConfig{Enabled: true, Download: p.Download, Upload: p.Upload, Profile: p.Profile})
	return err
}

// parseOnOff accepts the payloads Home Assistant sends for switches.
func parseOnOff(payload string) (bool, error) {
	switch payload {
	case "ON", "1", "TRUE":
		return true, nil
	case "OFF", "0", "FALSE":
		return false, nil
	}
	return false, fmt.Errorf("expected ON or OFF, got %q", payload)
}

// mqttNodeID resolves the node identifier used in topics: the configured value
// or the hostname. Characters that cannot appear in a topic are replaced.
func mqttNodeID(cfg MQTTConfig) string {
	node := strings.TrimSpace(cfg.NodeID)
	if node == "" {
		node, _ = os.Hostname()
	}
	node = strings.TrimSpace(node)
	if node == "" {
		node = "netgrip"
	}
	node = strings.NewReplacer("/", "-", "+", "-", "#", "-", " ", "-").Replace(node)
	return node
}

func availabilityTopic(node string) string { return "netgrip/" + node + "/availability" }
func stateTopic(node string) string        { return "netgrip/" + node + "/state" }
func resultTopic(node string) string       { return "netgrip/" + node + "/result" }
func commandFilter(node string) string     { return "netgrip/" + node + "/command/#" }

// publishResult reports the outcome of a command on the (non-retained) result
// topic.
func publishResult(client *mq.Client, node, action string, cmdErr error) {
	res := struct {
		Action string `json:"action"`
		OK     bool   `json:"ok"`
		Error  string `json:"error,omitempty"`
		Ts     int64  `json:"ts"`
	}{Action: action, OK: cmdErr == nil, Ts: time.Now().Unix()}
	if cmdErr != nil {
		res.Error = cmdErr.Error()
		log.Printf("mqtt: command %q failed: %v", action, cmdErr)
	}
	payload, err := json.Marshal(res)
	if err != nil {
		return
	}
	mqttPublish(client, resultTopic(node), payload, false)
}

func publishAvailability(client *mq.Client, node string) {
	publishAvailabilityPayload(client, node, "online")
}

func publishAvailabilityPayload(client *mq.Client, node, value string) {
	mqttPublish(client, availabilityTopic(node), []byte(value), true)
}

// mqttPublish sends one payload at QoS 1. Retained when retain is true. It is
// best-effort: failures are logged and reflected in the runtime error.
func mqttPublish(client *mq.Client, topic string, payload []byte, retain bool) {
	if !client.IsConnected() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), mqttPublishTimeout)
	defer cancel()
	opts := []mq.PublishOption{mq.WithQoS(mq.AtLeastOnce)}
	if retain {
		opts = append(opts, mq.WithRetain(true))
	}
	if err := client.Publish(ctx, topic, payload, opts...).Wait(ctx); err != nil {
		log.Printf("mqtt: publish %s: %v", topic, err)
		setMQTTError(err)
		return
	}
	mqttMu.Lock()
	mqttLastPub = time.Now()
	mqttLastErr = ""
	mqttMu.Unlock()
}

func setMQTTConnected() {
	mqttMu.Lock()
	mqttConnected = true
	mqttLastErr = ""
	mqttMu.Unlock()
}

func setMQTTDisconnected() {
	mqttMu.Lock()
	mqttConnected = false
	mqttMu.Unlock()
}

func setMQTTError(err error) {
	if err == nil {
		return
	}
	mqttMu.Lock()
	mqttLastErr = err.Error()
	mqttMu.Unlock()
}

// sleepCtx sleeps for d, returning false if ctx ends first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
