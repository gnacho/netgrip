package modules

import "testing"

// #720: la lista de carpetas se escribe como media_dir y su inversa las deja
// como estaban.
func TestStoragePathsOps(t *testing.T) {
	ops, rollback := storagePathsOps("minidlna", []string{"/mnt/sda", "/mnt/sda/Music"}, []string{"/mnt"})

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
	if _, err := SetStoragePaths("nfs", []string{"/mnt/sda"}); err == nil {
		t.Fatal("nfs has no folder picker")
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

// Samba: una sección sambashare por carpeta, invitada y escribible, y la
// inversa las quita.
func TestSambaShareOps(t *testing.T) {
	ops := sambaShareOps([]string{"/mnt/sda", "/mnt/sda/Music"}, true)
	sets := map[string]string{}
	for _, op := range ops {
		if op.Kind == "uci_set" {
			sets[op.Args[0]] = op.Args[1]
		}
	}
	if sets["samba4.netgrip_sda"] != "sambashare" || sets["samba4.netgrip_sda.path"] != "/mnt/sda" {
		t.Errorf("sda share missing: %v", sets)
	}
	if sets["samba4.netgrip_sda.guest_ok"] != "yes" || sets["samba4.netgrip_sda.read_only"] != "no" {
		t.Errorf("share should be guest and writable: %v", sets)
	}
	if sets["samba4.netgrip_music.path"] != "/mnt/sda/Music" {
		t.Errorf("music share missing: %v", sets)
	}
	last := ops[len(ops)-1]
	if last.Kind != "initd" || last.Args[0] != "samba4" {
		t.Errorf("last op should restart samba4, got %+v", last)
	}
}
