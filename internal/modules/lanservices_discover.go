package modules

import (
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Local services autodiscovery (#321): on demand only, never in background.
// The catalog's well-known ports are dialed against known LAN hosts (DHCP
// leases + reservation names) with a bounded worker pool. A TCP hit is the
// authoritative signal; results are suggestions only and are never written
// to storage. Adding still goes through the regular upsert endpoint.

const (
	lanDiscoverDialTimeout = 1 * time.Second
	lanDiscoverWorkers     = 32
	lanDiscoverMaxDials    = 400
)

// LanDiscoverySuggestion is one candidate service found on a known host.
type LanDiscoverySuggestion struct {
	Host   string `json:"host"`
	IP     string `json:"ip"`
	Kind   string `json:"kind"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme"`
	Path   string `json:"path,omitempty"`
}

// LanDiscoveryResult is the response payload for POST /api/lanservices/discover.
type LanDiscoveryResult struct {
	Suggestions  []LanDiscoverySuggestion `json:"suggestions"`
	ScannedHosts int                      `json:"scanned_hosts"`
	SkippedHosts int                      `json:"skipped_hosts,omitempty"`
	TS           int64                    `json:"ts"`
}

// lanDiscoverTarget is one host+port to dial, with the catalog entries that
// match that port (more than one when several kinds share a port).
type lanDiscoverTarget struct {
	Host    LanHost
	Port    int
	Entries []LanServiceCatalogEntry
}

// lanDiscoverUniquePorts returns the sorted set of distinct catalog ports.
func lanDiscoverUniquePorts() []int {
	seen := map[int]bool{}
	for _, e := range lanServiceCatalog {
		seen[e.Port] = true
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

// lanDiscoverKindsForPort returns the catalog entries whose well-known port
// matches p, in catalog order. Empty when no entry uses p.
func lanDiscoverKindsForPort(p int) []LanServiceCatalogEntry {
	var out []LanServiceCatalogEntry
	for _, e := range lanServiceCatalog {
		if e.Port == p {
			out = append(out, e)
		}
	}
	return out
}

// planLanDiscovery builds the (host, port) dial targets for a bounded scan,
// skipping combinations already present in the stored services and trimming
// the host list when the total would exceed maxDials. It returns the targets
// and how many hosts were dropped by trimming.
func planLanDiscovery(hosts []LanHost, existing []LanService, maxDials int) ([]lanDiscoverTarget, int) {
	ports := lanDiscoverUniquePorts()
	if len(hosts) == 0 || len(ports) == 0 || maxDials <= 0 {
		return nil, 0
	}

	skipped := 0
	maxHosts := maxDials / len(ports)
	if maxHosts < 1 {
		maxHosts = 1
	}
	if len(hosts) > maxHosts {
		skipped = len(hosts) - maxHosts
		hosts = hosts[:maxHosts]
	}

	// Index existing host:port combos (case-insensitive host), resolving URL
	// entries through the same normalization the probe uses.
	existingPorts := map[string]map[int]bool{}
	for _, s := range existing {
		host, port, _, _, errMsg := lanServiceTarget(s)
		if errMsg != "" || host == "" || port <= 0 {
			continue
		}
		host = strings.ToLower(strings.TrimSpace(host))
		if existingPorts[host] == nil {
			existingPorts[host] = map[int]bool{}
		}
		existingPorts[host][port] = true
	}

	var targets []lanDiscoverTarget
	for _, h := range hosts {
		if h.IP == "" {
			continue
		}
		hostKey := strings.ToLower(strings.TrimSpace(h.Name))
		for _, p := range ports {
			if existingPorts[hostKey] != nil && existingPorts[hostKey][p] {
				continue
			}
			targets = append(targets, lanDiscoverTarget{
				Host:    h,
				Port:    p,
				Entries: lanDiscoverKindsForPort(p),
			})
		}
	}
	return targets, skipped
}

// DiscoverLanServices scans the catalog's well-known ports against known LAN
// hosts and returns suggestions only. It never writes to storage.
func DiscoverLanServices() (*LanDiscoveryResult, error) {
	cfg, err := loadLanServices()
	if err != nil {
		return nil, err
	}
	targets, skipped := planLanDiscovery(lanServiceHosts(), cfg.Services, lanDiscoverMaxDials)

	return &LanDiscoveryResult{
		Suggestions:  probeDiscoveryTargets(targets),
		ScannedHosts: countDistinctHosts(targets),
		SkippedHosts: skipped,
		TS:           time.Now().UnixMilli(),
	}, nil
}

// probeDiscoveryTargets dials each target in parallel with a bounded worker
// pool and emits one suggestion per matching catalog entry on a TCP hit.
func probeDiscoveryTargets(targets []lanDiscoverTarget) []LanDiscoverySuggestion {
	if len(targets) == 0 {
		return nil
	}
	var mu sync.Mutex
	var suggestions []LanDiscoverySuggestion
	sem := make(chan struct{}, lanDiscoverWorkers)
	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func(t lanDiscoverTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if !tcpReachable(t.Host.IP, t.Port) {
				return
			}
			mu.Lock()
			for _, e := range t.Entries {
				suggestions = append(suggestions, LanDiscoverySuggestion{
					Host:   t.Host.Name,
					IP:     t.Host.IP,
					Kind:   e.Kind,
					Port:   t.Port,
					Scheme: e.Scheme,
					Path:   e.Path,
				})
			}
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	sort.Slice(suggestions, func(i, j int) bool {
		a, b := suggestions[i], suggestions[j]
		ah, bh := strings.ToLower(a.Host), strings.ToLower(b.Host)
		if ah != bh {
			return ah < bh
		}
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		return a.Kind < b.Kind
	})
	return suggestions
}

func tcpReachable(ip string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), lanDiscoverDialTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func countDistinctHosts(targets []lanDiscoverTarget) int {
	seen := map[string]bool{}
	for _, t := range targets {
		seen[strings.ToLower(t.Host.Name)] = true
	}
	return len(seen)
}
