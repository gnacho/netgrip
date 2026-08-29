package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const telegramConfigPath = "/etc/netgrip/telegram.json"

type TelegramConfig struct {
	BotToken string `json:"botToken"`
	ChatID   string `json:"chatId"`
	Enabled  bool   `json:"enabled"`
}

func LoadTelegramConfig() TelegramConfig {
	data, err := os.ReadFile(telegramConfigPath)
	if err != nil {
		return TelegramConfig{}
	}
	var cfg TelegramConfig
	if json.Unmarshal(data, &cfg) != nil {
		return TelegramConfig{}
	}
	return cfg
}

func SaveTelegramConfig(cfg TelegramConfig) error {
	if err := os.MkdirAll(filepath.Dir(telegramConfigPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(telegramConfigPath, data, 0600)
}

type telegramPayload struct {
	ChatID              string `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func SendTelegram(cfg TelegramConfig, text string, urgent bool) error {
	payload := telegramPayload{
		ChatID:              cfg.ChatID,
		Text:                text,
		ParseMode:           "HTML",
		DisableNotification: !urgent,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var apiResp telegramResponse
	if json.Unmarshal(respBody, &apiResp) != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	if !apiResp.OK {
		return fmt.Errorf("telegram %d: %s", apiResp.ErrorCode, apiResp.Description)
	}
	return nil
}

func ValidateTelegram(botToken, chatID string) (botName string, chatTitle string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", botToken))
	if err != nil {
		return "", "", fmt.Errorf("getMe: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var meResp struct {
		OK     bool `json:"ok"`
		Result struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if json.Unmarshal(body, &meResp) != nil || !meResp.OK {
		desc := meResp.Description
		if desc == "" {
			desc = "invalid token"
		}
		return "", "", fmt.Errorf("getMe: %s", desc)
	}
	botName = meResp.Result.Username
	if botName == "" {
		botName = meResp.Result.FirstName
	}

	resp2, err := client.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%s", botToken, chatID))
	if err != nil {
		return botName, "", fmt.Errorf("getChat: %w", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 4096))

	var chatResp struct {
		OK     bool `json:"ok"`
		Result struct {
			Title     string `json:"title"`
			FirstName string `json:"first_name"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if json.Unmarshal(body2, &chatResp) != nil || !chatResp.OK {
		desc := chatResp.Description
		if desc == "" {
			desc = "invalid chat ID"
		}
		return botName, "", fmt.Errorf("getChat: %s", desc)
	}
	chatTitle = chatResp.Result.Title
	if chatTitle == "" {
		chatTitle = chatResp.Result.FirstName
	}
	return botName, chatTitle, nil
}

func htmlEsc(s string) string {
	r := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			r = append(r, '&', 'l', 't', ';')
		case '>':
			r = append(r, '&', 'g', 't', ';')
		case '&':
			r = append(r, '&', 'a', 'm', 'p', ';')
		default:
			r = append(r, s[i])
		}
	}
	return string(r)
}
