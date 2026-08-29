package modules

import (
	"context"
	"fmt"
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
}

var optionalCatalog = []OptionalPkg{
	{ID: "wireguard", Packages: []string{"wireguard-tools", "kmod-wireguard"}, I18nKey: "wizard.packages.wireguard", Module: "wireguard"},
	{ID: "ddns", Packages: []string{"ddns-scripts"}, I18nKey: "wizard.packages.ddns", Module: "ddns"},
	{ID: "openvpn", Packages: []string{"openvpn-openssl", "openvpn-easy-rsa"}, I18nKey: "wizard.packages.openvpn", Module: "openvpn"},
	{ID: "sqm", Packages: []string{"sqm-scripts"}, I18nKey: "wizard.packages.sqm", Module: "sqm"},
	{ID: "nlbwmon", Packages: []string{"nlbwmon"}, I18nKey: "wizard.packages.nlbwmon", Module: "nlbwmon"},
	{ID: "nft-qos", Packages: []string{"nft-qos"}, I18nKey: "wizard.packages.nftqos", Module: "nftqos"},
	{ID: "tailscale", Packages: []string{"tailscale"}, I18nKey: "wizard.packages.tailscale", Module: "tailscale"},
	{ID: "adguard", Packages: []string{"adguardhome"}, I18nKey: "wizard.packages.adguard", Module: "adguard"},
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

// ListOptionalPackages returns the catalog with live installed flags.
func ListOptionalPackages() []OptionalPkg {
	out := make([]OptionalPkg, 0, len(optionalCatalog))
	for _, e := range optionalCatalog {
		e.Installed = optionalPkgInstalled(e)
		out = append(out, e)
	}
	return out
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
// user-space packages with upgrades available.
func ListUpgradable() ([]PkgUpgrade, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "apk", "update").Run()
	out, err := exec.Command("apk", "list", "--upgradable").Output()
	if err != nil {
		return nil, fmt.Errorf("apk list --upgradable: %w", err)
	}
	return parseUpgradable(string(out)), nil
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
