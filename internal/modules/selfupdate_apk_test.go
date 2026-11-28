package modules

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// A release that publishes both the binary and the architecture packages,
// each package with its signed digest.
const releaseWithApk = `{"tag_name":"v9.9.9","body":"notes","assets":[` +
	`{"name":"netgrip-linux-arm64","browser_download_url":"http://example.com/netgrip-linux-arm64","size":1234},` +
	`{"name":"netgrip-linux-arm64.sha256","browser_download_url":"http://example.com/netgrip-linux-arm64.sha256","size":65},` +
	`{"name":"netgrip-linux-arm64.sha256.sig","browser_download_url":"http://example.com/netgrip-linux-arm64.sha256.sig","size":64},` +
	`{"name":"netgrip-9.9.9-r1-aarch64_cortex-a53.apk","browser_download_url":"http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk","size":4321},` +
	`{"name":"netgrip-9.9.9-r1-aarch64_cortex-a53.apk.sha256","browser_download_url":"http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk.sha256","size":65},` +
	`{"name":"netgrip-9.9.9-r1-aarch64_cortex-a53.apk.sha256.sig","browser_download_url":"http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk.sha256.sig","size":64},` +
	`{"name":"netgrip-9.9.9-r1-x86_64.apk","browser_download_url":"http://example.com/netgrip-9.9.9-r1-x86_64.apk","size":4321},` +
	`{"name":"netgrip-9.9.9-r1-x86_64.apk.sha256","browser_download_url":"http://example.com/netgrip-9.9.9-r1-x86_64.apk.sha256","size":65},` +
	`{"name":"netgrip-9.9.9-r1-x86_64.apk.sha256.sig","browser_download_url":"http://example.com/netgrip-9.9.9-r1-x86_64.apk.sha256.sig","size":64}]}`

// Same release with the package present but its signed digest missing: the
// self-update must not install a package it cannot verify.
const releaseWithUnsignedApk = `{"tag_name":"v9.9.9","body":"notes","assets":[` +
	`{"name":"netgrip-linux-arm64","browser_download_url":"http://example.com/netgrip-linux-arm64","size":1234},` +
	`{"name":"netgrip-linux-arm64.sha256","browser_download_url":"http://example.com/netgrip-linux-arm64.sha256","size":65},` +
	`{"name":"netgrip-linux-arm64.sha256.sig","browser_download_url":"http://example.com/netgrip-linux-arm64.sha256.sig","size":64},` +
	`{"name":"netgrip-linux-amd64","browser_download_url":"http://example.com/netgrip-linux-amd64","size":1234},` +
	`{"name":"netgrip-linux-amd64.sha256","browser_download_url":"http://example.com/netgrip-linux-amd64.sha256","size":65},` +
	`{"name":"netgrip-linux-amd64.sha256.sig","browser_download_url":"http://example.com/netgrip-linux-amd64.sha256.sig","size":64},` +
	`{"name":"netgrip-9.9.9-r1-aarch64_cortex-a53.apk","browser_download_url":"http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk","size":4321}]}`

