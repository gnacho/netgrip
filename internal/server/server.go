package server

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
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
	rpcdURL  string
	mux      *http.ServeMux
	mu       sync.Mutex
	sessions map[string]time.Time
}

func New(rpcdURL string) *Server {
	s := &Server{
		rpcdURL:  rpcdURL,
		mux:      http.NewServeMux(),
		sessions: make(map[string]time.Time),
	}
	s.mux.HandleFunc("/", s.handleSPA)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
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
	token := newToken()
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
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
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || !s.validSession(c.Value) {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		next(w, r)
	}
}

func (s *Server) validSession(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiry, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(s.sessions, token)
		return false
	}
	return true
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
	s.mu.Lock()
	s.sessions = make(map[string]time.Time)
	s.mu.Unlock()
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
	if !check.SafeToProceed {
		writeError(w, http.StatusConflict, "owut reports it is not safe to proceed")
		return
	}
	if err := modules.StartUpgrade(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"started": true, "reboot_pending": true})
}

func newToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
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
