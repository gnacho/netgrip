package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un fichero de lista vacio (o solo con comentarios) dejaba el slice a nil:
// json lo serializa como null y la pagina de banIP hace entries.length sobre
// el, tirando el panel entero con el error boundary (#616).
func TestBanipReadListNeverNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banip.blocklist")
	if err := os.WriteFile(path, []byte("\n# solo comentarios\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := banipReadList(path)
	if got == nil {
		t.Fatal("banipReadList devolvio nil: saldria null en JSON")
	}
	if len(got) != 0 {
		t.Fatalf("entradas = %v, want vacio", got)
	}

	raw, err := json.Marshal(struct {
		Allowlist []string `json:"allowlist"`
		Blocklist []string `json:"blocklist"`
	}{got, got})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("las listas salen null: %s", raw)
	}
}

// La ruta "no instalado" sale sin forkar nada y es la que mas facil se
// queda con slices nil: se comprueba el contrato JSON entero.
func TestBanipProbeHasNoNullSlices(t *testing.T) {
	raw, err := json.Marshal(ProbeBanIP())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("el probe lleva null: %s", raw)
	}
}

func TestBanipFeedsNeverNil(t *testing.T) {
	got := banipFeeds(map[string][]string{})
	if got == nil {
		t.Fatal("banipFeeds devolvio nil: saldria null en JSON")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != "[]" {
		t.Fatalf("feeds = %s, want []", raw)
	}
}
