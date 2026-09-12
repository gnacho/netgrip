package server

import (
	"encoding/json"
	"net/http"

	"github.com/gnacho/netgrip/internal/modules"
)

func (s *Server) handleNtfyGet(w http.ResponseWriter, r *http.Request) {
	cfg := modules.LoadNtfyConfig()
	writeJSON(w, map[string]any{
		"server":   cfg.Server,
		"topic":    cfg.Topic,
		"tokenSet": cfg.Token != "",
		"enabled":  cfg.Enabled,
	})
}

func (s *Server) handleNtfySet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Server  string `json:"server"`
		Topic   string `json:"topic"`
		Token   string `json:"token"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_body"}`, http.StatusBadRequest)
		return
	}

	cfg := modules.LoadNtfyConfig()
	if body.Server != "" {
		cfg.Server = body.Server
	}
	if body.Topic != "" {
		cfg.Topic = body.Topic
	}
	if body.Token != "" {
		cfg.Token = body.Token
	}
	cfg.Enabled = body.Enabled

	if err := modules.SaveNtfyConfig(cfg); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleNtfyTest(w http.ResponseWriter, r *http.Request) {
	cfg := modules.LoadNtfyConfig()
	if !cfg.Enabled || cfg.Topic == "" {
		http.Error(w, `{"error":"ntfy not configured or disabled"}`, http.StatusBadRequest)
		return
	}
	if err := modules.SendNtfyTest(cfg); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
