package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gnacho/netgrip/internal/executor"
)

type StorageDevice struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FSType     string `json:"fs_type"`
	SizeBytes  uint64 `json:"size_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	MountPoint string `json:"mount_point,omitempty"`
	// Managed: la entrada de fstab la creó este panel, así que se puede
	// desmontar desde aquí sin pisar una configurada a mano (#739).
	Managed bool `json:"managed"`
}

type StorageService struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Enabled bool   `json:"enabled"`
	// Installed: el paquete está puesto (existe su init script). Sin él la
	// página ofrece instalarlo en vez de un interruptor muerto (#720).
	Installed bool `json:"installed"`
}

// storageServiceUCI maps a service to the UCI switch its init script reads
// before starting. minidlna refuses to run with enabled=0 even when the init
// script itself is enabled, so toggling only the symlink left it "on" and
// dead (#747).
var storageServiceUCI = map[string]string{
	"minidlna": "minidlna.config.enabled",
}

// storageServiceProcesses lists the names to try with pidof: the PROG the
// init script launches (minidlnad), then the service name.
func storageServiceProcesses(name string) []string {
	if data, err := os.ReadFile("/etc/init.d/" + name); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "PROG=") {
				continue
			}
			prog := strings.Trim(strings.TrimPrefix(line, "PROG="), "\"'")
			if base := filepath.Base(prog); base != "" && base != "." && base != "/" {
				return []string{base, name, name + "d"}
			}
		}
	}
	return []string{name, name + "d"}
}

// storageServicePackages maps the services the page manages to the package
// that provides them.
var storageServicePackages = map[string]string{
	"samba4":   "samba4",
	"minidlna": "minidlna",
}

// StorageServicePackages returns the packages to install for name, validating
// it against the services the page manages.
func StorageServicePackages(name string) ([]string, error) {
	pkg, ok := storageServicePackages[name]
	if !ok {
		return nil, fmt.Errorf("unknown storage service %q", name)
	}
	if _, err := os.Stat("/etc/init.d/" + name); err == nil {
		return nil, nil
	}
	return []string{pkg}, nil
}

type StorageProbe struct {
	Applicable bool `json:"applicable"`
	// Supported: el equipo tiene buses USB, aunque no haya nada enchufado.
	// Antes Applicable exigía un disco y la página decía "no gestionable" en
	// un router con puerto USB, sin llegar a invitar a conectar nada (#636).
	Supported bool `json:"supported"`
	// USBPresent: hay un dispositivo USB conectado (no un hub).
	USBPresent bool `json:"usb_present"`
	// USBStorageLoaded: el kernel tiene el controlador de almacenamiento USB.
	// Sin él el disco no aparece como /dev/sd*, y la página lo dice en vez de
	// fingir que no hay nada enchufado (#737).
	USBStorageLoaded bool             `json:"usb_storage_loaded"`
	Devices          []StorageDevice  `json:"devices"`
	Services         []StorageService `json:"services"`
}

// usbHostPresent reports whether the kernel registered any USB bus: un router
// con puerto USB los tiene aunque esté vacío.
func usbHostPresent() bool {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "usb") {
			return true
		}
	}
	return false
}

// usbDevicePresent reports a real USB device (not a root hub) plugged in.
func usbDevicePresent() bool {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !strings.Contains(e.Name(), "-") || strings.Contains(e.Name(), ":") {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/bus/usb/devices", e.Name(), "idVendor")); err == nil {
			return true
		}
	}
	return false
}

// usbStorageLoaded reports whether the kernel has the USB mass storage driver.
func usbStorageLoaded() bool {
	_, err := os.Stat("/sys/module/usb_storage")
	return err == nil
}

func ProbeStorage() StorageProbe {
	devices := detectUSBStorage()
	supported := usbHostPresent()
	if len(devices) == 0 {
		return StorageProbe{
			Applicable:       supported, // con puerto USB la página se muestra e invita a conectar
			Supported:        supported,
			USBPresent:       usbDevicePresent(),
			USBStorageLoaded: usbStorageLoaded(),
			Devices:          []StorageDevice{},
			Services:         []StorageService{},
		}
	}

	services := []StorageService{
		probeStorageService("samba4"),
		probeStorageService("minidlna"),
	}

	return StorageProbe{
		Applicable:       true,
		Supported:        true,
		USBPresent:       true,
		USBStorageLoaded: usbStorageLoaded(),
		Devices:          devices,
		Services:         services,
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
			dev.Managed = storageMountManaged(dev.MountPoint)
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

	if _, err := os.Stat("/etc/init.d/" + name); err == nil {
		svc.Installed = true
	}

	out, err := exec.Command("/etc/init.d/"+name, "enabled").CombinedOutput()
	initEnabled := err == nil && strings.TrimSpace(string(out)) == ""
	if key, ok := storageServiceUCI[name]; ok {
		// Arrancará al bootear solo si el switch propio también está a 1.
		svc.Enabled = initEnabled && uciGet(key) == "1"
	} else {
		svc.Enabled = initEnabled
	}

	for _, proc := range storageServiceProcesses(name) {
		out, err = exec.Command("pidof", proc).CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			svc.Running = true
			break
		}
	}

	return svc
}

func SetStorageService(name string, action string) error {
	InvalidateKey("storage")
	if action != "enable" && action != "disable" {
		return nil
	}

	initPath := "/etc/init.d/" + name
	if _, err := os.Stat(initPath); err != nil {
		return err
	}

	// Sin este switch el servicio queda "encendido" y muerto (#747).
	if key, ok := storageServiceUCI[name]; ok {
		val := "0"
		if action == "enable" {
			val = "1"
		}
		cfg := strings.SplitN(key, ".", 2)[0]
		ops := []executor.Op{
			{Kind: "uci_set", Args: []string{key, val}},
			{Kind: "uci_commit", Args: []string{cfg}},
		}
		if err := executor.Apply(ops, nil); err != nil {
			return err
		}
	}

	exec.Command(initPath, action).Run()

	if action == "enable" {
		exec.Command(initPath, "restart").Run()
	} else {
		exec.Command(initPath, "stop").Run()
	}

	return nil
}
