package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMQTTNodeID(t *testing.T) {
	cases := []struct {
		name string
		cfg  MQTTConfig
		want string
	}{
		{"explicit", MQTTConfig{NodeID: "rt3"}, "rt3"},
		{"sanitizes", MQTTConfig{NodeID: "my router/#1"}, "my-router--1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mqttNodeID(tc.cfg); got != tc.want {
				t.Fatalf("mqttNodeID(%+v) = %q, want %q", tc.cfg, got, tc.want)
			}
		})
	}
	// Without a configured id it must fall back to a non-empty hostname.
	if got := mqttNodeID(MQTTConfig{}); got == "" {
		t.Fatal("mqttNodeID(empty) returned empty")
	}
}

func TestMQTTConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mqtt.env")
	in := MQTTConfig{
		Enabled:     true,
		Host:        "broker.example",
		Port:        8883,
		User:        "mnemonic",
		Pass:        "s3cret",
		NodeID:      "rt-lab",
		Interval:    120,
		TLS:         true,
		TLSInsecure: true,
	}
	if err := writeMQTTEnv(path, in); err != nil {
		t.Fatalf("writeMQTTEnv: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("env file mode = %o, want 600", perm)
	}
	out, err := ReadMQTTConfig(path)
	if err != nil {
		t.Fatalf("ReadMQTTConfig: %v", err)
	}
	if out != in {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func TestReadMQTTConfigDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mqtt.env")
	if err := os.WriteFile(path, []byte("MQTT_ENABLED=1\nMQTT_HOST=broker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := ReadMQTTConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != mqttDefaultPort || cfg.Interval != mqttDefaultInterval {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if !cfg.Enabled || cfg.Host != "broker" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestReadMQTTConfigTLSDefaultsPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mqtt.env")
	body := "MQTT_ENABLED=1\nMQTT_HOST=broker\nMQTT_TLS=1\nMQTT_TLS_INSECURE=1\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := ReadMQTTConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TLS || !cfg.TLSInsecure {
		t.Fatalf("tls flags not read back: %+v", cfg)
	}
	if cfg.Port != mqttDefaultTLSPort {
		t.Fatalf("port = %d, want %d with TLS", cfg.Port, mqttDefaultTLSPort)
	}
}

func TestWriteMQTTEnvDropsTLSWhenOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mqtt.env")
	if err := writeMQTTEnv(path, MQTTConfig{Enabled: true, Host: "broker", Port: 8883, TLS: true, TLSInsecure: true}); err != nil {
		t.Fatal(err)
	}
	if err := writeMQTTEnv(path, MQTTConfig{Enabled: true, Host: "broker", Port: 1883}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "MQTT_TLS") {
		t.Fatalf("TLS keys left in the env file:\n%s", raw)
	}
	cfg, err := ReadMQTTConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TLS || cfg.TLSInsecure {
		t.Fatalf("tls flags survived a plain rewrite: %+v", cfg)
	}
}

func TestSetMQTTConfigAtTLSPort(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		cfg  MQTTConfig
		port int
	}{
		{"tls", MQTTConfig{Host: "broker", TLS: true, TLSInsecure: true}, mqttDefaultTLSPort},
		{"plain", MQTTConfig{Host: "broker", TLSInsecure: true}, mqttDefaultPort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".env")
			if err := setMQTTConfigAt(path, tc.cfg); err != nil {
				t.Fatalf("setMQTTConfigAt: %v", err)
			}
			got, err := ReadMQTTConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			if got.Port != tc.port {
				t.Fatalf("port = %d, want %d", got.Port, tc.port)
			}
			if tc.name == "plain" && got.TLSInsecure {
				t.Fatal("tls insecure must be cleared when TLS is off")
			}
		})
	}
}

func TestParseOnOff(t *testing.T) {
	for _, in := range []string{"ON", "1", "TRUE", "on"} {
		if on, err := parseOnOff(strings.ToUpper(in)); err != nil || !on {
			t.Fatalf("parseOnOff(%q) = (%v, %v), want (true, nil)", in, on, err)
		}
	}
	for _, in := range []string{"OFF", "0", "FALSE"} {
		if on, err := parseOnOff(in); err != nil || on {
			t.Fatalf("parseOnOff(%q) = (%v, %v), want (false, nil)", in, on, err)
		}
	}
	if _, err := parseOnOff("MAYBE"); err == nil {
		t.Fatal("parseOnOff(MAYBE) should fail")
	}
}

func TestMQTTDiscoveryEntities(t *testing.T) {
	const node = "rt-lab"
	ents := mqttDiscoveryEntities(node, "v1.2.3", "TestModel")
	if len(ents) == 0 {
		t.Fatal("no discovery entities")
	}
	seen := map[string]bool{}
	seenObj := map[string]bool{}
	for _, e := range ents {
		uid, _ := e.config["unique_id"].(string)
		if uid == "" {
			t.Fatalf("%s/%s: missing unique_id", e.component, e.objectID)
		}
		if seen[uid] {
			t.Fatalf("duplicate unique_id %q", uid)
		}
		seen[uid] = true
		seenObj[e.objectID] = true
		if got, _ := e.config["availability_topic"].(string); got != availabilityTopic(node) {
			t.Fatalf("%s: availability_topic = %q", e.objectID, got)
		}
		if _, err := json.Marshal(e.config); err != nil {
			t.Fatalf("%s: config not marshallable: %v", e.objectID, err)
		}
	}
	// The phase-1 command surface must be present.
	for _, obj := range []string{"guest_wifi", "banip", "ipv6", "sqm", "banip_reload", "reboot"} {
		if !seenObj[obj] {
			t.Fatalf("missing discovery entity %q", obj)
		}
	}
	// Switches must carry their command topic.
	for _, e := range ents {
		if e.component == "switch" {
			if got, _ := e.config["command_topic"].(string); got != "netgrip/"+node+"/command/"+e.objectID {
				t.Fatalf("switch %s command_topic = %q", e.objectID, got)
			}
		}
	}
}

func TestBuildMQTTStateMarshals(t *testing.T) {
	st := buildMQTTState("rt-lab", "v1.2.3")
	if st.Node != "rt-lab" || st.Version != "v1.2.3" || st.Ts == 0 {
		t.Fatalf("unexpected state identity: %+v", st)
	}
	if _, err := json.Marshal(st); err != nil {
		t.Fatalf("state not marshallable: %v", err)
	}
}
