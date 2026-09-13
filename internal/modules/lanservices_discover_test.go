package modules

import (
	"strings"
	"testing"
)

func TestLanDiscoverUniquePorts(t *testing.T) {
	ports := lanDiscoverUniquePorts()
	// Ports must be sorted and deduplicated (80 and 3000 appear once).
	for i := 1; i < len(ports); i++ {
		if ports[i-1] >= ports[i] {
			t.Fatalf("ports not sorted/unique: %v", ports)
		}
	}
	counts := map[int]int{}
	for _, p := range ports {
		counts[p]++
	}
	if counts[80] != 1 {
		t.Fatalf("port 80 should appear once, got %d", counts[80])
	}
	if counts[3000] != 1 {
		t.Fatalf("port 3000 should appear once, got %d", counts[3000])
	}
}

func TestLanDiscoverKindsForPort(t *testing.T) {
	kinds := func(p int) []string {
		out := []string{}
		for _, e := range lanDiscoverKindsForPort(p) {
			out = append(out, e.Kind)
		}
		return out
	}
	if got := kinds(80); len(got) != 3 {
		t.Fatalf("port 80 should map to 3 kinds, got %v", got)
	}
	if got := kinds(3000); len(got) != 2 {
		t.Fatalf("port 3000 should map to 2 kinds (adguardhome+grafana), got %v", got)
	}
	if got := kinds(8096); len(got) != 1 || got[0] != "jellyfin" {
		t.Fatalf("port 8096 should map to jellyfin only, got %v", got)
	}
	if got := kinds(9999); len(got) != 0 {
		t.Fatalf("port 9999 should map to nothing, got %v", got)
	}
}

func TestPlanLanDiscovery(t *testing.T) {
	hosts := []LanHost{
		{Name: "nas", IP: "192.168.8.10"},
		{Name: "tv", IP: "192.168.8.11"},
	}
	ports := lanDiscoverUniquePorts()

	t.Run("generates hosts x ports targets", func(t *testing.T) {
		targets, skipped := planLanDiscovery(hosts, nil, 400)
		if skipped != 0 {
			t.Fatalf("unexpected skip: %d", skipped)
		}
		if len(targets) != len(hosts)*len(ports) {
			t.Fatalf("expected %d targets, got %d", len(hosts)*len(ports), len(targets))
		}
	})

	t.Run("dedups existing host+port", func(t *testing.T) {
		existing := []LanService{
			{ID: "jellyfin", Name: "Jellyfin", Kind: "jellyfin", Host: "NAS", Port: 8096, Scheme: "http"},
		}
		targets, _ := planLanDiscovery(hosts, existing, 400)
		if len(targets) != len(hosts)*len(ports)-1 {
			t.Fatalf("expected one dedup, got %d targets", len(targets))
		}
		for _, tg := range targets {
			if strings.EqualFold(tg.Host.Name, "nas") && tg.Port == 8096 {
				t.Fatal("existing nas:8096 should have been deduped")
			}
		}
	})

	t.Run("dedups url entry via normalized target", func(t *testing.T) {
		existing := []LanService{
			{ID: "proxy", Name: "Proxy", Kind: "custom", URL: "http://tv:8096"},
		}
		targets, _ := planLanDiscovery(hosts, existing, 400)
		for _, tg := range targets {
			if strings.EqualFold(tg.Host.Name, "tv") && tg.Port == 8096 {
				t.Fatal("existing tv:8096 (via url) should have been deduped")
			}
		}
	})

	t.Run("trims hosts when over budget", func(t *testing.T) {
		manyHosts := make([]LanHost, 10)
		for i := range manyHosts {
			manyHosts[i] = LanHost{Name: "h" + string(rune('a'+i)), IP: "192.168.8.1"}
		}
		targets, skipped := planLanDiscovery(manyHosts, nil, 4*len(ports))
		if skipped != 10-4 {
			t.Fatalf("expected 6 skipped, got %d", skipped)
		}
		if len(targets) > 4*len(ports) {
			t.Fatalf("targets over budget: %d", len(targets))
		}
	})

	t.Run("skips hosts without ip", func(t *testing.T) {
		noIP := []LanHost{{Name: "ghost", IP: ""}}
		targets, _ := planLanDiscovery(noIP, nil, 400)
		if len(targets) != 0 {
			t.Fatalf("expected no targets for empty-ip host, got %d", len(targets))
		}
	})
}
