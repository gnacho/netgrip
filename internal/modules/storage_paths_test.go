package modules

import "testing"

// #720: la lista de carpetas se escribe como media_dir y su inversa las deja
// como estaban.
func TestStoragePathsOps(t *testing.T) {
	ops, rollback := storagePathsOps([]string{"/mnt/sda", "/mnt/sda/Music"}, []string{"/mnt"})

	adds := 0
	for _, op := range ops {
		if op.Kind == "uci_add_list" {
			adds++
			if op.Args[0] != "minidlna.config.media_dir" {
				t.Errorf("add_list on %q", op.Args[0])
			}
		}
	}
	if adds != 2 {
		t.Errorf("expected 2 add_list ops, got %d", adds)
	}
	last := ops[len(ops)-1]
	if last.Kind != "initd" || last.Args[0] != "minidlna" || last.Args[1] != "restart" {
		t.Errorf("last op should restart minidlna, got %+v", last)
	}
	if rollback[len(rollback)-1].Kind != "initd" {
		t.Errorf("rollback should restart too, got %+v", rollback[len(rollback)-1])
	}
	oldAdds := 0
	for _, op := range rollback {
		if op.Kind == "uci_add_list" && op.Args[1] == "/mnt" {
			oldAdds++
		}
	}
	if oldAdds != 1 {
		t.Errorf("rollback should restore /mnt, got %d", oldAdds)
	}
}

func TestSetStoragePathsValidation(t *testing.T) {
	if _, err := SetStoragePaths("samba4", []string{"/mnt/sda"}); err == nil {
		t.Fatal("samba4 has no folder picker yet")
	}
	if _, err := SetStoragePaths("minidlna", nil); err == nil {
		t.Fatal("empty list must be rejected")
	}
	if _, err := SetStoragePaths("minidlna", []string{"/etc"}); err == nil {
		t.Fatal("paths outside /mnt must be rejected")
	}
	if _, err := SetStoragePaths("minidlna", []string{"/mnt/does-not-exist"}); err == nil {
		t.Fatal("missing folders must be rejected")
	}
}
