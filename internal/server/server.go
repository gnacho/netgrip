package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gnacho/owpanel/internal/auth"
	"github.com/gnacho/owpanel/internal/modules"
	"github.com/gnacho/owpanel/internal/ubus"
)

//go:embed all:dist
var distFS embed.FS

const (
	sessionCookie = "owpanel_session"
	sessionTTL    = 12 * time.Hour
	leasesPath    = "/tmp/dhcp.leases"
)

type Server struct {
	rpcdURL string
	version string
	mux     *http.ServeMux
	mu      sync.Mutex
	revoked map[string]bool
}

func New(rpcdURL, version string) *Server {
	s := &Server{
		rpcdURL: rpcdURL,
		version: version,
		mux:     http.NewServeMux(),
		revoked: make(map[string]bool),
	}
	s.mux.HandleFunc("/", s.handleSPA)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/me", s.requireAuth(s.handleMe))
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/board", s.requireAuth(s.handleBoard))
	s.mux.HandleFunc("GET /api/system", s.requireAuth(s.handleSystem))
	s.mux.HandleFunc("GET /api/wan", s.requireAuth(s.handleWan))
	s.mux.HandleFunc("GET /api/wireless", s.requireAuth(s.handleWireless))
	s.mux.HandleFunc("GET /api/leases", s.requireAuth(s.handleLeases))
	s.mux.HandleFunc("GET /api/ipv6", s.requireAuth(s.handleIPv6Get))
	s.mux.HandleFunc("POST /api/ipv6", s.requireAuth(s.handleIPv6Set))
	s.mux.HandleFunc("POST /api/password", s.requireAuth(s.handlePasswordSet))
	s.mux.HandleFunc("GET /api/update", s.requireAuth(s.handleUpdateCheck))
	s.mux.HandleFunc("POST /api/update", s.requireAuth(s.handleUpdateStart))
	s.mux.HandleFunc("GET /api/wireguard", s.requireAuth(s.handleWGGet))
	s.mux.HandleFunc("POST /api/wireguard", s.requireAuth(s.handleWGSet))
	s.mux.HandleFunc("POST /api/wireguard/peers", s.requireAuth(s.handleWGPeerAdd))
	s.mux.HandleFunc("POST /api/wireguard/peers/qr", s.requireAuth(s.handleWGPeerQR))
	s.mux.HandleFunc("POST /api/wireguard/peers/delete", s.requireAuth(s.handleWGPeerDelete))
	s.mux.HandleFunc("GET /api/ddns", s.requireAuth(s.handleDDNSGet))
	s.mux.HandleFunc("POST /api/ddns", s.requireAuth(s.handleDDNSSet))
	s.mux.HandleFunc("GET /api/sqm", s.requireAuth(s.handleSQMGet))
	s.mux.HandleFunc("POST /api/sqm", s.requireAuth(s.handleSQMSet))
	s.mux.HandleFunc("GET /api/openvpn", s.requireAuth(s.handleOVPNGet))
	s.mux.HandleFunc("POST /api/openvpn", s.requireAuth(s.handleOVPNSet))
	s.mux.HandleFunc("POST /api/openvpn/clients", s.requireAuth(s.handleOVPNClientAdd))
	s.mux.HandleFunc("POST /api/openvpn/clients/delete", s.requireAuth(s.handleOVPNClientDelete))
	s.mux.HandleFunc("GET /api/packages", s.requireAuth(s.handlePackagesGet))
	s.mux.HandleFunc("POST /api/packages/upgrade", s.requireAuth(s.handlePackageUpgrade))
	s.mux.HandleFunc("GET /api/iotwifi", s.requireAuth(s.handleIoTGet))
	s.mux.HandleFunc("POST /api/iotwifi", s.requireAuth(s.handleIoTSet))
	s.mux.HandleFunc("GET /api/portforward", s.requireAuth(s.handleFwdGet))
	s.mux.HandleFunc("POST /api/portforward", s.requireAuth(s.handleFwdAdd))
	s.mux.HandleFunc("POST /api/portforward/delete", s.requireAuth(s.handleFwdDelete))
	s.mux.HandleFunc("GET /api/tailscale", s.requireAuth(s.handleTSGet))
	s.mux.HandleFunc("POST /api/tailscale", s.requireAuth(s.handleTSSet))
	s.mux.HandleFunc("GET /api/guestwifi", s.requireAuth(s.handleGuestGet))
	s.mux.HandleFunc("POST /api/guestwifi", s.requireAuth(s.handleGuestSet))
	s.mux.HandleFunc("GET /api/mode", s.requireAuth(s.handleMode))
	s.mux.HandleFunc("POST /api/mode", s.requireAuth(s.handleModeSet))
	s.mux.HandleFunc("GET /api/access", s.requireAuth(s.handleAccessGet))
	s.mux.HandleFunc("POST /api/access", s.requireAuth(s.handleAccessSet))
	s.mux.HandleFunc("GET /api/remoteaccess", s.requireAuth(s.handleRemoteGet))
	s.mux.HandleFunc("POST /api/remoteaccess", s.requireAuth(s.handleRemoteSet))
	s.mux.HandleFunc("GET /api/offload", s.requireAuth(s.handleOffloadGet))
	s.mux.HandleFunc("POST /api/offload", s.requireAuth(s.handleOffloadSet))
	s.mux.HandleFunc("GET /api/wifi", s.requireAuth(s.handleWifiGet))
	s.mux.HandleFunc("POST /api/wifi", s.requireAuth(s.handleWifiSet))
	s.mux.HandleFunc("GET /api/lan", s.requireAuth(s.handleLANGet))
	s.mux.HandleFunc("POST /api/lan", s.requireAuth(s.handleLANSet))
	s.mux.HandleFunc("POST /api/lan/dhcp", s.requireAuth(s.handleDHCPSet))
	s.mux.HandleFunc("POST /api/lan/reservation", s.requireAuth(s.handleReservationSet))
	s.mux.HandleFunc("POST /api/lan/reservations/clear", s.requireAuth(s.handleReservationsClear))
	s.mux.HandleFunc("GET /api/dns", s.requireAuth(s.handleDNSGet))
	s.mux.HandleFunc("POST /api/dns", s.requireAuth(s.handleDNSSet))
	s.mux.HandleFunc("POST /api/dns/hosts", s.requireAuth(s.handleDNSHostsSet))
	s.mux.HandleFunc("GET /api/netdev", s.requireAuth(s.handleNetDev))
	s.mux.HandleFunc("GET /api/ethports", s.requireAuth(s.handleEthPorts))
	s.mux.HandleFunc("GET /api/dawn", s.requireAuth(s.handleDawn))
	s.mux.HandleFunc("GET /api/clients", s.requireAuth(s.handleClients))
	s.mux.HandleFunc("POST /api/clients/reserve", s.requireAuth(s.handleClientReserve))
	s.mux.HandleFunc("POST /api/clients/block", s.requireAuth(s.handleClientBlock))
	s.mux.HandleFunc("GET /api/config/snapshots", s.requireAuth(s.handleSnapshotsList))
	s.mux.HandleFunc("POST /api/config/snapshot", s.requireAuth(s.handleSnapshotCreate))
	s.mux.HandleFunc("DELETE /api/config/snapshot", s.requireAuth(s.handleSnapshotDelete))
	s.mux.HandleFunc("GET /api/config/diff", s.requireAuth(s.handleSnapshotDiff))
	s.mux.HandleFunc("POST /api/config/rollback", s.requireAuth(s.handleSnapshotRollback))
	s.mux.HandleFunc("POST /api/ports/bounce", s.requireAuth(s.handlePortBounce))
	s.mux.HandleFunc("GET /api/igmp", s.requireAuth(s.handleIGMPGet))
	s.mux.HandleFunc("POST /api/igmp", s.requireAuth(s.handleIGMPSet))
	s.mux.HandleFunc("GET /api/loops", s.requireAuth(s.handleLoops))
	s.mux.HandleFunc("GET /api/selfupdate", s.requireAuth(s.handleSelfUpdateCheck))
	s.mux.HandleFunc("POST /api/selfupdate", s.requireAuth(s.handleSelfUpdateApply))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handleSPA serves the embedded frontend with SPA fallback to index.html.
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		http.Error(w, "frontend not embedded", http.StatusInternalServerError)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" {
		if f, err := dist.Open(path); err == nil {
			f.Close()
			http.FileServer(http.FS(dist)).ServeHTTP(w, r)
			return
		}
	}
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "frontend not embedded", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Username == "" {
		req.Username = "root"
	}
	ok, err := auth.ValidatePassword(s.rpcdURL, req.Username, req.Password)
	if err != nil {
		log.Printf("login: rpcd validation error: %v", err)
		writeError(w, http.StatusBadGateway, "rpcd unreachable")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	ttl := time.Duration(modules.PanelSessionTTLMinutes()) * time.Minute
	token, err := auth.NewSessionToken(ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session token")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		s.revoked[c.Value] = true
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !auth.ValidSessionToken(c.Value) || s.isRevoked(c.Value) {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		next(w, r)
	}
}

func (s *Server) isRevoked(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revoked[token]
}

func (s *Server) handleBoard(w http.ResponseWriter, _ *http.Request) {
	raw, err := ubus.Call("system", "board")
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

func (s *Server) handleSystem(w http.ResponseWriter, _ *http.Request) {
	info, err := ubus.GetSystemInfo()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, info)
}

func (s *Server) handleWan(w http.ResponseWriter, _ *http.Request) {
	status, err := ubus.GetWanStatus()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleWireless(w http.ResponseWriter, _ *http.Request) {
	radios, err := ubus.GetWirelessStatus()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, radios)
}

func (s *Server) handleLeases(w http.ResponseWriter, _ *http.Request) {
	leases, err := ubus.ReadLeases(leasesPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, leases)
}

func (s *Server) handleIPv6Get(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeIPv6())
}

