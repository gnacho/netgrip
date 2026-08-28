package server

import (
	"encoding/json"
	"net/http"

	"github.com/gnacho/netgrip/internal/modules"
)

func (s *Server) handleTelegramGet(w http.ResponseWriter, r *http.Request) {
	cfg := modules.LoadTelegramConfig()
	resp := map[string]any{
		"chatId":  cfg.ChatID,
		"enabled": cfg.Enabled,
	}
	if cfg.BotToken != "" {
		if len(cfg.BotToken) > 10 {
			resp["botToken"] = cfg.BotToken[:6] + "..." + cfg.BotToken[len(cfg.BotToken)-4:]
		} else {
			resp["botToken"] = "***"
		}
	} else {
		resp["botToken"] = ""
	}
	writeJSON(w, resp)
}

func (s *Server) handleTelegramSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BotToken string `json:"botToken"`
		ChatID   string `json:"chatId"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid_body"}`, http.StatusBadRequest)
		return
	}

	cfg := modules.LoadTelegramConfig()
	if body.BotToken != "" {
		cfg.BotToken = body.BotToken
	}
	cfg.ChatID = body.ChatID
	cfg.Enabled = body.Enabled

	if cfg.BotToken == "" || cfg.ChatID == "" {
		http.Error(w, `{"error":"botToken and chatId are required"}`, http.StatusBadRequest)
		return
	}

	botName, chatName, err := modules.ValidateTelegram(cfg.BotToken, cfg.ChatID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := modules.SaveTelegramConfig(cfg); err != nil {
		http.Error(w, `{"error":"save_failed"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"ok":       true,
		"botName":  botName,
		"chatName": chatName,
	})
}

func (s *Server) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	cfg := modules.LoadTelegramConfig()
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" {
		http.Error(w, `{"error":"telegram not configured or disabled"}`, http.StatusBadRequest)
		return
	}
	text := "✅ <b>owpanel</b> Telegram notifications configured successfully."
	if err := modules.SendTelegram(cfg, text, true); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
