package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stubUCI replaces uciGet for the duration of a test. Keys not in the map
// return "".
func stubUCI(t *testing.T, values map[string]string) {
	t.Helper()
	orig := uciGet
	uciGet = func(key string) string { return values[key] }
	t.Cleanup(func() { uciGet = orig })
}

// testEnv mounts the MCP handler with the given UCI stub values.
func testEnv(t *testing.T, uci map[string]string, ratePerMin int) (string, string) {
	t.Helper()
	stubUCI(t, uci)
	srv := New(Deps{Version: "0.0.0-test", Started: time.Now(), RatePerMin: ratePerMin})
	mux := http.NewServeMux()
	mux.Handle("/mcp", srv.Handler())
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL, uci[uciToken]
}

// rpc posts one JSON-RPC 2.0 request to /mcp and returns status and body.
func rpc(t *testing.T, url, token, method string, id int, params any) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})
	req, err := http.NewRequest("POST", url+"/mcp", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return resp.StatusCode, nil
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("Decode (status %d): %v", resp.StatusCode, err)
	}
	return resp.StatusCode, out
}

func callTool(t *testing.T, url, token, name string, args map[string]any) map[string]any {
	t.Helper()
	_, out := rpc(t, url, token, "tools/call", 2, map[string]any{
		"name":      name,
		"arguments": args,
	})
	return out
}

// structured extracts the JSON text content of a CallToolResult.
func structured(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	res, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", out)
	}
	if res["isError"] == true {
		t.Fatalf("tool error: %v", res)
	}
	content, ok := res["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in %v", res)
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] is not a map: %v", content[0])
	}
	text, ok := first["text"].(string)
	if !ok {
		t.Fatalf("content[0].text is not a string: %v", first)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("content[0].text is not JSON (%q): %v", text, err)
	}
	return v
}

var enabledUCI = map[string]string{
	uciEnabled: "1",
	uciToken:   "test-token-1234567890",
}

func TestDisabledIsNotFound(t *testing.T) {
	url, _ := testEnv(t, map[string]string{}, 60)

	for _, token := range []string{"", "test-token-1234567890"} {
		status, _ := rpc(t, url, token, "tools/list", 1, nil)
		if status != http.StatusNotFound {
			t.Errorf("disabled with token %q: status %d, want 404", token, status)
		}
	}
}

func TestEnabledWithoutTokenIsNotFound(t *testing.T) {
	url, _ := testEnv(t, map[string]string{uciEnabled: "1"}, 60)

	status, _ := rpc(t, url, "whatever", "tools/list", 1, nil)
	if status != http.StatusNotFound {
		t.Errorf("enabled without token: status %d, want 404", status)
	}
}

func TestAuthBearerOnly(t *testing.T) {
	url, tok := testEnv(t, enabledUCI, 60)

	status, _ := rpc(t, url, "", "tools/list", 1, nil)
	if status != http.StatusUnauthorized {
		t.Errorf("no token: status %d", status)
	}
	status, _ = rpc(t, url, "bogus", "tools/list", 1, nil)
	if status != http.StatusUnauthorized {
		t.Errorf("bad token: status %d", status)
	}
	status, _ = rpc(t, url, tok, "tools/list", 1, nil)
	if status != http.StatusOK {
		t.Errorf("valid token: status %d", status)
	}
}

func TestRateLimit(t *testing.T) {
	url, tok := testEnv(t, enabledUCI, 2)

	for i := 1; i <= 2; i++ {
		status, _ := rpc(t, url, tok, "tools/list", i, nil)
		if status != http.StatusOK {
			t.Fatalf("request %d: status %d", i, status)
		}
	}
	status, body := rpc(t, url, tok, "tools/list", 3, nil)
	if status != http.StatusTooManyRequests {
		t.Errorf("request 3: status %d body %v", status, body)
	}
}

func TestRateLimitUCIOverride(t *testing.T) {
	uci := map[string]string{
		uciEnabled:    "1",
		uciToken:      "test-token-1234567890",
		uciRatePerMin: "1",
	}
	url, tok := testEnv(t, uci, 60)

	status, _ := rpc(t, url, tok, "tools/list", 1, nil)
	if status != http.StatusOK {
		t.Fatalf("request 1: status %d", status)
	}
	status, _ = rpc(t, url, tok, "tools/list", 2, nil)
	if status != http.StatusTooManyRequests {
		t.Errorf("request 2 with UCI limit 1: status %d", status)
	}
}