type ipv6SetRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleIPv6Set(w http.ResponseWriter, r *http.Request) {
	var req ipv6SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetIPv6(req.Enabled)
	result := map[string]any{
		"state":       probe,
		"rolled_back": rolledBack,
	}
	if err != nil {
		result["error"] = err.Error()
		if rolledBack {
			result["status"] = "rolled_back"
		} else {
			result["status"] = "failed"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(result)
		return
	}
	result["status"] = "applied"
	writeJSON(w, result)
}

type passwordSetRequest struct {
	Current string `json:"current"`
	Next    string `json:"next"`
}

func (s *Server) handlePasswordSet(w http.ResponseWriter, r *http.Request) {
	var req passwordSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := modules.ChangePassword(s.rpcdURL, req.Current, req.Next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// The password changed: every existing panel session must die, including
	// the caller's. The user logs in again with the new password.
	auth.BumpEpoch()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.CheckUpdate())
}

type updateStartRequest struct {
	Confirm bool `json:"confirm"`
}

func (s *Server) handleUpdateStart(w http.ResponseWriter, r *http.Request) {
	var req updateStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Confirm {
		writeError(w, http.StatusBadRequest, "explicit confirmation required")
		return
	}
	check := modules.CheckUpdate()
	if !check.OwutPresent {
		writeError(w, http.StatusBadGateway, "owut is not installed on this router")
		return
	}
	if !check.SafeToProceed && !check.SafeWithReinstall {
		writeError(w, http.StatusConflict, "owut reports it is not safe to proceed")
		return
	}
	if err := modules.StartUpgrade(check.SafeWithReinstall && !check.SafeToProceed); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"started": true, "reboot_pending": true})
}

