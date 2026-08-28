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
