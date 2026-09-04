package executor

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