func (s *Server) handleWGGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeWG())
}

type wgSetRequest struct {
	Action string `json:"action"` // enable | disable
}

func (s *Server) handleWGSet(w http.ResponseWriter, r *http.Request) {
	var req wgSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var probe *modules.WGProbe
	var rolledBack bool
	var err error
	switch req.Action {
	case "enable":
		probe, rolledBack, err = modules.SetWG(true)
	case "disable":
		probe, rolledBack, err = modules.SetWG(false)
	default:
		writeError(w, http.StatusBadRequest, "action must be enable or disable")
		return
	}
	writeModuleResult(w, probe, rolledBack, err)
}

type wgPeerAddRequest struct {
	Name       string   `json:"name"`
	PublicKey  string   `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips"`
	Admin      bool     `json:"admin"`
}

func (s *Server) handleWGPeerAdd(w http.ResponseWriter, r *http.Request) {
	var req wgPeerAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.AddWGPeer(req.Name, req.PublicKey, req.AllowedIPs, req.Admin)
	writeModuleResult(w, probe, rolledBack, err)
}

type wgPeerQRRequest struct {
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
	Endpoint string `json:"endpoint"`
}

func (s *Server) handleWGPeerQR(w http.ResponseWriter, r *http.Request) {
	var req wgPeerQRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	config, probe, err := modules.AddWGPeerGenerated(req.Name, req.Admin, req.Endpoint)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"config": config, "state": probe})
}

