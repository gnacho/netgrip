package modules

import (
	"regexp"
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
	want := []string{
		"wireguard-tools", "kmod-wireguard", "ddns-scripts",
		"openvpn-openssl", "openvpn-easy-rsa", "sqm-scripts",
		"nlbwmon", "nft-qos", "tailscale", "adguardhome",
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

func TestInstallOptionalPackagesRejectsUnknownID(t *testing.T) {
	if _, err := InstallOptionalPackages([]string{"does-not-exist"}); err == nil {
		t.Fatal("expected error for unknown id")
	}
}
