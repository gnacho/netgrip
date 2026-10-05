package mcp

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UCI keys that control the MCP endpoint. Everything lives in UCI (not in
// env or flags) so it survives self-updates: the binary alone never carries
// config, and the same mechanism the panel uses for its own settings applies.
const (
	uciEnabled    = "netgrip.mcp.enabled"
	uciToken      = "netgrip.mcp.token"
	uciRatePerMin = "netgrip.mcp.rate_per_min"
)

// uciGet reads one UCI option. A variable so tests can stub it: on a dev
// machine without uci it returns "" and the endpoint stays dark.
var uciGet = func(key string) string {
	out, err := exec.Command("uci", "-q", "get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Handler returns the /mcp HTTP handler: rate limit per IP -> enabled check
// -> bearer token auth -> streamable-HTTP MCP server. The chain goes in this
// order so token brute force is rate-limited too.
//
// Only the dedicated MCP token is accepted, never the session cookie: mixing
// AI client auth with the browser CSRF surface would let a cross-site request
// ride an active panel session into the tools.
func (s *Server) Handler() http.Handler {
	return rateLimit(s.ratePerMin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uciGet(uciEnabled) != "1" {
			// Disabled installs pretend the route does not exist.
			http.NotFound(w, r)
			return
		}
		token := uciGet(uciToken)
		if token == "" {
			// Enabled without a token is a misconfiguration, not an
			// invitation: same silent 404, and the panel log stays quiet
			// about an endpoint that was never armed.
			http.NotFound(w, r)
			return
		}
		if bearerToken(r) != token {
			unauthorized(w)
			return
		}
		s.http.ServeHTTP(w, r)
	}))
}

// bearerToken extracts the token from the Authorization header. Anything not
// shaped like "Bearer <token>" yields "".
func bearerToken(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(v, "Bearer ")
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

// rlBucket is one client's fixed window.
type rlBucket struct {
	window time.Time
	count  int
}

// rateLimit caps /mcp per IP (fixed 1-minute window). Stateless apart from
// this map: brute force on the token is bounded to perMin tries per minute.
// The UCI rate override is read lazily so it applies without a restart.
func rateLimit(perMin int, next http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := map[string]*rlBucket{}
	window := time.Minute
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := perMin
		if v := uciGet(uciRatePerMin); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		ip := clientIP(r)
		now := time.Now()
		mu.Lock()
		b := buckets[ip]
		if b == nil || now.Sub(b.window) >= window {
			b = &rlBucket{window: now}
			buckets[ip] = b
		}
		b.count++
		full := b.count > limit
		mu.Unlock()
		if full {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate_limited"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP uses RemoteAddr. NetGrip has no trusted-proxy mode; without that
// configuration X-Forwarded-For is not trustworthy and must not rate-limit.
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