type wgPeerDeleteRequest struct {
	PublicKey string `json:"public_key"`
}

func (s *Server) handleWGPeerDelete(w http.ResponseWriter, r *http.Request) {
	var req wgPeerDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.RemoveWGPeer(req.PublicKey)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleDDNSGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeDDNS())
}

func (s *Server) handleDDNSSet(w http.ResponseWriter, r *http.Request) {
	var cfg modules.DDNSConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetDDNS(cfg)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleSQMGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeSQM())
}

func (s *Server) handleSQMSet(w http.ResponseWriter, r *http.Request) {
	var cfg modules.SQMConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetSQM(cfg)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleOVPNGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeOVPN())
}

type ovpnSetRequest struct {
	Action string `json:"action"` // enable | disable
}

func (s *Server) handleOVPNSet(w http.ResponseWriter, r *http.Request) {
	var req ovpnSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var probe *modules.OVPNProbe
	var rolledBack bool
	var err error
	switch req.Action {
	case "enable":
		probe, rolledBack, err = modules.SetOVPN(true)
	case "disable":
		probe, rolledBack, err = modules.SetOVPN(false)
	default:
		writeError(w, http.StatusBadRequest, "action must be enable or disable")
		return
	}
	writeModuleResult(w, probe, rolledBack, err)
}

type ovpnClientAddRequest struct {
	Name   string `json:"name"`
	Remote string `json:"remote"`
}

func (s *Server) handleOVPNClientAdd(w http.ResponseWriter, r *http.Request) {
	var req ovpnClientAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	config, probe, err := modules.AddOVPNClient(req.Name, req.Remote)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"config": config, "state": probe})
}

type ovpnClientDeleteRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleOVPNClientDelete(w http.ResponseWriter, r *http.Request) {
	var req ovpnClientDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, err := modules.RemoveOVPNClient(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"state": probe})
}

func (s *Server) handlePackagesGet(w http.ResponseWriter, _ *http.Request) {
	pkgs, err := modules.ListUpgradable()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"upgradable": pkgs})
}

type packageUpgradeRequest struct {
	Name string `json:"name"`
}

func (s *Server) handlePackageUpgrade(w http.ResponseWriter, r *http.Request) {
	var req packageUpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	pkgs, err := modules.UpgradePackage(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"upgradable": pkgs})
}

func (s *Server) handleIoTGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeIoT())
}

func (s *Server) handleIoTSet(w http.ResponseWriter, r *http.Request) {
	var cfg modules.IoTConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetIoT(cfg)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleFwdGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeFwd())
}

type fwdAddRequest struct {
	SrcDport string `json:"src_dport"`
	DestIP   string `json:"dest_ip"`
	DestPort string `json:"dest_port"`
	Proto    string `json:"proto"`
}

func (s *Server) handleFwdAdd(w http.ResponseWriter, r *http.Request) {
	var req fwdAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.AddFwdRule(req.SrcDport, req.DestIP, req.DestPort, req.Proto)
	writeModuleResult(w, probe, rolledBack, err)
}

type fwdDeleteRequest struct {
	Section string `json:"section"`
}

func (s *Server) handleFwdDelete(w http.ResponseWriter, r *http.Request) {
	var req fwdDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.RemoveFwdRule(req.Section)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleTSGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeTailscale())
}

type tsSetRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleTSSet(w http.ResponseWriter, r *http.Request) {
	var req tsSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetTailscale(req.Enabled)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleGuestGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeGuest())
}

func (s *Server) handleGuestSet(w http.ResponseWriter, r *http.Request) {
	var cfg modules.GuestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetGuest(cfg)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleMode(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeMode())
}

type modeSetRequest struct {
	Target  string `json:"target"`
	Confirm bool   `json:"confirm"`
}

func (s *Server) handleModeSet(w http.ResponseWriter, r *http.Request) {
	var req modeSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !req.Confirm {
		writeError(w, http.StatusBadRequest, "explicit confirmation required")
		return
	}
	probe, rolledBack, err := modules.SetMode(req.Target)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleAccessGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeAccess())
}

