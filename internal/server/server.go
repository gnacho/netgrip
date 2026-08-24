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
	mux     *http.ServeMux
	mu      sync.Mutex
	revoked map[string]bool
}

func New(rpcdURL string) *Server {
	s := &Server{
		rpcdURL: rpcdURL,
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
	token, err := auth.NewSessionToken(sessionTTL)
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
		MaxAge:   int(sessionTTL.Seconds()),
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