func TestCheckSelfUpdatePicksApkAssetForPackageArch(t *testing.T) {
	withAPI(t, releaseWithApk)
	orig := apkPrintArch
	apkPrintArch = func() string { return "aarch64_cortex-a53" }
	t.Cleanup(func() { apkPrintArch = orig })

	ck := CheckSelfUpdate("0.62.0")
	if want := "http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk"; ck.ApkURL != want {
		t.Fatalf("ApkURL = %q, want %q", ck.ApkURL, want)
	}
	if ck.ApkSize != 4321 {
		t.Fatalf("ApkSize = %d, want 4321", ck.ApkSize)
	}
	if want := "http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk.sha256"; ck.ApkDigestURL != want {
		t.Fatalf("ApkDigestURL = %q, want %q", ck.ApkDigestURL, want)
	}
	if want := "http://example.com/netgrip-9.9.9-r1-aarch64_cortex-a53.apk.sha256.sig"; ck.ApkSigURL != want {
		t.Fatalf("ApkSigURL = %q, want %q", ck.ApkSigURL, want)
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

func TestCheckSelfUpdateUnsignedPackageIsNotUsed(t *testing.T) {
	withAPI(t, releaseWithUnsignedApk)
	orig := apkPrintArch
	apkPrintArch = func() string { return "aarch64_cortex-a53" }
	t.Cleanup(func() { apkPrintArch = orig })

	ck := CheckSelfUpdate("0.62.0")
	if ck.ApkURL != "" {
		t.Fatalf("ApkURL = %q, want empty for a package without a signed digest", ck.ApkURL)
	}
	if !ck.Available {
		t.Fatal("Available = false, want the signed binary to keep the update available")
	}
}

// apkTestServer serves the package and its sidecars from one handler.
func apkTestServer(t *testing.T) (*httptest.Server, map[string]string) {
	t.Helper()
	files := map[string]string{
		"/netgrip.apk":            "apk-bytes",
		"/netgrip.apk.sha256":     "digest",
		"/netgrip.apk.sha256.sig": "signature",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, files
}

func apkTarget(base string) selfUpdateTarget {
	return selfUpdateTarget{
		ApkURL:       base + "/netgrip.apk",
		ApkDigestURL: base + "/netgrip.apk.sha256",
		ApkSigURL:    base + "/netgrip.apk.sha256.sig",
	}
}

func stubAPK(t *testing.T) {
	t.Helper()
	origAdd, origPrev, origVerify := apkAddInstall, prevBinaryPath, verifyPackageRelease
	t.Cleanup(func() {
		apkAddInstall = origAdd
		prevBinaryPath = origPrev
		verifyPackageRelease = origVerify
	})
}

func TestInstallViaApkVerifiesPackageAndInstallsIt(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(currentBin, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	srv, _ := apkTestServer(t)
	stubAPK(t)
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
		return nil, nil
	}
	verifyPackageRelease = func(path string, digest, sig []byte) error {
		if filepath.Ext(path) != ".apk" {
			t.Fatalf("verified %q, want the downloaded package", path)
		}
		if string(digest) != "digest" || string(sig) != "signature" {
			t.Fatalf("digest/sig = %q/%q, want the downloaded sidecars", digest, sig)
		}
		return nil
	}

	if err := installViaApk(apkTarget(srv.URL), currentBin); err != nil {
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
	if string(got) != "old-binary" {
		t.Fatalf("binary = %q, want it untouched (apk owns the file)", got)
	}
}

func TestInstallViaApkRefusesUnverifiedPackage(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(currentBin, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	srv, _ := apkTestServer(t)
	stubAPK(t)
	prevBinaryPath = filepath.Join(dir, "prev")
	installed := false
	apkAddInstall = func(string) ([]byte, error) {
		installed = true
		return nil, nil
	}
	verifyPackageRelease = func(string, []byte, []byte) error {
		return errors.New("sha256 mismatch")
	}

	if err := installViaApk(apkTarget(srv.URL), currentBin); err == nil {
		t.Fatal("installViaApk = nil, want an error on digest mismatch")
	}
	if installed {
		t.Fatal("apk add ran with an unverified package")
	}
}

func TestInstallViaApkReportsPackageManagerFailure(t *testing.T) {
	dir := t.TempDir()
	currentBin := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(currentBin, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	srv, _ := apkTestServer(t)
	stubAPK(t)
	prevBinaryPath = filepath.Join(dir, "prev")
	apkAddInstall = func(string) ([]byte, error) {
		return []byte("ERROR: unsatisfiable constraints"), errors.New("exit status 1")
	}
	verifyPackageRelease = func(string, []byte, []byte) error { return nil }

	if err := installViaApk(apkTarget(srv.URL), currentBin); err == nil {
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
