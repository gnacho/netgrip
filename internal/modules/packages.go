package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
)

// PkgUpgrade is one package with an available upgrade.
type PkgUpgrade struct {
	Name      string `json:"name"`
	Current   string `json:"current"`
	Available string `json:"available"`
}

// OptionalPkg is one catalog entry: a service whose OpenWrt packages can be
// installed on demand (wizard) or are auto-installed on first enable.
type OptionalPkg struct {
	ID        string   `json:"id"`
	Packages  []string `json:"packages"`
	I18nKey   string   `json:"i18n_key"`
	Module    string   `json:"module"`
	Installed bool     `json:"installed"`
	// Available is false when the feeds this firmware subscribes to do not
	// offer the packages at all (a snapshot can drop a package that a release
	// still ships). Installed is false and Available false means "this router
	// cannot install it", which is not the same as "you have not installed it".
	Available bool `json:"available"`
}

var optionalCatalog = []OptionalPkg{
	{ID: "wireguard", Packages: []string{"wireguard-tools", "kmod-wireguard"}, I18nKey: "wizard.packages.wireguard", Module: "wireguard"},
	{ID: "ddns", Packages: []string{"ddns-scripts"}, I18nKey: "wizard.packages.ddns", Module: "ddns"},
	{ID: "openvpn", Packages: []string{"openvpn-openssl", "openvpn-easy-rsa"}, I18nKey: "wizard.packages.openvpn", Module: "openvpn"},
	{ID: "sqm", Packages: []string{"sqm-scripts"}, I18nKey: "wizard.packages.sqm", Module: "sqm"},
	{ID: "nlbwmon", Packages: []string{"nlbwmon"}, I18nKey: "wizard.packages.nlbwmon", Module: "nlbwmon"},
	{ID: "tailscale", Packages: []string{"tailscale"}, I18nKey: "wizard.packages.tailscale", Module: "tailscale"},
	{ID: "adguard", Packages: []string{"adguardhome"}, I18nKey: "wizard.packages.adguard", Module: "adguard"},
	{ID: "netifyd", Packages: []string{"netifyd"}, I18nKey: "wizard.packages.netifyd", Module: "netifyd"},
	{ID: "mwan3", Packages: []string{"mwan3"}, I18nKey: "wizard.packages.mwan3", Module: "multiwan"},
	{ID: "owut", Packages: []string{"owut"}, I18nKey: "wizard.packages.owut", Module: "owut"},
	{ID: "snmp", Packages: []string{"snmpd"}, I18nKey: "wizard.packages.snmp", Module: "snmp"},
	{ID: "usbstorage", Packages: []string{"kmod-usb-storage", "block-mount", "kmod-fs-ext4", "kmod-fs-vfat", "kmod-fs-exfat"}, I18nKey: "wizard.packages.usbstorage", Module: "usbstorage"},
}

// optionalServices maps catalog modules to their init.d service, stopped and
// disabled before uninstalling the packages.
var optionalServices = map[string]string{
	"wireguard": "wireguard",
	"ddns":      "ddns",
	"openvpn":   "openvpn",
	"sqm":       "sqm",
	"nlbwmon":   "nlbwmon",
	"tailscale": "tailscale",
	"adguard":   "AdGuardHome",
	"netifyd":   "netifyd",
	"multiwan":  "mwan3",
	"snmp":      "snmpd",
}

// pkgInstalled reports whether one package is installed (apk on 25.12+,
// opkg on older releases).
func pkgInstalled(name string) bool {
	if _, err := exec.LookPath("apk"); err == nil {
		return exec.Command("apk", "info", "-e", name).Run() == nil
	}
	out, err := exec.Command("opkg", "list-installed", name).Output()
	return err == nil && strings.HasPrefix(string(out), name+" ")
}

func optionalPkgInstalled(entry OptionalPkg) bool {
	for _, p := range entry.Packages {
		if !pkgInstalled(p) {
			return false
		}
	}
	return true
}

// apkAvailableFrom parses `apk search -x` output and reports which of the
// requested names the configured feeds can provide. apk prints one
// "name-version" line per match and matches variants too (snmpd answers with
// snmpd-nossl and snmpd-ssl), so the prefix check accepts those as available.
func apkAvailableFrom(names []string, out string) map[string]bool {
	res := make(map[string]bool, len(names))
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, name := range names {
			if strings.HasPrefix(line, name+"-") {
				res[name] = true
			}
		}
	}
	return res
}

