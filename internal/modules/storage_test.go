package modules

import "testing"

// #720: solo los servicios que la página gestiona se pueden instalar, y el
// nombre desconocido se rechaza en vez de llegar a apk/opkg.
func TestStorageServicePackages(t *testing.T) {
	if _, err := StorageServicePackages("../../bin/sh"); err == nil {
		t.Fatal("expected an error for an unknown service")
	}
	if _, err := StorageServicePackages("samba4"); err != nil {
		t.Fatalf("samba4 should be accepted: %v", err)
	}
	if _, err := StorageServicePackages("minidlna"); err != nil {
		t.Fatalf("minidlna should be accepted: %v", err)
	}
}
