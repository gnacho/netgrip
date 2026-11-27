package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// Settings is the panel view of the endpoint configuration (#807). The keys
// live in UCI (netgrip.mcp.*), the same three the auth layer reads on every
// request, so saving here takes effect without restarting the service.
type Settings struct {
	Enabled    bool   `json:"enabled"`
	Token      string `json:"token"`
	RatePerMin int    `json:"rate_per_min"`
	Tools      int    `json:"tools"`
}

const (
	defaultRatePerMin = 60
	minRatePerMin     = 10
	maxRatePerMin     = 600
	minTokenLen       = 16
)

// Writes go through variables for the same reason reads do: tests assert the
// exact uci commands, and a dev machine without uci never touches a config.
var (
	uciSet = func(kv string) error {
		out, err := exec.Command("uci", "set", kv).CombinedOutput()
		if err != nil {
			return errors.New("uci set " + kv + ": " + strings.TrimSpace(string(out)))
		}
		return nil
	}
	uciCommit = func(pkg string) error {
		out, err := exec.Command("uci", "commit", pkg).CombinedOutput()
		if err != nil {
			return errors.New("uci commit " + pkg + ": " + strings.TrimSpace(string(out)))
		}
		return nil
	}
)

// ReadSettings loads the configuration. Missing keys mean disabled, no token
// and the default rate, which is how a fresh install starts.
func ReadSettings() Settings {
	s := Settings{Enabled: uciGet(uciEnabled) == "1", Token: uciGet(uciToken), RatePerMin: defaultRatePerMin}
	if n, err := strconv.Atoi(uciGet(uciRatePerMin)); err == nil && n >= minRatePerMin && n <= maxRatePerMin {
		s.RatePerMin = n
	}
	return s
}

// SaveSettings persists the configuration. Enabling without a usable token is
// refused: the handler would keep answering 404, which reads as a bug.
func SaveSettings(s Settings) (Settings, error) {
	s.Token = strings.TrimSpace(s.Token)
	if strings.ContainsAny(s.Token, " \t\r\n") {
		return s, errors.New("the token cannot contain spaces")
	}
	if s.Enabled && len(s.Token) < minTokenLen {
		return s, errors.New("a token of at least 16 characters is required to enable the endpoint")
	}
	switch {
	case s.RatePerMin <= 0:
		s.RatePerMin = defaultRatePerMin
	case s.RatePerMin < minRatePerMin:
		s.RatePerMin = minRatePerMin
	case s.RatePerMin > maxRatePerMin:
		s.RatePerMin = maxRatePerMin
	}
	sets := []string{
		"netgrip.mcp=mcp",
		"netgrip.mcp.enabled=" + onOff(s.Enabled),
		"netgrip.mcp.rate_per_min=" + strconv.Itoa(s.RatePerMin),
	}
	// Disabling keeps the current token, so switching it back on does not
	// force clients to be reconfigured.
	if s.Token != "" {
		sets = append(sets, "netgrip.mcp.token="+s.Token)
	}
	for _, kv := range sets {
		if err := uciSet(kv); err != nil {
			return s, err
		}
	}
	if err := uciCommit("netgrip"); err != nil {
		return s, err
	}
	return s, nil
}

// NewToken returns a 32-byte hex token. The endpoint rate-limits per IP, so
// the token is the only barrier an attacker has to guess.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func onOff(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
