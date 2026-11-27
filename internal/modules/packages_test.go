package modules

import (
	"regexp"
	"strings"
	"testing"
)

func TestOptionalCatalogWellFormed(t *testing.T) {
	seen := map[string]bool{}
	re := regexp.MustCompile(`^[a-z0-9][a-z0-9+_.-]*$`)
	for _, e := range optionalCatalog {
		if seen[e.ID] {
			t.Fatalf("duplicate id: %s", e.ID)
		}
		seen[e.ID] = true
		if len(e.Packages) == 0 {
			t.Fatalf("%s: no packages", e.ID)
		}
		for _, p := range e.Packages {
			if !re.MatchString(p) {
				t.Fatalf("%s: invalid package name %q", e.ID, p)
			}
		}
		if e.I18nKey == "" || e.Module == "" {
			t.Fatalf("%s: missing i18n key or module", e.ID)
		}
	}
}

func TestOptionalCatalogCoversServices(t *testing.T) {
	// nft-qos is NOT here on purpose: the package does not exist in the
	// official feeds (verified on 25.12, see issue #138).
	want := []string{
		"wireguard-tools", "kmod-wireguard", "ddns-scripts",
		"openvpn-openssl", "openvpn-easy-rsa", "sqm-scripts",
		"nlbwmon", "tailscale", "adguardhome",
	}
	have := map[string]bool{}
	for _, e := range optionalCatalog {
		for _, p := range e.Packages {
			have[p] = true
		}
	}
	for _, w := range want {
		if !have[w] {
			t.Fatalf("catalog missing package %q", w)
		}
	}
}

func TestRemoveOptionalPackagesRejectsUnknownID(t *testing.T) {
	if _, err := RemoveOptionalPackages([]string{"does-not-exist"}); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestRemoveOptionalPackagesRefusesForeignNames(t *testing.T) {
	// The whitelist must never expose base-system packages: unknown ids are
	// rejected even if they look like real package names.
	for _, id := range []string{"dropbear", "kmod-nft-core", "firewall4"} {
		if _, err := RemoveOptionalPackages([]string{id}); err == nil {
			t.Fatalf("expected rejection for %q", id)
		}
	}
}

func TestInstallOptionalPackagesRejectsUnknownID(t *testing.T) {
	if _, err := InstallOptionalPackages([]string{"does-not-exist"}); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestResolveOptionalPackageIDsEmpty(t *testing.T) {
	if _, err := ResolveOptionalPackageIDs(nil); err == nil {
		t.Errorf("empty ids must fail instead of returning a silent empty job")
	}
	if _, err := ResolveOptionalPackageIDs([]string{}); err == nil {
		t.Errorf("empty ids must fail instead of returning a silent empty job")
	}
}

func TestApkAvailableFrom(t *testing.T) {
	// Real `apk search -x` output: one name-version line per match, and
	// variants answer for the plain name (snmpd -> snmpd-nossl/snmpd-ssl).
	out := strings.Join([]string{
		"adguardhome-1.0.0-r1",
		"curl-8.23.0-r2",
		"nlbwmon-2025.06.02~29236be6-r2",
		"snmpd-nossl-5.9.5.2-r2",
		"snmpd-ssl-5.9.5.2-r2",
		"tailscale-1.104.6-r1",
		"",
	}, "\n")
	got := apkAvailableFrom([]string{"adguardhome", "nlbwmon", "netifyd", "snmpd", "tailscale", "zlib"}, out)
	want := map[string]bool{
		"adguardhome": true,
		"nlbwmon":     true,
		"netifyd":     false,
		"snmpd":       true,
		"tailscale":   true,
		"zlib":        false,
	}
	for name, available := range want {
		if got[name] != available {
			t.Fatalf("%s: available=%v, want %v", name, got[name], available)
		}
	}
}

func TestApkAvailableFromRequiresDashSeparator(t *testing.T) {
	// A shorter name must not be satisfied by a longer one that happens to
	// share its prefix: the version separator is what makes it the same name.
	if got := apkAvailableFrom([]string{"netify"}, "netifyd-4.4.7-r2\n"); got["netify"] {
		t.Fatal("netify must not be reported available by netifyd")
	}
}

func TestOptionalPkgAvailableNeedsEveryPackage(t *testing.T) {
	entry := OptionalPkg{ID: "usbstorage", Packages: []string{"kmod-usb-storage", "block-mount"}}
	if optionalPkgAvailable(entry, map[string]bool{"kmod-usb-storage": true, "block-mount": false}) {
		t.Fatal("one missing package must make the entry unavailable")
	}
	if !optionalPkgAvailable(entry, map[string]bool{"kmod-usb-storage": true, "block-mount": true}) {
		t.Fatal("entry with every package available must be available")
	}
}
