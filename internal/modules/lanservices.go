package modules

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

// Local services dashboard (#303): cards for self-hosted services reachable
// from the router. Services bind to known clients by hostname (from DHCP
// leases/reservations), never raw IPs, so cards survive address changes.
// Reachability is tested from the router with a TCP dial first and an
// optional HTTP HEAD/GET for web UIs. No credentials are stored, no proxying.

const (
	lanServicesPath       = "/etc/netgrip/lan_services.json"
	lanServicesBackupPath = lanServicesPath + ".bak"
	lanProbeDialTimeout   = 3 * time.Second
	lanProbeHTTPTimeout   = 4 * time.Second
	lanProbeWorkers       = 8
)

// LanService is one entry of the local services list.
type LanService struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"` // a catalog preset (e.g. "jellyfin") or "custom"
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	Scheme  string `json:"scheme,omitempty"`
	Path    string `json:"path,omitempty"`
	URL     string `json:"url,omitempty"`
	Alias   string `json:"alias,omitempty"`
	Enabled bool   `json:"enabled"`
}

// LanServiceCatalogEntry is one preset of the service catalog.
type LanServiceCatalogEntry struct {
	Kind   string `json:"kind"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme"`
	Path   string `json:"path"`
}

// LanHost is a known client hostname with its current address, offered to the
// add/edit form so services can bind by hostname.
type LanHost struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// LanServiceStatus is one service with its last reachability result.
type LanServiceStatus struct {
	LanService
	OK         bool   `json:"ok"`
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	ResolvedIP string `json:"resolved_ip,omitempty"`
	Error      string `json:"error,omitempty"`
}

// LanServicesProbe is the full response payload for the UI.
type LanServicesProbe struct {
	Services []LanServiceStatus       `json:"services"`
	Catalog  []LanServiceCatalogEntry `json:"catalog"`
	Hosts    []LanHost                `json:"hosts"`
	TS       int64                    `json:"ts"`
}

// lanServicesFile is the on-disk shape of /etc/netgrip/lan_services.json.
type lanServicesFile struct {
	Services []LanService `json:"services"`
}

// lanServiceCatalog holds the well-known defaults for each preset. It is also
// the single source of truth for the catalog picker and any future
// autodiscovery (well-known ports per kind).
var lanServiceCatalog = []LanServiceCatalogEntry{
	{Kind: "homeassistant", Port: 8123, Scheme: "http", Path: ""},
	{Kind: "pihole", Port: 80, Scheme: "http", Path: "/admin"},
	{Kind: "adguardhome", Port: 3000, Scheme: "http", Path: ""},
	{Kind: "proxmox", Port: 8006, Scheme: "https", Path: ""},
	{Kind: "immich", Port: 2283, Scheme: "http", Path: ""},
	{Kind: "jellyfin", Port: 8096, Scheme: "http", Path: ""},
	{Kind: "plex", Port: 32400, Scheme: "http", Path: "/web"},
	{Kind: "truenas", Port: 80, Scheme: "http", Path: ""},
	{Kind: "synology", Port: 5000, Scheme: "http", Path: ""},
	{Kind: "openmediavault", Port: 80, Scheme: "http", Path: ""},
	{Kind: "portainer", Port: 9443, Scheme: "https", Path: ""},
	{Kind: "grafana", Port: 3000, Scheme: "http", Path: ""},
}

