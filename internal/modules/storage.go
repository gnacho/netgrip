package modules

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type StorageDevice struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FSType     string `json:"fs_type"`
	SizeBytes  uint64 `json:"size_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	MountPoint string `json:"mount_point,omitempty"`
}

type StorageService struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Enabled bool   `json:"enabled"`
}

type StorageProbe struct {
	Applicable bool             `json:"applicable"`
	Devices    []StorageDevice  `json:"devices"`
	Services   []StorageService `json:"services"`
}

func ProbeStorage() StorageProbe {
	devices := detectUSBStorage()
	if len(devices) == 0 {
		return StorageProbe{Applicable: false}
	}

	services := []StorageService{
		probeStorageService("samba4"),
		probeStorageService("minidlna"),
	}

	return StorageProbe{
		Applicable: true,
		Devices:    devices,
		Services:   services,
	}
}

func detectUSBStorage() []StorageDevice {
	var devices []StorageDevice

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return devices
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "sd") {
			continue
		}

		devPath := filepath.Join("/sys/block", name, "device")
		if _, err := os.Stat(devPath); err != nil {
			continue
		}

		dev := StorageDevice{Name: name, Path: "/dev/" + name}

		sizePath := filepath.Join("/sys/block", name, "size")
		if data, err := os.ReadFile(sizePath); err == nil {
			sectors, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			if err == nil {
				dev.SizeBytes = sectors * 512
			}
		}

		partName := name + "1"
		partPath := filepath.Join("/sys/block", name, partName)
		if _, err := os.Stat(partPath); err == nil {
			dev.Name = partName
			dev.Path = "/dev/" + partName
		}

		mounts, _ := os.Open("/proc/mounts")
		if mounts != nil {
			scanner := bufio.NewScanner(mounts)
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) >= 3 && fields[0] == dev.Path {
					dev.MountPoint = fields[1]
					dev.FSType = fields[2]
					break
				}
			}
			mounts.Close()
		}

		if dev.MountPoint != "" {
			cmd := exec.Command("df", "-B1", dev.MountPoint)
			out, err := cmd.Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				if len(lines) >= 2 {
					fields := strings.Fields(lines[1])
					if len(fields) >= 4 {
						if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							dev.SizeBytes = v
						}
						if v, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
							dev.UsedBytes = v
						}
						if v, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
							dev.FreeBytes = v
						}
					}
				}
			}
		}

		devices = append(devices, dev)
	}

	return devices
}

func probeStorageService(name string) StorageService {
	svc := StorageService{Name: name}

	out, err := exec.Command("/etc/init.d/"+name, "enabled").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "" {
		svc.Enabled = true
	}

	out, err = exec.Command("pidof", name).CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		svc.Running = true
	}

	return svc
}

func SetStorageService(name string, action string) error {
	if action != "enable" && action != "disable" {
		return nil
	}

	initPath := "/etc/init.d/" + name
	if _, err := os.Stat(initPath); err != nil {
		return err
	}

	exec.Command(initPath, action).Run()

	if action == "enable" {
		exec.Command(initPath, "restart").Run()
	} else {
		exec.Command(initPath, "stop").Run()
	}

	return nil
}