type accessSetRequest struct {
	// Exactly one of the following targets is applied per request.
	Target       string                `json:"target"` // luci | ssh | panel_session
	Luci         modules.LuciAccess    `json:"luci"`
	SSH          modules.SSHAccess     `json:"ssh"`
	SessionTtlM  int                   `json:"session_ttl_minutes"`
}

func (s *Server) handleAccessSet(w http.ResponseWriter, r *http.Request) {
	var req accessSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	switch req.Target {
	case "luci":
		probe, rolledBack, err := modules.SetLuciAccess(req.Luci)
		writeModuleResult(w, probe, rolledBack, err)
	case "ssh":
		probe, rolledBack, err := modules.SetSSHAccess(req.SSH)
		writeModuleResult(w, probe, rolledBack, err)
	case "panel_session":
		if req.SessionTtlM <= 0 {
			writeError(w, http.StatusBadRequest, "session_ttl_minutes must be > 0")
			return
		}
		if err := modules.SetPanelSessionTTL(req.SessionTtlM); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]any{"status": "applied", "panel": modules.ProbeAccess().Panel})
	default:
		writeError(w, http.StatusBadRequest, "target must be luci, ssh or panel_session")
	}
}

func (s *Server) handleRemoteGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeRemoteAccess())
}

type remoteSetRequest struct {
	PingWAN     *bool `json:"ping_wan"`
	RemoteHTTPS *bool `json:"remote_https"`
	RemoteSSH   *bool `json:"remote_ssh"`
}

func (s *Server) handleRemoteSet(w http.ResponseWriter, r *http.Request) {
	var req remoteSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetRemoteAccess(req.PingWAN, req.RemoteHTTPS, req.RemoteSSH)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleOffloadGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeOffload())
}

type offloadSetRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleOffloadSet(w http.ResponseWriter, r *http.Request) {
	var req offloadSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetOffload(req.Enabled)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleWifiGet(w http.ResponseWriter, _ *http.Request) {
	ui, err := modules.ProbeWifiUI()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"interfaces": ui})
}

func (s *Server) handleWifiSet(w http.ResponseWriter, r *http.Request) {
	var edit modules.WifiEdit
	if err := json.NewDecoder(r.Body).Decode(&edit); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetWifi(edit)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleLANGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeLAN())
}

type lanSetRequest struct {
	IpAddr      *string `json:"ipaddr,omitempty"`
	Netmask     *string `json:"netmask,omitempty"`
	ApIsolation *bool   `json:"ap_isolation,omitempty"`
}

func (s *Server) handleLANSet(w http.ResponseWriter, r *http.Request) {
	var req lanSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg := struct {
		IpAddr      *string `json:"ipaddr,omitempty"`
		Netmask     *string `json:"netmask,omitempty"`
		ApIsolation *bool   `json:"ap_isolation,omitempty"`
	}{req.IpAddr, req.Netmask, req.ApIsolation}
	probe, rolledBack, err := modules.SetLAN(cfg)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleDHCPSet(w http.ResponseWriter, r *http.Request) {
	var cfg modules.DHCPConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetDHCP(cfg)
	writeModuleResult(w, probe, rolledBack, err)
}

type reservationSetRequest struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Name     string `json:"name,omitempty"`
	Reserved bool   `json:"reserved"`
}

func (s *Server) handleReservationSet(w http.ResponseWriter, r *http.Request) {
	var req reservationSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetReservation(req.MAC, req.IP, req.Name, req.Reserved)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleReservationsClear(w http.ResponseWriter, _ *http.Request) {
	probe, rolledBack, err := modules.ClearReservations()
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleDNSGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeDNS())
}

type dnsSetRequest struct {
	RebindProtect *bool `json:"rebind_protection,omitempty"`
	OverrideDNS   *bool `json:"override_dns,omitempty"`
	DnsVpn        *bool `json:"dns_vpn,omitempty"`
}

func (s *Server) handleDNSSet(w http.ResponseWriter, r *http.Request) {
	var req dnsSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetDNS(req.RebindProtect, req.OverrideDNS, req.DnsVpn)
	writeModuleResult(w, probe, rolledBack, err)
}

type dnsHostsSetRequest struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Remove   bool   `json:"remove"`
}

