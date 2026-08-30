package modules

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setClientMetaPath(t *testing.T) string {
	t.Helper()
	old := clientMetaPath
	clientMetaPath = filepath.Join(t.TempDir(), "clients.json")
	t.Cleanup(func() {
		clientMetaPath = old
	})
	return clientMetaPath
}

// TestSetClientMetaNoDeadlock: regresión del bug de rt3 (29-Ago-2026). La
// versión original llamaba GetClientMeta() con el mutex tomado y colgaba
// cada POST /api/clients/meta (mutex no reentrante).
func TestSetClientMetaNoDeadlock(t *testing.T) {
	path := setClientMetaPath(t)

	done := make(chan struct{})
	var payload clientMetaPayload
	var err error
	go func() {
		payload, err = SetClientMeta("AA:BB:CC:DD:EE:FF", "fox", "phone")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetClientMeta se colgó: deadlock del mutex (regresión)")
	}
	if err != nil {
		t.Fatalf("SetClientMeta: %v", err)
	}
	if m, ok := payload.Meta["aa:bb:cc:dd:ee:ff"]; !ok || m.Name != "fox" || m.DeviceType != "phone" {
		t.Fatalf("payload tras set: %+v", payload.Meta)
	}

	data, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("persistencia: %v", rerr)
	}
	if len(data) == 0 {
		t.Fatal("clients.json vacío tras el set")
	}

	got := GetClientMeta()
	if m, ok := got.Meta["aa:bb:cc:dd:ee:ff"]; !ok || m.Name != "fox" {
		t.Fatalf("GetClientMeta tras set: %+v", got.Meta)
	}
}

func TestSetClientMetaInvalidMac(t *testing.T) {
	setClientMetaPath(t)
	if _, err := SetClientMeta("no-mac", "x", "phone"); err == nil {
		t.Fatal("MAC inválida debe devolver error")
	}
}

// TestSetClientMetaEmptyDeletes: pasar nombre y tipo vacíos borra la
// entrada en lugar de dejarla persistida como struct vacío (#165).
func TestSetClientMetaEmptyDeletes(t *testing.T) {
	path := setClientMetaPath(t)
	if _, err := SetClientMeta("AA:BB:CC:DD:EE:FF", "fox", "phone"); err != nil {
		t.Fatalf("set inicial: %v", err)
	}
	if _, err := SetClientMeta("AA:BB:CC:DD:EE:FF", "", ""); err != nil {
		t.Fatalf("set vacío: %v", err)
	}
	if got := GetClientMeta(); len(got.Meta) != 0 {
		t.Fatalf("tras borrar: %+v", got.Meta)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("clients.json debería haberse borrado; stat=%v", err)
	}
}

// TestClientMetaRefreshOnDiskEdit: editar el fichero a mano debe
// verse sin reiniciar el servicio (#165). La versión cacheada ocultaba
// los cambios hasta el reinicio.
func TestClientMetaRefreshOnDiskEdit(t *testing.T) {
	path := setClientMetaPath(t)
	if _, err := SetClientMeta("AA:BB:CC:DD:EE:FF", "fox", "phone"); err != nil {
		t.Fatalf("set inicial: %v", err)
	}
	// Escritura externa que simula un edit manual.
	if err := os.WriteFile(path, []byte(`{"11:22:33:44:55:66":{"name":"manual","device_type":"pc"}}`), 0o600); err != nil {
		t.Fatalf("write manual: %v", err)
	}
	got := GetClientMeta()
	if m, ok := got.Meta["11:22:33:44:55:66"]; !ok || m.Name != "manual" {
		t.Fatalf("edición manual no visible sin restart: %+v", got.Meta)
	}
	if _, ok := got.Meta["aa:bb:cc:dd:ee:ff"]; ok {
		t.Fatal("la entrada previa debería haber desaparecido tras la edición manual")
	}
}

func TestClientMetaConcurrent(t *testing.T) {
	setClientMetaPath(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = SetClientMeta("11:22:33:44:55:0"+string(rune('0'+i)), "n", "pc")
			_ = GetClientMeta()
		}(i)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("acceso concurrente bloqueado (race o deadlock)")
	}
}
