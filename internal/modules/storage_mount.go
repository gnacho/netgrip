package modules

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/gnacho/netgrip/internal/executor"
)

// Montaje de discos desde el panel (#739). La entrada de /etc/config/fstab
// se escribe con las ops allowlisted del executor (snapshot incluido) y el
// montaje inmediato se hace con mount(2): no hay shell de por medio y el
// sistema de ficheros no se toca.

var (
	reStorageDevice = regexp.MustCompile(`^/dev/sd[a-z][0-9]*$`)
	reStorageTarget = regexp.MustCompile(`^/mnt/[A-Za-z0-9._-]+$`)
	reBlockInfoLine = regexp.MustCompile(`^(\S+):.*UUID="([^"]*)".*TYPE="([^"]*)"`)
)

// StorageMountRequest is what the panel sends to mount a detected disk.
type StorageMountRequest struct {
	Device   string `json:"device"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

// StorageSectionName returns the stable UCI section for a mount target, so a
// second mount of the same target updates the entry instead of stacking one.
func StorageSectionName(target string) string {
	name := strings.TrimPrefix(target, "/mnt/")
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		default:
			b.WriteRune('_')
		}
	}
	clean := strings.Trim(b.String(), "_")
	if clean == "" {
		clean = "usb"
	}
	return "netgrip_" + clean
}

// ParseBlockInfo pulls the UUID and filesystem type of one device out of the
// `block info` output (block-mount).
func ParseBlockInfo(out, device string) (uuid, fstype string) {
	for _, line := range strings.Split(out, "\n") {
		m := reBlockInfoLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || m[1] != device {
			continue
		}
		return m[2], m[3]
	}
	return "", ""
}

func blockInfoFor(device string) (uuid, fstype string, err error) {
	out, err := exec.Command("/sbin/block", "info").Output()
	if err != nil {
		return "", "", fmt.Errorf("block info: %w", err)
	}
	uuid, fstype = ParseBlockInfo(string(out), device)
	if uuid == "" || fstype == "" {
		return "", "", fmt.Errorf("%s reports no UUID or filesystem", device)
	}
	return uuid, fstype, nil
}

// storageMountOps writes the fstab entry. Kept pure so the keys and values
// can be asserted in a test.
func storageMountOps(target, uuid string, readOnly bool) []executor.Op {
	sec := "fstab." + StorageSectionName(target)
	opts := "rw,iocharset=utf8"
	if readOnly {
		opts = "ro,iocharset=utf8"
	}
	return []executor.Op{
		{Kind: "uci_set", Args: []string{sec, "mount"}},
		{Kind: "uci_set", Args: []string{sec + ".target", target}},
		{Kind: "uci_set", Args: []string{sec + ".uuid", uuid}},
		{Kind: "uci_set", Args: []string{sec + ".enabled", "1"}},
		{Kind: "uci_set", Args: []string{sec + ".options", opts}},
		{Kind: "uci_commit", Args: []string{"fstab"}},
	}
}

func storageUnmountOps(target string) []executor.Op {
	return []executor.Op{
		{Kind: "uci_delete", Args: []string{"fstab." + StorageSectionName(target)}},
		{Kind: "uci_commit", Args: []string{"fstab"}},
	}
}

// storageMountManaged reports whether the fstab entry for target is one this
// panel created, so it never deletes a mount configured by hand.
func storageMountManaged(target string) bool {
	out, err := exec.Command("uci", "-q", "show", "fstab").Output()
	if err != nil {
		return false
	}
	sec := "fstab." + StorageSectionName(target)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, sec+".target=") {
			return true
		}
	}
	return false
}

// mountPointOf returns where device is mounted right now, or "".
func mountPointOf(device string) string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 64*1024)
	n, _ := f.Read(buf)
	for _, line := range strings.Split(string(buf[:n]), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == device {
			return fields[1]
		}
	}
	return ""
}

// MountStorage mounts a detected disk now and leaves the fstab entry so it
// comes back after a reboot. Read-only is honoured on both.
func MountStorage(req StorageMountRequest) (StorageProbe, error) {
	if !reStorageDevice.MatchString(req.Device) {
		return ProbeStorage(), fmt.Errorf("invalid device %q", req.Device)
	}
	if !reStorageTarget.MatchString(req.Target) {
		return ProbeStorage(), fmt.Errorf("invalid mount point %q", req.Target)
	}
	if current := mountPointOf(req.Device); current != "" {
		if current == req.Target {
			return ProbeStorage(), nil
		}
		return ProbeStorage(), fmt.Errorf("%s is already mounted at %s", req.Device, current)
	}
	uuid, fstype, err := blockInfoFor(req.Device)
	if err != nil {
		return ProbeStorage(), err
	}
	if err := os.MkdirAll(req.Target, 0755); err != nil {
		return ProbeStorage(), err
	}
	if err := executor.Apply(storageMountOps(req.Target, uuid, req.ReadOnly), nil); err != nil {
		return ProbeStorage(), err
	}

	flags := uintptr(0)
	data := "iocharset=utf8"
	if req.ReadOnly {
		flags = syscall.MS_RDONLY
	}
	if err := syscall.Mount(req.Device, req.Target, fstype, flags, data); err != nil {
		// The entry would fail again at boot: take it out.
		_ = executor.Apply(storageUnmountOps(req.Target), nil)
		return ProbeStorage(), fmt.Errorf("mount %s: %w", req.Target, err)
	}
	InvalidateKey("storage")
	return ProbeStorage(), nil
}

// UnmountStorage unmounts and removes the fstab entry, but only for mounts
// this panel created.
func UnmountStorage(target string) (StorageProbe, error) {
	if !reStorageTarget.MatchString(target) {
		return ProbeStorage(), fmt.Errorf("invalid mount point %q", target)
	}
	if !storageMountManaged(target) {
		return ProbeStorage(), fmt.Errorf("%s was not mounted from this panel", target)
	}
	if err := syscall.Unmount(target, 0); err != nil {
		return ProbeStorage(), fmt.Errorf("unmount %s: %w", target, err)
	}
	if err := executor.Apply(storageUnmountOps(target), nil); err != nil {
		return ProbeStorage(), err
	}
	InvalidateKey("storage")
	return ProbeStorage(), nil
}
