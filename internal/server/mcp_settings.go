package server

import (
	"encoding/json"
	"net/http"

	"github.com/gnacho/netgrip/internal/mcp"
)

// mcpSettingsView adds the URL a client should paste, which depends on the
// request (scheme and host) and cannot live in UCI (#807).
type mcpSettingsView struct {
	mcp.Settings
	URL string `json:"url"`
}

func (s *Server) handleMCPGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.mcpView(r))
}

func (s *Server) mcpView(r *http.Request) mcpSettingsView {
	st := mcp.ReadSettings()
	st.Tools = s.mcpTools
	return mcpSettingsView{Settings: st, URL: mcpURL(r)}
}

func mcpURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/mcp"
}

type mcpSetRequest struct {
	Enabled    bool   `json:"enabled"`
	Token      string `json:"token"`
	RatePerMin int    `json:"rate_per_min"`
}

func (s *Server) handleMCPSet(w http.ResponseWriter, r *http.Request) {
	var req mcpSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	cfg := mcp.Settings{Enabled: req.Enabled, Token: req.Token, RatePerMin: req.RatePerMin}
	if _, err := mcp.SaveSettings(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, s.mcpView(r))
}

// handleMCPNewToken issues a token without saving it: the panel fills the
// field and the user decides when to persist (and so when to break the old
// client).
func (s *Server) handleMCPNewToken(w http.ResponseWriter, _ *http.Request) {
	tok, err := mcp.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"token": tok})
}