func (s *Server) handleDNSHostsSet(w http.ResponseWriter, r *http.Request) {
	var req dnsHostsSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, rolledBack, err := modules.SetHosts(req.IP, req.Hostname, req.Remove)
	writeModuleResult(w, probe, rolledBack, err)
}

func (s *Server) handleNetDev(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"counters": modules.NetDevCounters(), "ts": time.Now().UnixMilli()})
}

func (s *Server) handleEthPorts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"ports": modules.EthPorts()})
}

func (s *Server) handleDawn(w http.ResponseWriter, _ *http.Request) {
	aps, err := modules.DawnNetwork()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{"aps": aps})
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	requesterIP, _, _ := strings.Cut(r.RemoteAddr, ":")
	writeJSON(w, map[string]any{"clients": modules.ListClients(requesterIP), "ts": time.Now().UnixMilli()})
}

type clientReserveRequest struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Reserved bool   `json:"reserved"`
}

func (s *Server) handleClientReserve(w http.ResponseWriter, r *http.Request) {
	var req clientReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	_, rolledBack, err := modules.SetClientReservation(req.MAC, req.IP, req.Reserved)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"rolled_back": rolledBack, "status": "applied"})
}

type clientBlockRequest struct {
	MAC     string `json:"mac"`
	Type    string `json:"type"`
	Blocked bool   `json:"blocked"`
}

func (s *Server) handleClientBlock(w http.ResponseWriter, r *http.Request) {
	var req clientBlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	_, rolledBack, err := modules.SetClientBlocked(req.MAC, req.Type, req.Blocked)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"rolled_back": rolledBack, "status": "applied"})
}

// writeModuleResult is the shared response shape for write modules:
// the new probe, whether a rollback happened, and the error if any.
func writeModuleResult(w http.ResponseWriter, probe any, rolledBack bool, err error) {
	result := map[string]any{"state": probe, "rolled_back": rolledBack}
	if err != nil {
		result["error"] = err.Error()
		if rolledBack {
			result["status"] = "rolled_back"
		} else {
			result["status"] = "failed"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(result)
		return
	}
	result["status"] = "applied"
	writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleSnapshotsList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"snapshots": modules.ListSnapshots()})
}

func (s *Server) handleSnapshotCreate(w http.ResponseWriter, _ *http.Request) {
	snap, err := modules.CreateSnapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, snap)
}

func (s *Server) handleSnapshotDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id query param required")
		return
	}
	if err := modules.DeleteSnapshot(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSnapshotDiff(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to query params required")
		return
	}
	diffs, err := modules.DiffSnapshots(from, to)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"diffs": diffs})
}

type rollbackRequest struct {
	ID string `json:"id"`
}

func (s *Server) handleSnapshotRollback(w http.ResponseWriter, r *http.Request) {
	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "snapshot id required")
		return
	}
	if err := modules.RollbackSnapshot(req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "applied"})
}

type bounceRequest struct {
	Iface string `json:"iface"`
}

func (s *Server) handlePortBounce(w http.ResponseWriter, r *http.Request) {
	var req bounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Iface == "" {
		writeError(w, http.StatusBadRequest, "iface required")
		return
	}
	result, err := modules.BounceLink(req.Iface)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleIGMPGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.ProbeIGMP())
}

type igmpSetRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleIGMPSet(w http.ResponseWriter, r *http.Request) {
	var req igmpSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	probe, err := modules.SetIGMP(req.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, probe)
}

func (s *Server) handleLoops(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.DetectLoops())
}

func (s *Server) handleSelfUpdateCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, modules.CheckSelfUpdate(s.version))
}

type selfUpdateApplyRequest struct {
	Confirm bool `json:"confirm"`
}

func (s *Server) handleSelfUpdateApply(w http.ResponseWriter, r *http.Request) {
	var req selfUpdateApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Confirm {
		writeError(w, http.StatusBadRequest, "explicit confirmation required")
		return
	}
	check := modules.CheckSelfUpdate(s.version)
	if !check.Available {
		writeError(w, http.StatusConflict, "no update available")
		return
	}
	if check.AssetURL == "" {
		writeError(w, http.StatusBadGateway, "no binary asset found in release")
		return
	}
	if err := modules.ApplySelfUpdate(check.AssetURL); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"status": "applied", "restarting": true})
}