func TestToolsListAllReadOnly(t *testing.T) {
	url, tok := testEnv(t, enabledUCI, 60)

	_, out := rpc(t, url, tok, "tools/list", 1, nil)
	res, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", out)
	}
	tools, ok := res["tools"].([]any)
	if !ok || len(tools) != 7 {
		t.Fatalf("tools = %v", res["tools"])
	}
	want := map[string]bool{
		"server_status": true, "list_clients": true, "wireless_status": true,
		"wan_status": true, "multiwan_status": true, "list_leases": true,
		"list_devices": true,
	}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		name := tool["name"].(string)
		if !want[name] {
			t.Errorf("unexpected tool: %s", name)
		}
		delete(want, name)
		ann, ok := tool["annotations"].(map[string]any)
		if !ok {
			t.Errorf("%s has no annotations", name)
			continue
		}
		if ann["readOnlyHint"] != true {
			t.Errorf("%s readOnlyHint != true: %v", name, ann)
		}
		for _, forbidden := range []string{"destructiveHint", "openWorldHint"} {
			if ann[forbidden] == true {
				t.Errorf("%s must not declare %s=true", name, forbidden)
			}
		}
	}
	if len(want) > 0 {
		t.Errorf("tools not registered: %v", want)
	}
}

// TestServerStatusNoUbus verifies the tool degrades gracefully on a host
// without ubus (the dev machine): identity and uptime are still reported,
// counts come back empty instead of failing, and no secret leaks.
func TestServerStatusNoUbus(t *testing.T) {
	url, tok := testEnv(t, enabledUCI, 60)

	v := structured(t, callTool(t, url, tok, "server_status", nil))
	if v["name"] != "NetGrip" || v["version"] != "0.0.0-test" {
		t.Errorf("identity = %v", v)
	}
	if v["uptime_sec"] == nil {
		t.Errorf("uptime missing: %v", v)
	}
	clients, ok := v["clients"].(map[string]any)
	if !ok {
		t.Fatalf("clients missing: %v", v)
	}
	if total, ok := clients["total"].(float64); !ok || total != 0 {
		t.Errorf("clients.total = %v, want 0 without ubus", clients["total"])
	}
	if _, leaked := v["rpcd_url"]; leaked {
		t.Errorf("server_status must not expose internal URLs: %v", v)
	}
}

// TestServerStatusThenListClientsShareCache reproduces the rt-lab live bug:
// server_status populates the "clients|mcp" cache entry through its counts,
// and a later list_clients must still see the full payload (bands included),
// not the counts-only shape.
func TestServerStatusThenListClientsShareCache(t *testing.T) {
	url, tok := testEnv(t, enabledUCI, 60)

	structured(t, callTool(t, url, tok, "server_status", nil))
	v := structured(t, callTool(t, url, tok, "list_clients", nil))
	if _, ok := v["bands"].([]any); !ok {
		t.Errorf("list_clients after server_status: bands missing in %v", v)
	}
	if _, ok := v["clients"].([]any); !ok {
		t.Errorf("list_clients after server_status: clients missing in %v", v)
	}
}

// TestReadToolsDegradeWithoutUbus verifies probes that need ubus either fail
// as isError tool results or come back empty on a host without it (the dev
// machine): never as transport failures, never with stale data.
func TestReadToolsDegradeWithoutUbus(t *testing.T) {
	url, tok := testEnv(t, enabledUCI, 60)

	// wireless_status shells out to ubus directly and must surface the
	// failure as an isError result.
	out := callTool(t, url, tok, "wireless_status", nil)
	res, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("wireless_status: no result: %v", out)
	}
	if res["isError"] != true {
		t.Errorf("wireless_status: want isError without ubus, got %v", res)
	}

	// These probes degrade to empty results when their sources are missing.
	for _, name := range []string{"wan_status", "list_leases", "list_devices", "list_clients"} {
		out := callTool(t, url, tok, name, nil)
		res, ok := out["result"].(map[string]any)
		if !ok {
			t.Fatalf("%s: no result: %v", name, out)
		}
		if res["isError"] == true {
			t.Errorf("%s: unexpected isError: %v", name, res)
		}
	}
}
