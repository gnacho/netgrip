package executor

import "encoding/json"

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpkgListsMissingDir(t *testing.T) {
	t.Run("empty dir means missing", func(t *testing.T) {
		dir := t.TempDir()
		if !opkgListsMissingDir(dir) {
			t.Fatalf("empty dir %q should report lists missing", dir)
		}
	})

	t.Run("dir with an index means present", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "openwrt_core"), []byte("sig\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if opkgListsMissingDir(dir) {
			t.Fatalf("dir %q with an index should report lists present", dir)
		}
	})

	t.Run("missing dir means missing", func(t *testing.T) {
		if !opkgListsMissingDir(filepath.Join(t.TempDir(), "does-not-exist")) {
			t.Fatal("nonexistent dir should report lists missing")
		}
	})
}

func TestValidatePkgAdd(t *testing.T) {
	if err := Validate(Op{Kind: "pkg_add", Args: []string{"curl"}}); err != nil {
		t.Fatalf("valid pkg_add rejected: %v", err)
	}
	if err := Validate(Op{Kind: "pkg_add", Args: nil}); err == nil {
		t.Fatal("pkg_add without packages should be rejected")
	}
	if err := Validate(Op{Kind: "pkg_add", Args: []string{"curl; rm -rf /"}}); err == nil {
		t.Fatal("pkg_add with invalid package name should be rejected")
	}
}

func TestOpUnmarshalObjectArgs(t *testing.T) {
	// uci_set with the NetPulse object form (config/section/option/value).
	var op Op
	if err := json.Unmarshal([]byte(`{"kind":"uci_set","args":{"config":"network","section":"lan","option":"ipaddr","value":"192.168.2.1"}}`), &op); err != nil {
		t.Fatal(err)
	}
	if op.Kind != "uci_set" || len(op.Args) != 2 || op.Args[0] != "network.lan.ipaddr" || op.Args[1] != "192.168.2.1" {
		t.Fatalf("got %+v", op)
	}

	// uci_commit object form.
	if err := json.Unmarshal([]byte(`{"kind":"uci_commit","args":{"config":"wireless"}}`), &op); err != nil {
		t.Fatal(err)
	}
	if len(op.Args) != 1 || op.Args[0] != "wireless" {
		t.Fatalf("uci_commit got %+v", op)
	}

	// service object form.
	if err := json.Unmarshal([]byte(`{"kind":"service","args":{"service":"dnsmasq","action":"restart"}}`), &op); err != nil {
		t.Fatal(err)
	}
	if len(op.Args) != 2 || op.Args[0] != "dnsmasq" || op.Args[1] != "restart" {
		t.Fatalf("service got %+v", op)
	}

	// Positional array form still works.
	if err := json.Unmarshal([]byte(`{"kind":"uci_set","args":["wireless.default_radio0.ssid","Test"]}`), &op); err != nil {
		t.Fatal(err)
	}
	if len(op.Args) != 2 || op.Args[0] != "wireless.default_radio0.ssid" {
		t.Fatalf("array got %+v", op)
	}
}
