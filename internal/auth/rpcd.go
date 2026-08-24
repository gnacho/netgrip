package auth

import (
	"bytes"
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

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(rpcdURL, "application/json", bytes.NewReader(body))
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
