package modules

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// A release that publishes both the binary and the architecture packages.
const releaseWithApk = `{"tag_name":"v9.9.9","body":"notes","assets":[` +
	`{"name":"netgrip-linux-arm64","browser_download_url":"http://example.com/netgrip-linux-arm64","size":1234},` +
	`{"name":"netgrip-linux-arm64.sha256","browser_download_url":"http://example.com/netgrip-linux-arm64.sha256","size":65},` +
	`{"name":"netgrip-linux-arm64.sha256.sig","browser_download_url":"http://example.com/netgrip-linux-arm64.sha256.sig","size":64},` +
	`{"name":"netgrip-9.9.9-r1-aarch64_cortex-a53.apk","browser_download_url":"http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk","size":4321},` +
	`{"name":"netgrip-9.9.9-r1-x86_64.apk","browser_download_url":"http://example.com/netgrip-9.9.9-r1-x86_64.apk","size":4321}]}`

func TestCheckSelfUpdatePicksApkAssetForPackageArch(t *testing.T) {
	withAPI(t, releaseWithApk)
	orig := apkPrintArch
	apkPrintArch = func() string { return "aarch64_cortex-a53" }
	t.Cleanup(func() { apkPrintArch = orig })

	ck := CheckSelfUpdate("0.62.0")
	want := "http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk"
	if ck.ApkURL != want {
		t.Fatalf("ApkURL = %q, want %q", ck.ApkURL, want)
	}
	if ck.ApkSize != 4321 {
		t.Fatalf("ApkSize = %d, want 4321", ck.ApkSize)
	}
}

func TestCheckSelfUpdateWithoutPackageManagerHasNoApkAsset(t *testing.T) {
	withAPI(t, releaseWithApk)

	ck := CheckSelfUpdate("0.62.0")
	if ck.ApkURL != "" {
		t.Fatalf("ApkURL = %q, want empty without apk on the device", ck.ApkURL)
	}
}

func TestCheckSelfUpdateUnknownPackageArchHasNoApkAsset(t *testing.T) {
	withAPI(t, releaseWithApk)
	orig := apkPrintArch
	apkPrintArch = func() string { return "mips_24kc" }
	t.Cleanup(func() { apkPrintArch = orig })

	ck := CheckSelfUpdate("0.62.0")
	if ck.ApkURL != "" {
		t.Fatalf("ApkURL = %q, want empty when the release ships no package for this arch", ck.ApkURL)
	}
}

func TestInstallViaApkInstallsPackageAndVerifiesBinary(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(currentBin, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("apk-bytes"))
	}))
	t.Cleanup(srv.Close)

	origAdd, origPrev, origVerify := apkAddInstall, prevBinaryPath, verifyInstalledRelease
	t.Cleanup(func() {
		apkAddInstall = origAdd
		prevBinaryPath = origPrev
		verifyInstalledRelease = origVerify
	})
	prevBinaryPath = filepath.Join(dir, "prev")

	var gotPath string
	apkAddInstall = func(apkPath string) ([]byte, error) {
		gotPath = apkPath
		data, err := os.ReadFile(apkPath)
		if err != nil {
			return nil, err
		}
		if string(data) != "apk-bytes" {
			return nil, errUnexpectedApk
		}
		if err := os.WriteFile(currentBin, []byte("new-binary"), 0755); err != nil {
			return nil, err
		}
		return nil, nil
	}
	verifyInstalledRelease = func(path string, digest, sig []byte) error {
		if path != currentBin {
			t.Fatalf("verified %q, want %q", path, currentBin)
		}
		return nil
	}

	if err := installViaApk(srv.URL, []byte("digest"), []byte("sig"), currentBin); err != nil {
		t.Fatalf("installViaApk: %v", err)
	}
	if gotPath == "" {
		t.Fatal("apk add was not called")
	}
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Fatalf("downloaded apk was not cleaned up: %v", err)
	}
	got, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary = %q, want the one apk installed", got)
	}
}

func TestInstallViaApkRestoresPreviousBinaryOnDigestMismatch(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(currentBin, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("apk-bytes"))
	}))
	t.Cleanup(srv.Close)

	origAdd, origPrev, origVerify := apkAddInstall, prevBinaryPath, verifyInstalledRelease
	t.Cleanup(func() {
		apkAddInstall = origAdd
		prevBinaryPath = origPrev
		verifyInstalledRelease = origVerify
	})
	prevBinaryPath = filepath.Join(dir, "prev")
	apkAddInstall = func(string) ([]byte, error) {
		return nil, os.WriteFile(currentBin, []byte("tampered"), 0755)
	}
	verifyInstalledRelease = func(string, []byte, []byte) error {
		return errors.New("sha256 mismatch")
	}

	if err := installViaApk(srv.URL, []byte("digest"), []byte("sig"), currentBin); err == nil {
		t.Fatal("installViaApk = nil, want an error on digest mismatch")
	}
	got, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary = %q, want the previous binary restored", got)
	}
}

func TestInstallViaApkReportsPackageManagerFailure(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(currentBin, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("apk-bytes"))
	}))
	t.Cleanup(srv.Close)

	origAdd, origPrev := apkAddInstall, prevBinaryPath
	t.Cleanup(func() {
		apkAddInstall = origAdd
		prevBinaryPath = origPrev
	})
	prevBinaryPath = filepath.Join(dir, "prev")
	apkAddInstall = func(string) ([]byte, error) {
		return []byte("ERROR: unsatisfiable constraints"), errors.New("exit status 1")
	}

	if err := installViaApk(srv.URL, []byte("digest"), []byte("sig"), currentBin); err == nil {
		t.Fatal("installViaApk = nil, want the apk failure")
	}
	got, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Fatalf("binary = %q, want it untouched after a failed install", got)
	}
}

var errUnexpectedApk = errors.New("unexpected apk payload")
