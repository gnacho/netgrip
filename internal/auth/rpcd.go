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
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "call",
		Params: []any{
			"00000000000000000000000000000000",
			"session",
			"login",
			map[string]string{"username": username, "password": password},
		},
	})
	if err != nil {
		return false, fmt.Errorf("marshal rpc request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(rpcdURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("rpcd login request: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return false, fmt.Errorf("decode rpcd response: %w", err)
	}
	// Success: result = [0, {"ubus_rpc_session": "..."}]. Failure: result = [6].
	if len(rpcResp.Result) < 2 {
		return false, nil
	}
	var loginResult struct {
		Session string `json:"ubus_rpc_session"`
	}
	if err := json.Unmarshal(rpcResp.Result[1], &loginResult); err != nil {
		return false, nil
	}
	return loginResult.Session != "", nil
}
