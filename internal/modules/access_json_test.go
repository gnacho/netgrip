package modules

import (
	"encoding/json"
	"strings"
	"testing"
)

// El contrato usa "web" (#750): el nombre viejo no debe volver por descuido.
func TestAccessProbeJSONUsesWebKey(t *testing.T) {
	b, err := json.Marshal(AccessProbe{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"web"`) {
		t.Fatalf("expected a web key, got %s", b)
	}
	if strings.Contains(string(b), `"luci"`) {
		t.Fatalf("stale key in %s", b)
	}
}

func TestSQMProbeJSONUsesWebUIAppKey(t *testing.T) {
	b, err := json.Marshal(SQMProbe{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"web_ui_app"`) {
		t.Fatalf("expected web_ui_app, got %s", b)
	}
}
