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
	clientMetaMu.Lock()
	clientMetaData = nil
	clientMetaMu.Unlock()
	t.Cleanup(func() {
		clientMetaPath = old
		clientMetaMu.Lock()
		clientMetaData = nil
		clientMetaMu.Unlock()
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
