package modules

import "testing"

func TestStorageSectionName(t *testing.T) {
	cases := map[string]string{
		"/mnt/sda":      "netgrip_sda",
		"/mnt/Backup-1": "netgrip_backup_1",
		"/mnt/usb_disk": "netgrip_usb_disk",
	}
	for target, want := range cases {
		if got := StorageSectionName(target); got != want {
			t.Errorf("StorageSectionName(%q) = %q, want %q", target, got, want)
		}
	}
	if got := StorageSectionName("/mnt/"); got != "netgrip_usb" {
		t.Errorf("empty name should fall back to usb, got %q", got)
	}
}

func TestParseBlockInfo(t *testing.T) {
	out := `/dev/mtdblock21: UUID="118234562" VERSION="1" TYPE="ubi"
/dev/sda: UUID="78CC-80F9" VERSION="FAT32" TYPE="vfat"
/dev/sdb1: UUID="0f3a-1111" VERSION="FAT32" MOUNT="/mnt/b" TYPE="vfat"
`
	if uuid, fstype := ParseBlockInfo(out, "/dev/sda"); uuid != "78CC-80F9" || fstype != "vfat" {
		t.Fatalf("sda: got %q %q", uuid, fstype)
	}
	if uuid, fstype := ParseBlockInfo(out, "/dev/sdb1"); uuid != "0f3a-1111" || fstype != "vfat" {
		t.Fatalf("sdb1: got %q %q", uuid, fstype)
	}
	if uuid, fstype := ParseBlockInfo(out, "/dev/sdz"); uuid != "" || fstype != "" {
		t.Fatalf("unknown device should return blanks, got %q %q", uuid, fstype)
	}
}

func TestStorageMountOps(t *testing.T) {
	ops := storageMountOps("/mnt/usb", "78CC-80F9", false)
	want := map[string]string{
		"fstab.netgrip_usb":         "mount",
		"fstab.netgrip_usb.target":  "/mnt/usb",
		"fstab.netgrip_usb.uuid":    "78CC-80F9",
		"fstab.netgrip_usb.enabled": "1",
		"fstab.netgrip_usb.options": "rw,iocharset=utf8",
	}
	seen := map[string]string{}
	for _, op := range ops {
		if op.Kind == "uci_set" {
			seen[op.Args[0]] = op.Args[1]
		}
	}
	for k, v := range want {
		if seen[k] != v {
			t.Errorf("%s = %q, want %q", k, seen[k], v)
		}
	}
	if ops[len(ops)-1].Kind != "uci_commit" || ops[len(ops)-1].Args[0] != "fstab" {
		t.Errorf("last op should commit fstab, got %+v", ops[len(ops)-1])
	}

	ro := storageMountOps("/mnt/usb", "78CC-80F9", true)
	for _, op := range ro {
		if op.Kind == "uci_set" && op.Args[0] == "fstab.netgrip_usb.options" && op.Args[1] != "ro,iocharset=utf8" {
			t.Errorf("read-only options = %q", op.Args[1])
		}
	}
}

// Los nombres raros se rechazan antes de tocar nada.
func TestMountStorageValidation(t *testing.T) {
	if _, err := MountStorage(StorageMountRequest{Device: "/dev/../etc/passwd", Target: "/mnt/x"}); err == nil {
		t.Fatal("expected a device validation error")
	}
	if _, err := MountStorage(StorageMountRequest{Device: "/dev/sda", Target: "/etc"}); err == nil {
		t.Fatal("expected a target validation error")
	}
	if _, err := UnmountStorage("/etc"); err == nil {
		t.Fatal("expected an unmount target validation error")
	}
}
