package modules

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func withTempKeyStore(t *testing.T) {
	t.Helper()
	old := wgClientsPath
	wgClientsPath = filepath.Join(t.TempDir(), "wg-clients.json")
	t.Cleanup(func() { wgClientsPath = old })
}

// #756: las claves de cliente se guardan en un fichero 0600 y se puede
// actualizar y olvidar una sin tocar las demás.
func TestWGClientKeyStore(t *testing.T) {
	withTempKeyStore(t)

	if err := SaveWGClientPrivate("pub1", "priv1"); err != nil {
		t.Fatal(err)
	}
	if got := storedWGPrivate("pub1"); got != "priv1" {
		t.Fatalf("got %q", got)
	}
	info, err := os.Stat(wgClientsPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("key file is %o, want 600", perm)
	}

	if err := SaveWGClientPrivate("pub1", "priv2"); err != nil {
		t.Fatal(err)
	}
	if got := storedWGPrivate("pub1"); got != "priv2" {
		t.Fatalf("update failed, got %q", got)
	}
	if err := SaveWGClientPrivate("pub2", "other"); err != nil {
		t.Fatal(err)
	}
	if n := len(readWGClients().Clients); n != 2 {
		t.Fatalf("expected 2 clients, got %d", n)
	}

	ForgetWGClient("pub1")
	if got := storedWGPrivate("pub1"); got != "" {
		t.Fatalf("pub1 should be gone, got %q", got)
	}
	if got := storedWGPrivate("pub2"); got != "other" {
		t.Fatalf("pub2 should survive, got %q", got)
	}
}

func TestWGPeerConfigNeedsStoredKey(t *testing.T) {
	withTempKeyStore(t)
	if _, err := WGPeerConfig("no-such-peer"); !errors.Is(err, ErrNoStoredKey) {
		t.Fatalf("expected ErrNoStoredKey, got %v", err)
	}
}
