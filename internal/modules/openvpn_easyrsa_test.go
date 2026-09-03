package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEasyrsaIn(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "easyrsa")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plainPath := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(plainPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := resolveEasyrsaIn([]string{execPath}); got != execPath {
		t.Errorf("want %q, got %q", execPath, got)
	}
	if got := resolveEasyrsaIn([]string{plainPath, execPath}); got != execPath {
		t.Errorf("non-executable candidate must be skipped: got %q", got)
	}
	if got := resolveEasyrsaIn([]string{filepath.Join(dir, "missing"), plainPath}); got != "" {
		t.Errorf("want empty resolution, got %q", got)
	}
	if got := resolveEasyrsaIn([]string{dir}); got != "" {
		t.Errorf("directory candidate must be skipped: got %q", got)
	}
}

func TestOVPNMissingPkgsGLFirmwareShape(t *testing.T) {
	origInstalled, origEasyrsa := ovpnInstalledF, easyrsaBinF
	t.Cleanup(func() { ovpnInstalledF, easyrsaBinF = origInstalled, origEasyrsa })

	// GL.iNet Flint2 shape: openvpn shipped by the firmware, no easyrsa
	// anywhere (verified: openvpn-openssl installed, feeds carry no
	// openvpn-easy-rsa). easy-rsa must be installed on its own.
	ovpnInstalledF = func() bool { return true }
	easyrsaBinF = func() string { return "" }
	assertPkgs(t, ovpnMissingPkgs(), []string{"openvpn-easy-rsa"})

	// Stock router with nothing installed: both packages.
	ovpnInstalledF = func() bool { return false }
	assertPkgs(t, ovpnMissingPkgs(), []string{"openvpn-openssl", "openvpn-easy-rsa"})

	// Healthy stock router (rt3 shape): nothing to install.
	ovpnInstalledF = func() bool { return true }
	easyrsaBinF = func() string { return "/usr/bin/easyrsa" }
	assertPkgs(t, ovpnMissingPkgs(), nil)
}

func assertPkgs(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, got)
	}
}