// lanProbeHTTPClient is shared by probes. TLS verify is off because LAN web
// UIs routinely use self-signed certs; the check is informational (HEAD/GET
// only, no credentials) and TCP reachability is the authoritative result.
var lanProbeHTTPClient = &http.Client{
	Timeout: lanProbeHTTPTimeout,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func lanServiceCatalogEntry(kind string) (LanServiceCatalogEntry, bool) {
	for _, e := range lanServiceCatalog {
		if e.Kind == kind {
			return e, true
		}
	}
	return LanServiceCatalogEntry{}, false
}

func isCatalogKind(kind string) bool {
	_, ok := lanServiceCatalogEntry(kind)
	return ok
}

// validateLanService normalizes and validates one service. A full URL or a
// host+port are both valid; host entries require a valid port and an http(s)
// scheme (defaulting to http).
func validateLanService(s *LanService) error {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if s.Kind == "" {
		s.Kind = "custom"
	}
	if s.Kind != "custom" && !isCatalogKind(s.Kind) {
		return fmt.Errorf("unknown kind %q", s.Kind)
	}
	s.URL = strings.TrimSpace(s.URL)
	s.Host = strings.TrimSpace(s.Host)
	s.Scheme = strings.ToLower(strings.TrimSpace(s.Scheme))
	s.Path = strings.TrimSpace(s.Path)
	s.Alias = strings.TrimSpace(s.Alias)

	if s.Alias != "" {
		alias, err := normalizeLanAlias(s.Alias)
		if err != nil {
			return err
		}
		s.Alias = alias
		// A cname target must be a resolvable name, not a full URL or a
		// literal IP (a CNAME cannot point at an address).
		if s.URL != "" || s.Host == "" || reIPv4.MatchString(s.Host) {
			return fmt.Errorf("alias requires a hostname (not a full url or ip)")
		}
	}

	if s.URL != "" {
		u, err := url.Parse(s.URL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("invalid url (must start with http:// or https://)")
		}
		return nil
	}
	if s.Host == "" {
		return fmt.Errorf("host is required (or a full url)")
	}
	if s.Scheme == "" {
		s.Scheme = "http"
	}
	if s.Scheme != "http" && s.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if s.Path != "" && !strings.HasPrefix(s.Path, "/") {
		s.Path = "/" + s.Path
	}
	return nil
}

// applyLanServiceDefaults fills empty port/scheme/path from the catalog
// preset, so a catalog pick only needs a name + host.
func applyLanServiceDefaults(s *LanService) {
	if !isCatalogKind(s.Kind) {
		return
	}
	entry, _ := lanServiceCatalogEntry(s.Kind)
	if s.Port == 0 {
		s.Port = entry.Port
	}
	if s.Scheme == "" {
		s.Scheme = entry.Scheme
	}
	if s.Path == "" {
		s.Path = entry.Path
	}
}

// lanServiceURL builds the URL shown to the user (and used by the HTTP
// check). Default ports are elided for readability.
func lanServiceURL(s LanService) string {
	if s.URL != "" {
		return s.URL
	}
	host := s.Host
	if s.Alias != "" {
		host = lanAliasFQDN(s.Alias)
	}
	scheme := s.Scheme
	if scheme == "" {
		scheme = "http"
	}
	port := ""
	if s.Port > 0 && !((scheme == "http" && s.Port == 80) || (scheme == "https" && s.Port == 443)) {
		port = ":" + strconv.Itoa(s.Port)
	}
	return scheme + "://" + host + port + s.Path
}

// lanServiceTarget normalizes a service into the pieces needed for a probe,
// regardless of whether it was entered as a URL or as host+port.
func lanServiceTarget(s LanService) (host string, port int, scheme string, path string, errMsg string) {
	if s.URL != "" {
		u, err := url.Parse(s.URL)
		if err != nil {
			return "", 0, "", "", "invalid url"
		}
		port = 80
		if u.Scheme == "https" {
			port = 443
		}
		if p := u.Port(); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				port = n
			} else {
				return "", 0, "", "", "invalid url port"
			}
		}
		return u.Hostname(), port, u.Scheme, u.Path, ""
	}
	scheme = s.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return s.Host, s.Port, scheme, s.Path, ""
}

// resolveLanServiceIP matches a hostname against DHCP leases then reservation
// names (case-insensitive). Empty when unknown.
func resolveLanServiceIP(host string, leases []ubus.Lease, reservations []Reservation) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	for _, l := range leases {
		if strings.ToLower(l.Hostname) == h && l.IP != "" {
			return l.IP
		}
	}
	for _, r := range reservations {
		if strings.ToLower(r.Name) == h && r.IP != "" {
			return r.IP
		}
	}
	return ""
}

// resolveLanServiceHost resolves a service host to a current address:
// literal IP, then lease/reservation table, then DNS. Empty when nothing
// resolves.
func resolveLanServiceHost(host string) string {
	if reIPv4.MatchString(host) {
		return host
	}
	leases, _, _ := leasesForClients()
	if ip := resolveLanServiceIP(host, leases, probeReservations()); ip != "" {
		return ip
	}
	if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
		return addrs[0]
	}
	return ""
}

// lanServiceHosts lists known client hostnames (leases + reservation names)
// for the hostname picker, deduped case-insensitively.
func lanServiceHosts() []LanHost {
	seen := map[string]LanHost{}
	leases, _, _ := leasesForClients()
	for _, l := range leases {
		name := strings.TrimSpace(l.Hostname)
		if name == "" || name == "*" {
			continue
		}
		if _, ok := seen[strings.ToLower(name)]; !ok {
			seen[strings.ToLower(name)] = LanHost{Name: name, IP: l.IP}
		}
	}
	for _, r := range probeReservations() {
		name := strings.TrimSpace(r.Name)
		if name == "" || name == "*" {
			continue
		}
		if _, ok := seen[strings.ToLower(name)]; !ok {
			seen[strings.ToLower(name)] = LanHost{Name: name, IP: r.IP}
		}
	}
	hosts := make([]LanHost, 0, len(seen))
	for _, h := range seen {
		hosts = append(hosts, h)
	}
	sort.Slice(hosts, func(i, j int) bool { return strings.ToLower(hosts[i].Name) < strings.ToLower(hosts[j].Name) })
	return hosts
}

func loadLanServices() (lanServicesFile, error) {
	data, err := os.ReadFile(lanServicesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return lanServicesFile{Services: []LanService{}}, nil
		}
		return lanServicesFile{}, err
	}
	var cfg lanServicesFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return lanServicesFile{}, err
	}
	if cfg.Services == nil {
		cfg.Services = []LanService{}
	}
	return cfg, nil
}

