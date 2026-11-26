package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	// Available: el paquete existe en los repos del router. Sin esto la
	// página ofrecía instalar Samba en firmwares que no lo traen (#720).
	Available bool `json:"available"`
	// Paths: carpetas que comparte el servicio (media_dir de minidlna).
	Paths []string `json:"paths"`
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

// storagePackageAvailable reports whether the router feeds carry the package.
func storagePackageAvailable(pkg string) bool {
	if pkg == "" {
		return false
	}
	if _, err := exec.LookPath("apk"); err == nil {
		out, err := exec.Command("apk", "search", "-x", pkg).Output()
		return err == nil && strings.TrimSpace(string(out)) != ""
	}
	out, err := exec.Command("opkg", "list", pkg).Output()
	return err == nil && strings.Contains(string(out), pkg)
}

// storageServicePaths returns the folders a service shares.
func storageServicePaths(name string) []string {
	if name != "minidlna" {
		return []string{}
	}
	out, err := exec.Command("uci", "-q", "get", "minidlna.config.media_dir").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(strings.ReplaceAll(string(out), "\n", " "))
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

// reStoragePath acepta carpetas dentro de /mnt, con subcarpetas.
var reStoragePath = regexp.MustCompile(`^/mnt(/[A-Za-z0-9._-]+)+$`)

// SetStoragePaths replaces the folders a service shares (#720). Today only
// minidlna exposes a folder list (media_dir).
func SetStoragePaths(name string, paths []string) (StorageProbe, error) {
	if name != "minidlna" {
		return ProbeStorage(), fmt.Errorf("folder selection is not available for %q", name)
	}
	if len(paths) == 0 {
		return ProbeStorage(), fmt.Errorf("pick at least one folder")
	}
	for _, p := range paths {
		if !reStoragePath.MatchString(p) {
			return ProbeStorage(), fmt.Errorf("invalid folder %q", p)
		}
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			return ProbeStorage(), fmt.Errorf("%s is not a folder on this router", p)
		}
	}

	old := storageServicePaths(name)
	ops, rollback := storagePathsOps(paths, old)
	if err := executor.Apply(ops, rollback); err != nil {
		return ProbeStorage(), err
	}
	InvalidateKey("storage")

	// minidlna no arranca si una carpeta no existe: si no volvió, se restaura.
	time.Sleep(2 * time.Second)
	if !probeStorageService(name).Running {
		_ = executor.Apply(rollback, nil)
		return ProbeStorage(), fmt.Errorf("minidlna did not come back with those folders; previous ones restored")
	}
	return ProbeStorage(), nil
}

// storagePathsOps builds the ops and their inverse for the folder list.
func storagePathsOps(paths, old []string) (ops, rollback []executor.Op) {
	ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"minidlna.config.media_dir"}})
	rollback = append(rollback, executor.Op{Kind: "uci_delete", Args: []string{"minidlna.config.media_dir"}})
	for _, p := range paths {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{"minidlna.config.media_dir", p}})
	}
	for _, p := range old {
		rollback = append(rollback, executor.Op{Kind: "uci_add_list", Args: []string{"minidlna.config.media_dir", p}})
	}
	for _, set := range []*[]executor.Op{&ops, &rollback} {
		*set = append(*set,
			executor.Op{Kind: "uci_commit", Args: []string{"minidlna"}},
			executor.Op{Kind: "initd", Args: []string{"minidlna", "restart"}},
		)
	}
	return ops, rollback
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
	svc.Available = svc.Installed || storagePackageAvailable(storageServicePackages[name])
	svc.Paths = storageServicePaths(name)

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