// availablePackages reports which of the names the firmware feeds offer.
func availablePackages(names []string) map[string]bool {
	if len(names) == 0 {
		return map[string]bool{}
	}
	if _, err := exec.LookPath("apk"); err == nil {
		args := append([]string{"search", "-x"}, names...)
		out, _ := exec.Command("apk", args...).Output()
		return apkAvailableFrom(names, string(out))
	}
	res := make(map[string]bool, len(names))
	for _, name := range names {
		out, err := exec.Command("opkg", "list", name).Output()
		if err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), name+" ") {
			res[name] = true
		}
	}
	return res
}

func optionalPkgAvailable(entry OptionalPkg, avail map[string]bool) bool {
	for _, p := range entry.Packages {
		if !avail[p] {
			return false
		}
	}
	return true
}

// ListOptionalPackages returns the catalog with live installed and available
// flags.
func ListOptionalPackages() []OptionalPkg {
	out := make([]OptionalPkg, 0, len(optionalCatalog))
	var all []string
	for _, e := range optionalCatalog {
		all = append(all, e.Packages...)
	}
	avail := availablePackages(all)
	for _, e := range optionalCatalog {
		e.Installed = optionalPkgInstalled(e)
		e.Available = optionalPkgAvailable(e, avail)
		out = append(out, e)
	}
	return out
}

// ResolveOptionalPackageIDs validates ids against the catalog and returns the
// flat package list for those entries, skipping already-installed ones.
func ResolveOptionalPackageIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no package ids provided")
	}
	pkgs := []string{}
	for _, id := range ids {
		var entry *OptionalPkg
		for i := range optionalCatalog {
			if optionalCatalog[i].ID == id {
				entry = &optionalCatalog[i]
				break
			}
		}
		if entry == nil {
			return nil, fmt.Errorf("unknown package set: %q", id)
		}
		if optionalPkgInstalled(*entry) {
			continue
		}
		pkgs = append(pkgs, entry.Packages...)
	}
	if len(pkgs) > 0 {
		avail := availablePackages(pkgs)
		for _, p := range pkgs {
			if !avail[p] {
				return nil, fmt.Errorf("%s is not available in this firmware's feeds", p)
			}
		}
	}
	return pkgs, nil
}

// InstallOptionalPackages installs the catalog entries with the given ids.
// Ids are validated against the catalog; already installed entries are
// skipped. Returns the ids actually installed.
func InstallOptionalPackages(ids []string) ([]string, error) {
	installed := []string{}
	for _, id := range ids {
		var entry *OptionalPkg
		for i := range optionalCatalog {
			if optionalCatalog[i].ID == id {
				entry = &optionalCatalog[i]
				break
			}
		}
		if entry == nil {
			return nil, fmt.Errorf("unknown package set: %q", id)
		}
		if optionalPkgInstalled(*entry) {
			continue
		}
		if err := executor.Run(executor.Op{Kind: "pkg_add", Args: entry.Packages}); err != nil {
			return nil, fmt.Errorf("install %s: %w", id, err)
		}
		installed = append(installed, id)
	}
	return installed, nil
}

// RemoveOptionalPackages uninstalls the catalog entries with the given ids.
// Ids are validated against the catalog (whitelist: base-system packages can
// never reach this path). The associated init service is stopped and disabled
// first; /etc/config files are preserved so the choice is reversible.
func RemoveOptionalPackages(ids []string) ([]string, error) {
	removed := []string{}
	for _, id := range ids {
		var entry *OptionalPkg
		for i := range optionalCatalog {
			if optionalCatalog[i].ID == id {
				entry = &optionalCatalog[i]
				break
			}
		}
		if entry == nil {
			return nil, fmt.Errorf("unknown package set: %q", id)
		}
		if !optionalPkgInstalled(*entry) {
			continue
		}
		if svc, ok := optionalServices[entry.Module]; ok {
			if _, err := os.Stat("/etc/init.d/" + svc); err == nil {
				_ = executor.Run(executor.Op{Kind: "initd", Args: []string{svc, "stop"}})
				_ = executor.Run(executor.Op{Kind: "initd", Args: []string{svc, "disable"}})
			}
		}
		if err := executor.Run(executor.Op{Kind: "pkg_del", Args: entry.Packages}); err != nil {
			return removed, fmt.Errorf("remove %s: %w", id, err)
		}
		removed = append(removed, id)
	}
	return removed, nil
}