func saveLanServices(cfg lanServicesFile) error {
	if err := os.MkdirAll(filepath.Dir(lanServicesPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Keep a backup before overwriting, best effort.
	if existing, err := os.ReadFile(lanServicesPath); err == nil {
		_ = os.WriteFile(lanServicesBackupPath, existing, 0600)
	}
	return writeFileAtomic(lanServicesPath, data)
}

// ListLanServices loads config and returns it with fresh reachability status.
func ListLanServices() (*LanServicesProbe, error) {
	cfg, err := loadLanServices()
	if err != nil {
		return nil, err
	}
	return buildLanServicesProbe(cfg), nil
}

func buildLanServicesProbe(cfg lanServicesFile) *LanServicesProbe {
	return &LanServicesProbe{
		Services: probeLanServices(cfg.Services),
		Catalog:  lanServiceCatalog,
		Hosts:    lanServiceHosts(),
		TS:       time.Now().UnixMilli(),
	}
}

// UpsertLanService validates, fills catalog defaults, and replaces-or-appends
// one service. An empty ID generates a new unique one from the name.
func UpsertLanService(in LanService) (*LanServicesProbe, error) {
	if err := validateLanService(&in); err != nil {
		return nil, err
	}
	applyLanServiceDefaults(&in)

	cfg, err := loadLanServices()
	if err != nil {
		return nil, err
	}
	if in.ID == "" {
		in.ID = uniqueLanServiceID(cfg.Services, slugLanServiceID(in.Name))
	}
	var prev LanService
	replaced := false
	for i := range cfg.Services {
		if cfg.Services[i].ID == in.ID {
			prev = cfg.Services[i]
			cfg.Services[i] = in
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Services = append(cfg.Services, in)
	}
	if in.Alias != "" && lanAliasConflicts(lanAliasFQDN(in.Alias), cfg.Services, in.ID, readCnameList(), dnsHostNames()) {
		return nil, fmt.Errorf("alias %q is already in use", lanAliasFQDN(in.Alias))
	}
	if err := saveLanServices(cfg); err != nil {
		return nil, err
	}
	if err := reconcileLanServiceAlias(in, prev); err != nil {
		return nil, err
	}
	return buildLanServicesProbe(cfg), nil
}

// DeleteLanService removes one service by id.
func DeleteLanService(id string) error {
	cfg, err := loadLanServices()
	if err != nil {
		return err
	}
	found := false
	var removed LanService
	filtered := make([]LanService, 0, len(cfg.Services))
	for _, s := range cfg.Services {
		if s.ID == id {
			found = true
			removed = s
			continue
		}
		filtered = append(filtered, s)
	}
	if !found {
		return fmt.Errorf("service not found")
	}
	cfg.Services = filtered
	if err := saveLanServices(cfg); err != nil {
		return err
	}
	return reconcileLanServiceAlias(LanService{}, removed)
}

// slugLanServiceID turns a display name into a stable id slug.
func slugLanServiceID(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "service"
	}
	return out
}

// uniqueLanServiceID returns base, or base-2, base-3, ... until unused.
func uniqueLanServiceID(existing []LanService, base string) string {
	taken := map[string]bool{}
	for _, s := range existing {
		taken[s.ID] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !taken[candidate] {
			return candidate
		}
	}
}

// probeLanServices probes every service in parallel with a bounded worker
// pool, so the endpoint returns promptly even with many services.
func probeLanServices(services []LanService) []LanServiceStatus {
	results := make([]LanServiceStatus, len(services))
	if len(services) == 0 {
		return results
	}
	sem := make(chan struct{}, lanProbeWorkers)
	var wg sync.WaitGroup
	for i := range services {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			results[idx] = probeLanService(services[idx])
			<-sem
		}(i)
	}
	wg.Wait()
	return results
}

// probeLanService runs one reachability test from the router.
func probeLanService(s LanService) LanServiceStatus {
	st := LanServiceStatus{LanService: s}
	if !s.Enabled {
		st.Error = "disabled"
		return st
	}

	host, port, scheme, _, errMsg := lanServiceTarget(s)
	if errMsg != "" {
		st.Error = errMsg
		return st
	}

	ip := resolveLanServiceHost(host)
	if ip == "" {
		st.Error = "unresolved host"
		return st
	}
	st.ResolvedIP = ip

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), lanProbeDialTimeout)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	conn.Close()
	st.LatencyMs = time.Since(start).Milliseconds()
	st.OK = true

	if scheme == "http" || scheme == "https" {
		if code := httpCheckLanService(s); code > 0 {
			st.HTTPStatus = code
		}
	}
	return st
}

// httpCheckLanService performs an informational HTTP HEAD (falling back to
// GET). It never marks a service down: TCP reachability is the authoritative
// result and this only enriches the card with the reported status code.
func httpCheckLanService(s LanService) int {
	target := lanServiceURL(s)
	req, err := http.NewRequest(http.MethodHead, target, nil)
	if err != nil {
		return 0
	}
	if resp, err := lanProbeHTTPClient.Do(req); err == nil {
		resp.Body.Close()
		return resp.StatusCode
	}
	getReq, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return 0
	}
	if resp, err := lanProbeHTTPClient.Do(getReq); err == nil {
		resp.Body.Close()
		return resp.StatusCode
	}
	return 0
}
