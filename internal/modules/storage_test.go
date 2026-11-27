package modules

import (
	"os"
	"testing"
)

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

// #770: el probe usaba `df -B1`, que busybox no acepta, asi que todo disco
// montado volvia con tamano y uso a cero. Los bytes salen ahora de statfs.
func TestUsageFromStatMatchesDF(t *testing.T) {
	size, used, free, err := usageFromStat(15_344_992, 11_800_000, 14_991_088, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 15_344_992*1024 {
		t.Fatalf("size = %d", size)
	}
	if used != (15_344_992-11_800_000)*1024 {
		t.Fatalf("used = %d", used)
	}
	if free != 14_991_088*1024 {
		t.Fatalf("free = %d", free)
	}
}

func TestUsageFromStatRejectsAZeroBlockSize(t *testing.T) {
	if _, _, _, err := usageFromStat(10, 5, 5, 0); err == nil {
		t.Fatal("expected an error for a zero block size")
	}
}

func TestUsageFromStatClampsFreeAboveTotal(t *testing.T) {
	size, used, _, err := usageFromStat(100, 200, 0, 512)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 100*512 || used != 0 {
		t.Fatalf("size = %d, used = %d", size, used)
	}
}

func TestDiskUsageReadsARealFilesystem(t *testing.T) {
	size, _, _, err := diskUsage(os.TempDir())
	if err != nil {
		t.Fatalf("statfs on the temp dir: %v", err)
	}
	if size == 0 {
		t.Fatal("a mounted filesystem should report a size")
	}
}
