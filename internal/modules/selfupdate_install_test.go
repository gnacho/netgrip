package modules

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// installSelfUpdate must replace the target with a single copy's worth of
// space on the target filesystem and keep the rescue copy on tmpfs (#528):
// the previous on-overlay .bak was never consumed by any rollback path and
// its dead weight is what eventually made updates fail with ENOSPC.
func TestInstallSelfUpdateReplacesBinary(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "netgrip")
	if err := os.WriteFile(current, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	staged := filepath.Join(t.TempDir(), "netgrip.update")
	if err := os.WriteFile(staged, []byte("new-binary"), 0644); err != nil {
		t.Fatal(err)
	}

	prev := filepath.Join(t.TempDir(), "netgrip.prev")
	origPrev := prevBinaryPath
	prevBinaryPath = prev
	t.Cleanup(func() { prevBinaryPath = origPrev })

	if err := installSelfUpdate(staged, current); err != nil {
		t.Fatalf("installSelfUpdate: %v", err)
	}

	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("installed binary = %q, want new content", got)
	}
	fi, err := os.Stat(current)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0755 {
		t.Fatalf("installed mode = %o, want 0755", fi.Mode().Perm())
	}

	rescue, err := os.ReadFile(prev)
	if err != nil {
		t.Fatalf("rescue copy: %v", err)
	}
	if string(rescue) != "old-binary" {
		t.Fatalf("rescue copy = %q, want old content", rescue)
	}
}

// The space pre-check must count the current binary's size towards the
// budget, because the install unlinks it before copying the new one (#547):
// on a tiny overlay the free space alone is always smaller than the new
// binary, and demanding it up front makes every self-update fail.
func TestSpaceForCountsFreedBytes(t *testing.T) {
	dir := t.TempDir()
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil || st.Bsize <= 0 {
		t.Skip("statfs not available")
	}
	avail := uint64(st.Bavail) * uint64(st.Bsize)

	need := avail + 10<<20
	if spaceFor(dir, need, 0) {
		t.Fatalf("spaceFor(need=%d, freed=0) = true, want false", need)
	}
	if !spaceFor(dir, need, 20<<20) {
		t.Fatalf("spaceFor(need=%d, freed=%d) = false, want true", need, 20<<20)
	}
	// The 1MiB headroom is still required on top of avail+freed.
	if spaceFor(dir, avail+20<<20, 20<<20) {
		t.Fatal("spaceFor without the 1MiB headroom = true, want false")
	}
}

// A missing current binary (e.g. a previous partial install) must not
// block the install: the rescue copy fails, is logged, and the update
// continues.
func TestInstallSelfUpdateMissingCurrent(t *testing.T) {
	current := filepath.Join(t.TempDir(), "netgrip")

	staged := filepath.Join(t.TempDir(), "netgrip.update")
	if err := os.WriteFile(staged, []byte("new-binary"), 0644); err != nil {
		t.Fatal(err)
	}

	prev := filepath.Join(t.TempDir(), "netgrip.prev")
	origPrev := prevBinaryPath
	prevBinaryPath = prev
	t.Cleanup(func() { prevBinaryPath = origPrev })

	if err := installSelfUpdate(staged, current); err != nil {
		t.Fatalf("installSelfUpdate: %v", err)
	}
	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("installed binary = %q, want new content", got)
	}
}
