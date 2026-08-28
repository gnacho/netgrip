package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Session tokens are stateless and HMAC-signed, so they survive service
// restarts (the old in-memory map logged everyone out on every restart).
//
// Format: epoch.expiry.nonce.signature
//   signature = hex(hmac-sha256(secret, "epoch.expiry.nonce"))
//
// secret: /etc/netgrip.secret (32 random bytes, 0600, created on first use)
// epoch:  /etc/netgrip.epoch (integer; bumped on password change so all
//         previously issued tokens die at once)
const (
	secretPath = "/etc/netgrip.secret"
	epochPath  = "/etc/netgrip.epoch"
)

func secret() ([]byte, error) {
	data, err := os.ReadFile(secretPath)
	if err == nil && len(data) >= 32 {
		return data[:32], nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("rand secret: %w", err)
	}
	if err := os.WriteFile(secretPath, buf, 0o600); err != nil {
		return nil, fmt.Errorf("write secret: %w", err)
	}
	return buf, nil
}

func epoch() int {
	data, err := os.ReadFile(epochPath)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return n
}

// BumpEpoch invalidates every token issued so far (password change).
func BumpEpoch() {
	_ = os.WriteFile(epochPath, []byte(strconv.Itoa(epoch()+1)+"\n"), 0o600)
}

// NewSessionToken issues a signed token valid for ttl.
func NewSessionToken(ttl time.Duration) (string, error) {
	key, err := secret()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := fmt.Sprintf("%d.%d.%s", epoch(), time.Now().Add(ttl).Unix(), hex.EncodeToString(nonce))
	sig := sign(key, payload)
	return payload + "." + sig, nil
}

// ValidSessionToken reports whether the token is well-formed, correctly
// signed, from the current epoch and not expired.
func ValidSessionToken(token string) bool {
	key, err := secret()
	if err != nil {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return false
	}
	payload := strings.Join(parts[:3], ".")
	if !hmac.Equal([]byte(sign(key, payload)), []byte(parts[3])) {
		return false
	}
	tokenEpoch, err := strconv.Atoi(parts[0])
	if err != nil || tokenEpoch != epoch() {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	return true
}

func sign(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