var rePkgLine = regexp.MustCompile(`^(\S+)-([0-9][^\s]*)\s+\S+\s+\{[^}]*\}\s+\([^)]*\)\s+\[upgradable from:\s+([^\]]+)\]$`)
var rePkgName = regexp.MustCompile(`^[a-z0-9][a-z0-9+_.-]*$`)

// pkgDenied lists base-system prefixes/exact names that must NEVER be
// upgraded piecemeal via apk (kernel, init, network core, TLS core).
// Those belong to the ASU path only.
var pkgDeniedPrefixes = []string{
	"kmod-", "kernel", "base-files", "busybox", "procd", "netifd",
	"ubus", "ubusd", "uci", "libuci", "uhttpd", "rpcd", "dnsmasq",
	"hostapd", "wpad", "firewall4", "nftables", "libnftnl",
	"libc", "libgcc", "libpthread", "librt", "libstdcpp", "ucode",
	"apk-", "opkg", "luci-base", "luci-mod-", "luci-lib", "cgi-io",
	"liblucihttp", "libmbedtls", "mbedtls-util", "ca-bundle",
	"ca-certificates", "dropbear", "iwinfo", "libiwinfo",
	"wireless-regdb", "odhcpd", "odhcp6c", "swconfig", "ethtool",
}

var pkgDeniedExact = map[string]bool{"luci": true, "kernel": true, "libc": true}

func pkgAllowed(name string) bool {
	if pkgDeniedExact[name] {
		return false
	}
	for _, p := range pkgDeniedPrefixes {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return true
}

func parseUpgradable(out string) []PkgUpgrade {
	var pkgs []PkgUpgrade
	for _, line := range strings.Split(out, "\n") {
		m := rePkgLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || !pkgAllowed(m[1]) {
			continue
		}
		pkgs = append(pkgs, PkgUpgrade{Name: m[1], Available: m[2], Current: strings.TrimPrefix(m[3], m[1]+"-")})
	}
	if pkgs == nil {
		return []PkgUpgrade{}
	}
	return pkgs
}

// ListUpgradable refreshes the apk indexes (bounded) and returns the
// user-space packages with upgrades available. On opkg (older releases)
// it falls back to `opkg list-upgradable`; if that is unavailable it
// returns an empty list instead of erroring (upgrades there go via ASU).
func ListUpgradable() ([]PkgUpgrade, error) {
	if _, err := exec.LookPath("apk"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "apk", "update").Run()
		out, err := exec.Command("apk", "list", "--upgradable").Output()
		if err != nil {
			return nil, fmt.Errorf("apk list --upgradable: %w", err)
		}
		return parseUpgradable(string(out)), nil
	}
	out, err := exec.Command("opkg", "list-upgradable").Output()
	if err != nil {
		return []PkgUpgrade{}, nil
	}
	return parseOpkgUpgradable(string(out)), nil
}

// parseOpkgUpgradable parses `opkg list-upgradable` output (one package per
// line, "name - version - description").
func parseOpkgUpgradable(out string) []PkgUpgrade {
	var pkgs []PkgUpgrade
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) == 0 {
			continue
		}
		name := f[0]
		if !rePkgName.MatchString(name) || !pkgAllowed(name) {
			continue
		}
		version := ""
		if len(f) >= 3 && f[1] == "-" {
			version = f[2]
		} else if len(f) >= 2 {
			version = f[1]
		}
		pkgs = append(pkgs, PkgUpgrade{Name: name, Available: version})
	}
	if pkgs == nil {
		return []PkgUpgrade{}
	}
	return pkgs
}

// UpgradePackage upgrades one user-space package via apk add.
// Base-system packages are refused (they belong to the ASU path).
func UpgradePackage(name string) ([]PkgUpgrade, error) {
	if !rePkgName.MatchString(name) {
		return nil, fmt.Errorf("invalid package name")
	}
	if !pkgAllowed(name) {
		return nil, fmt.Errorf("base-system package: upgrade only via ASU firmware rebuild")
	}
	current, err := ListUpgradable()
	if err != nil {
		return nil, err
	}
	found := false
	for _, p := range current {
		if p.Name == name {
			found = true
			break
		}
	}
	if !found {
		return ListUpgradable()
	}
	if err := executor.Run(executor.Op{Kind: "apk_upgrade", Args: []string{name}}); err != nil {
		return nil, fmt.Errorf("apk upgrade %s: %w", name, err)
	}
	return ListUpgradable()
}
