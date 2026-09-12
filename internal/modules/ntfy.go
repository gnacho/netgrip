package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// ntfyConfigPath is a var (not const) so tests can point it at a temp dir.
var ntfyConfigPath = "/etc/netgrip/ntfy.json"

const (
	ntfyDefaultServer = "https://ntfy.sh"
	ntfySendTimeout   = 10 * time.Second
	ntfyMaxMsgLen     = 4096
)

// NtfyConfig is the persisted ntfy channel configuration. The token is
// write-only: it is stored but never returned by the API.
type NtfyConfig struct {
	Server  string `json:"server"`
	Topic   string `json:"topic"`
	Token   string `json:"token,omitempty"`
	Enabled bool   `json:"enabled"`
}

func LoadNtfyConfig() NtfyConfig {
	data, err := os.ReadFile(ntfyConfigPath)
	if err != nil {
		return NtfyConfig{}
	}
	var cfg NtfyConfig
	if json.Unmarshal(data, &cfg) != nil {
		return NtfyConfig{}
	}
	return cfg
}

// SaveNtfyConfig validates and persists. Server must be http(s) (empty falls
// back to ntfy.sh); topic must match [a-zA-Z0-9_-]{1,64} (empty clears the
// channel, which is allowed).
func SaveNtfyConfig(cfg NtfyConfig) error {
	cfg.Server = strings.TrimRight(strings.TrimSpace(cfg.Server), "/")
	if cfg.Server == "" {
		cfg.Server = ntfyDefaultServer
	}
	if !strings.HasPrefix(cfg.Server, "http://") && !strings.HasPrefix(cfg.Server, "https://") {
		return fmt.Errorf("server must be an http(s) URL")
	}
	cfg.Topic = strings.TrimSpace(cfg.Topic)
	if cfg.Topic != "" && !validNtfyTopic(cfg.Topic) {
		return fmt.Errorf("invalid topic (letters, numbers, - and _, max 64)")
	}
	cfg.Token = strings.TrimSpace(cfg.Token)
	if err := os.MkdirAll(filepath.Dir(ntfyConfigPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(ntfyConfigPath, data, 0600)
}

func validNtfyTopic(t string) bool {
	if len(t) == 0 || len(t) > 64 {
		return false
	}
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// SendNtfy publishes a plain-text message to <server>/<topic> with Title and
// Priority headers. It is synchronous (no worker queue: the monitor is low
// frequency) with a 10s timeout and at most one retry on transient failures.
func SendNtfy(cfg NtfyConfig, title, text string, urgent bool) error {
	if cfg.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	body := text
	if len(body) > ntfyMaxMsgLen {
		body = truncateUTF8(body, ntfyMaxMsgLen-3) + "..."
	}
	priority := "default"
	if urgent {
		priority = "urgent"
	}
	err := publishNtfy(cfg, title, body, priority)
	if err == nil || !isRetryableNtfy(err) {
		return err
	}
	return publishNtfy(cfg, title, body, priority)
}

// SendNtfyTest publishes a real test message against the saved config. It is
// the channel validation (ntfy has no getMe) and requires the channel to be
// enabled, same as the send path.
func SendNtfyTest(cfg NtfyConfig) error {
	if !cfg.Enabled {
		return fmt.Errorf("ntfy channel is disabled")
	}
	if cfg.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	return SendNtfy(cfg, "NetGrip", "✅ NetGrip ntfy notifications configured correctly.", false)
}

func publishNtfy(cfg NtfyConfig, title, body, priority string) error {
	url := cfg.Server + "/" + cfg.Topic
	ctx, cancel := context.WithTimeout(context.Background(), ntfySendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(body)))
	if err != nil {
		return redactNtfyErr("new request", err, cfg)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Priority", priority)
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return redactNtfyErr("http do", err, cfg)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return redactNtfyErr("publish", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody))), cfg)
}

// ntfyRedactedError keeps the error chain (Unwrap) for isRetryableNtfy but
// exposes a message without the topic, which is the channel secret.
type ntfyRedactedError struct {
	err error
	msg string
}

func (e *ntfyRedactedError) Error() string { return e.msg }
func (e *ntfyRedactedError) Unwrap() error { return e.err }

// redactNtfyErr replaces the topic (and the full <server>/<topic> URL) with
// "***" in any error that might include it, so logs and API responses never
// leak the channel secret.
func redactNtfyErr(prefix string, err error, cfg NtfyConfig) error {
	msg := err.Error()
	if cfg.Topic != "" {
		msg = strings.ReplaceAll(msg, cfg.Server+"/"+cfg.Topic, cfg.Server+"/***")
		msg = strings.ReplaceAll(msg, cfg.Topic, "***")
	}
	return &ntfyRedactedError{err: err, msg: prefix + ": " + msg}
}

// isRetryableNtfy decides whether a publish failure deserves the single retry:
// only 5xx, 429 and transient network errors (timeout, connection
// refused/reset). Permanent failures (4xx, invalid URL, DNS, TLS, context) are
// not retried.
func isRetryableNtfy(err error) bool {
	if err == nil {
		return false
	}
	if code, ok := ntfyStatusCode(err.Error()); ok {
		return code >= 500 || code == 429
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	for _, target := range []error{
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.ECONNABORTED,
		syscall.EPIPE,
		io.ErrUnexpectedEOF,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// ntfyStatusCode extracts the code from an error formatted "status NNN: ...".
func ntfyStatusCode(msg string) (int, bool) {
	const prefix = "status "
	i := strings.Index(msg, prefix)
	if i < 0 {
		return 0, false
	}
	code, digits := 0, 0
	for _, r := range msg[i+len(prefix):] {
		if r < '0' || r > '9' {
			break
		}
		code = code*10 + int(r-'0')
		digits++
	}
	if digits == 0 {
		return 0, false
	}
	return code, true
}

// truncateUTF8 cuts s to at most maxBytes bytes without splitting a multibyte
// rune (a raw byte cut could leave invalid UTF-8).
func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r != utf8.RuneError || size > 1 {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
