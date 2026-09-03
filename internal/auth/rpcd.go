package auth

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	Result []json.RawMessage `json:"result"`
}

// DefaultRPCdURL is the stock OpenWrt endpoint (uhttpd on port 80 proxies
// /ubus to rpcd).
const DefaultRPCdURL = "http://127.0.0.1/ubus"

// CandidateRPCdURLs lists the endpoints probed when no explicit override is
// set. GL.iNet firmware fronts port 80 with nginx (its own portal) and serves
// uhttpd on 8080/8443, redirecting /ubus to the HTTPS listener.
var CandidateRPCdURLs = []string{
	DefaultRPCdURL,
	"https://127.0.0.1:8443/ubus",
	"http://127.0.0.1:8080/ubus",
}

// rpcClient talks to the router's own rpcd over loopback. The TLS listener
// presents the firmware's self-signed uhttpd certificate, so verification is
// skipped; this is always a localhost call to the router itself, never a
// cross-host request. Redirects are not followed: an HTML 302/307 from a
// fronting web server must fail the call instead of being decoded.
var rpcClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// DetectRPCdEndpoint returns the first candidate endpoint that answers a
// JSON-RPC response, or "" when none does. An explicit endpoint is trusted
// as-is.
func DetectRPCdEndpoint(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, url := range CandidateRPCdURLs {
		if Probe(url) {
			return url
		}
	}
	return ""
}

// Probe reports whether the endpoint answers a decodable ubus JSON-RPC
// response. Any rpcd status (including permission-denied) counts: the goal is
// to tell rpcd apart from web servers answering HTML or redirects.
func Probe(rpcdURL string) bool {
	_, err := rpcCall(rpcdURL, "00000000000000000000000000000000", "session", "list", map[string]any{})
	return err == nil
}

// ValidatePassword checks credentials against the router's rpcd session login,
// the same mechanism LuCI uses. Nothing is stored: a valid rpcd session for
// the given user means the password is correct.
func ValidatePassword(rpcdURL, username, password string) (bool, error) {
	token, err := SessionLogin(rpcdURL, username, password)
	if err != nil {
		return false, err
	}
	return token != "", nil
}

// SessionLogin authenticates against rpcd and returns the session token
// ("" when the credentials are wrong).
func SessionLogin(rpcdURL, username, password string) (string, error) {
	result, err := rpcCall(rpcdURL, "00000000000000000000000000000000", "session", "login",
		map[string]string{"username": username, "password": password})
	if err != nil {
		return "", err
	}
	if len(result) < 2 {
		return "", nil
	}
	var loginResult struct {
		Session string `json:"ubus_rpc_session"`
	}
	if err := json.Unmarshal(result[1], &loginResult); err != nil {
		return "", nil
	}
	return loginResult.Session, nil
}

// Call invokes a ubus method over the rpcd HTTP endpoint with a session
// token. Keeps secrets (e.g. a new password) out of the process table,
// unlike `ubus call` CLI arguments.
func Call(rpcdURL, token, object, method string, params map[string]any) ([]json.RawMessage, error) {
	return rpcCall(rpcdURL, token, object, method, params)
}

func rpcCall(rpcdURL, token, object, method string, params any) ([]json.RawMessage, error) {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "call",
		Params:  []any{token, object, method, params},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal rpc request: %w", err)
	}

	resp, err := rpcClient.Post(rpcdURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("rpcd request %s %s: %w", object, method, err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode rpcd response: %w", err)
	}
	return rpcResp.Result, nil
}
