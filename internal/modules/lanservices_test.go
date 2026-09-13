package modules

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

func TestValidateLanService(t *testing.T) {
	t.Run("valid catalog", func(t *testing.T) {
		s := LanService{Name: "Jellyfin", Kind: "jellyfin", Host: "nas", Port: 8096}
		if err := validateLanService(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Scheme != "http" {
			t.Fatalf("scheme should default to http, got %q", s.Scheme)
		}
	})
	t.Run("valid custom with url", func(t *testing.T) {
		s := LanService{Name: "Proxy", Kind: "custom", URL: "https://home.example.com/jellyfin"}
		if err := validateLanService(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("empty kind defaults to custom", func(t *testing.T) {
		s := LanService{Name: "Something", Host: "host", Port: 1234}
		if err := validateLanService(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Kind != "custom" {
			t.Fatalf("kind should default to custom, got %q", s.Kind)
		}
	})
	t.Run("rejects empty name", func(t *testing.T) {
		if err := validateLanService(&LanService{Kind: "custom", Host: "h", Port: 80}); err == nil {
			t.Fatal("expected error for empty name")
		}
	})
	t.Run("rejects unknown kind", func(t *testing.T) {
		if err := validateLanService(&LanService{Name: "X", Kind: "nope", Host: "h", Port: 80}); err == nil {
			t.Fatal("expected error for unknown kind")
		}
	})
	t.Run("rejects bad port", func(t *testing.T) {
		if err := validateLanService(&LanService{Name: "X", Kind: "custom", Host: "h", Port: 70000}); err == nil {
			t.Fatal("expected error for out-of-range port")
		}
	})
	t.Run("rejects url without scheme", func(t *testing.T) {
		if err := validateLanService(&LanService{Name: "X", Kind: "custom", URL: "home.example.com"}); err == nil {
			t.Fatal("expected error for url without scheme")
		}
	})
	t.Run("normalizes path without leading slash", func(t *testing.T) {
		s := LanService{Name: "X", Kind: "custom", Host: "h", Port: 80, Path: "admin"}
		if err := validateLanService(&s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Path != "/admin" {
			t.Fatalf("path should be normalized to /admin, got %q", s.Path)
		}
	})
}

func TestLanServiceCatalogEntry(t *testing.T) {
	required := []string{
		"homeassistant", "pihole", "adguardhome", "proxmox", "immich",
		"jellyfin", "plex", "truenas", "synology", "openmediavault",
		"portainer", "grafana",
	}
	for _, kind := range required {
		if _, ok := lanServiceCatalogEntry(kind); !ok {
			t.Fatalf("catalog missing required kind %q", kind)
		}
	}
	if _, ok := lanServiceCatalogEntry("custom"); ok {
		t.Fatal("custom should not be a catalog entry")
	}
	if _, ok := lanServiceCatalogEntry("unknown"); ok {
		t.Fatal("unknown should not be a catalog entry")
	}
	// Proxmox is TLS with its well-known port.
	e, _ := lanServiceCatalogEntry("proxmox")
	if e.Scheme != "https" || e.Port != 8006 {
		t.Fatalf("unexpected proxmox defaults: %+v", e)
	}
}

func TestApplyLanServiceDefaults(t *testing.T) {
	s := LanService{Name: "Jellyfin", Kind: "jellyfin", Host: "nas"}
	applyLanServiceDefaults(&s)
	if s.Port != 8096 || s.Scheme != "http" {
		t.Fatalf("catalog defaults not applied: %+v", s)
	}
	// Custom entries keep their own values.
	c := LanService{Name: "X", Kind: "custom", Host: "h", Port: 1234, Scheme: "https"}
	applyLanServiceDefaults(&c)
	if c.Port != 1234 || c.Scheme != "https" {
		t.Fatalf("custom entry should be untouched: %+v", c)
	}
}

func TestLanServiceURL(t *testing.T) {
	cases := []struct {
		name string
		in   LanService
		want string
	}{
		{"full url passthrough", LanService{URL: "https://home.example.com/jellyfin"}, "https://home.example.com/jellyfin"},
		{"http default port elided", LanService{Scheme: "http", Host: "nas", Port: 80, Path: "/admin"}, "http://nas/admin"},
		{"https default port elided", LanService{Scheme: "https", Host: "nas", Port: 443}, "https://nas"},
		{"non-default port shown", LanService{Scheme: "http", Host: "nas", Port: 8096}, "http://nas:8096"},
		{"empty scheme defaults http", LanService{Host: "nas", Port: 8096}, "http://nas:8096"},
		{"alias preferred over host", LanService{Scheme: "http", Host: "nas", Alias: "jellyfin", Port: 8096}, "http://jellyfin.lan:8096"},
		{"alias with default port elided", LanService{Scheme: "http", Host: "nas", Alias: "jellyfin", Port: 80}, "http://jellyfin.lan"},
		{"alias with path", LanService{Scheme: "https", Host: "nas", Alias: "plex", Port: 32400, Path: "/web"}, "https://plex.lan:32400/web"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lanServiceURL(c.in); got != c.want {
				t.Fatalf("lanServiceURL() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestLanServiceTarget(t *testing.T) {
	host, port, scheme, path, errMsg := lanServiceTarget(LanService{Scheme: "https", Host: "nas", Port: 8006})
	if errMsg != "" || host != "nas" || port != 8006 || scheme != "https" || path != "" {
		t.Fatalf("unexpected host target: %s %d %s %q %q", host, port, scheme, path, errMsg)
	}
	host, port, scheme, path, errMsg = lanServiceTarget(LanService{URL: "https://home.example.com:8443/app"})
	if errMsg != "" || host != "home.example.com" || port != 8443 || scheme != "https" || path != "/app" {
		t.Fatalf("unexpected url target: %s %d %s %q %q", host, port, scheme, path, errMsg)
	}
	host, port, scheme, _, errMsg = lanServiceTarget(LanService{URL: "http://home.example.com"})
	if errMsg != "" || host != "home.example.com" || port != 80 || scheme != "http" {
		t.Fatalf("unexpected default-port url target: %s %d %s", host, port, scheme)
	}
}

func TestResolveLanServiceIP(t *testing.T) {
	leases := []ubus.Lease{
		{Expires: time.Now().Add(time.Hour), MAC: "aa:bb", IP: "192.168.8.10", Hostname: "nas"},
		{Expires: time.Now().Add(time.Hour), MAC: "cc:dd", IP: "192.168.8.11", Hostname: "tv"},
	}
	reservations := []Reservation{
		{MAC: "ee:ff", IP: "192.168.8.20", Name: "printer"},
	}
	if got := resolveLanServiceIP("NAS", leases, reservations); got != "192.168.8.10" {
		t.Fatalf("lease match should be case-insensitive, got %q", got)
	}
	if got := resolveLanServiceIP("printer", leases, reservations); got != "192.168.8.20" {
		t.Fatalf("reservation match failed, got %q", got)
	}
	if got := resolveLanServiceIP("unknown", leases, reservations); got != "" {
		t.Fatalf("unknown host should resolve empty, got %q", got)
	}
	if got := resolveLanServiceIP("", leases, reservations); got != "" {
		t.Fatalf("empty host should resolve empty, got %q", got)
	}
}

func TestSlugLanServiceID(t *testing.T) {
	cases := map[string]string{
		"Jellyfin":          "jellyfin",
		"  Home Assistant ": "home-assistant",
		"Pi-hole":           "pi-hole",
		"!!!":               "service",
		"Mi NAS 2":          "mi-nas-2",
	}
	for in, want := range cases {
		if got := slugLanServiceID(in); got != want {
			t.Fatalf("slugLanServiceID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUniqueLanServiceID(t *testing.T) {
	existing := []LanService{{ID: "jellyfin"}, {ID: "jellyfin-2"}}
	if got := uniqueLanServiceID(existing, "jellyfin"); got != "jellyfin-3" {
		t.Fatalf("expected jellyfin-3, got %q", got)
	}
	if got := uniqueLanServiceID(existing, "grafana"); got != "grafana" {
		t.Fatalf("expected grafana, got %q", got)
	}
}

func TestNormalizeLanAlias(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"jellyfin", "jellyfin", false},
		{"  Jellyfin  ", "jellyfin", false},
		{"JELLYFIN.LAN", "jellyfin", false},
		{"jellyfin.lan", "jellyfin", false},
		{"", "", false},
		{"a.b.c", "", true},
		{"foo.bar", "", true},
		{"has space", "", true},
		{"-leading", "", true},
		{"trailing-", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := normalizeLanAlias(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("normalizeLanAlias(%q) expected error, got %q", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeLanAlias(%q) unexpected error: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("normalizeLanAlias(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestLanAliasHelpers(t *testing.T) {
	if got := lanAliasFQDN("jellyfin"); got != "jellyfin.lan" {
		t.Fatalf("lanAliasFQDN = %q, want jellyfin.lan", got)
	}
	if got := lanAliasSection("jellyfin"); got != "netgrip_alias_jellyfin" {
		t.Fatalf("lanAliasSection = %q, want netgrip_alias_jellyfin", got)
	}
}

func TestSuggestLanAlias(t *testing.T) {
	if got := suggestLanAlias(LanService{Name: "Jellyfin"}); got != "jellyfin" {
		t.Fatalf("suggestLanAlias(name) = %q, want jellyfin", got)
	}
	if got := suggestLanAlias(LanService{Name: "Home Assistant"}); got != "home-assistant" {
		t.Fatalf("suggestLanAlias(name with spaces) = %q, want home-assistant", got)
	}
	if got := suggestLanAlias(LanService{Name: "", Kind: "jellyfin"}); got != "jellyfin" {
		t.Fatalf("suggestLanAlias(kind) = %q, want jellyfin", got)
	}
	if got := suggestLanAlias(LanService{Name: "", Kind: "custom"}); got != "" {
		t.Fatalf("suggestLanAlias(empty) = %q, want empty", got)
	}
	if got := suggestLanAlias(LanService{Name: "!!!"}); got != "" {
		t.Fatalf("suggestLanAlias(no slug) = %q, want empty", got)
	}
}

func TestLanAliasConflicts(t *testing.T) {
	services := []LanService{
		{ID: "a", Alias: "jellyfin"},
		{ID: "b"},
	}
	if !lanAliasConflicts("jellyfin.lan", services, "b", nil, nil) {
		t.Fatal("expected conflict with another service alias")
	}
	if lanAliasConflicts("grafana.lan", services, "b", nil, nil) {
		t.Fatal("did not expect conflict for unused alias")
	}
	// Self must be ignored (rename keeps its own alias).
	if lanAliasConflicts("jellyfin.lan", services, "a", nil, nil) {
		t.Fatal("self alias must not conflict")
	}
	// Existing dnsmasq cname.
	if !lanAliasConflicts("plex.lan", nil, "", []string{"plex.lan"}, nil) {
		t.Fatal("expected conflict with existing cname")
	}
	// Existing hosts-file hostname (FQDN and bare label).
	if !lanAliasConflicts("nas.lan", nil, "", nil, []string{"nas.lan"}) {
		t.Fatal("expected conflict with hosts FQDN")
	}
	if !lanAliasConflicts("nas.lan", nil, "", nil, []string{"NAS"}) {
		t.Fatal("expected case-insensitive conflict with hosts label")
	}
}

func TestValidateLanServiceAlias(t *testing.T) {
	s := LanService{Name: "Jellyfin", Kind: "jellyfin", Host: "nas", Port: 8096, Alias: "Jellyfin.LAN"}
	if err := validateLanService(&s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Alias != "jellyfin" {
		t.Fatalf("alias should normalize to jellyfin, got %q", s.Alias)
	}
	// Alias requires a hostname, not an IP or a full URL.
	if err := validateLanService(&LanService{Name: "X", Kind: "custom", Host: "192.168.8.10", Port: 80, Alias: "x"}); err == nil {
		t.Fatal("expected error for alias on ip host")
	}
	if err := validateLanService(&LanService{Name: "X", Kind: "custom", URL: "https://x.example.com", Alias: "x"}); err == nil {
		t.Fatal("expected error for alias on url service")
	}
	if err := validateLanService(&LanService{Name: "X", Kind: "custom", Host: "nas", Port: 80, Alias: "a.b"}); err == nil {
		t.Fatal("expected error for dotted alias")
	}
}

func TestLanServicesRoundtrip(t *testing.T) {
	cfg := lanServicesFile{Services: []LanService{
		{ID: "jellyfin", Name: "Jellyfin", Kind: "jellyfin", Host: "nas", Port: 8096, Scheme: "http", Alias: "jellyfin", Enabled: true},
		{ID: "proxy", Name: "Proxy", Kind: "custom", URL: "https://home.example.com", Enabled: true},
	}}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "latency_ms") {
		t.Fatal("persisted file must not contain status fields")
	}
	var back lanServicesFile
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(back.Services))
	}
	if back.Services[0].ID != "jellyfin" || back.Services[0].Port != 8096 || back.Services[0].Alias != "jellyfin" {
		t.Fatalf("roundtrip mismatch: %+v", back.Services[0])
	}
	if back.Services[1].URL != "https://home.example.com" {
		t.Fatalf("roundtrip url mismatch: %+v", back.Services[1])
	}
}
