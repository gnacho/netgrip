package modules

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func serveRelease(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const releaseNoAssets = `{"tag_name":"v9.9.9","body":"release notes","assets":[]}`

const releaseWithAsset = `{"tag_name":"v9.9.9","body":"release notes","assets":[` +
	`{"name":"netgrip-linux-arm64","browser_download_url":"http://example.com/netgrip-linux-arm64","size":1234},` +
	`{"name":"netgrip-linux-amd64","browser_download_url":"http://example.com/netgrip-linux-amd64","size":1234}]}`

func stubUpdater(t *testing.T) (*atomic.Bool, *[]string) {
	t.Helper()
	called := &atomic.Bool{}
	var urls []string
	orig := runSelfUpdateF
	runSelfUpdateF = func(assetURL string, _ int64, _ string) {
		called.Store(true)
		urls = append(urls, assetURL)
	}
	t.Cleanup(func() { runSelfUpdateF = orig })
	return called, &urls
}

func resetUpdateState(t *testing.T) {
	t.Helper()
	updateMu.Lock()
	updateCheck = nil
	updateStatus = SelfUpdateStatus{Phase: "idle"}
	updateMu.Unlock()
}

func withAPI(t *testing.T, body string) *httptest.Server {
	t.Helper()
	resetUpdateState(t)
	srv := serveRelease(t, body)
	orig := githubAPIURL
	githubAPIURL = srv.URL
	t.Cleanup(func() { githubAPIURL = orig })
	return srv
}

func TestCheckSelfUpdateWithoutAssetIsNotAvailable(t *testing.T) {
	withAPI(t, releaseNoAssets)

	ck := CheckSelfUpdate("0.62.0")
	if ck.Latest != "v9.9.9" {
		t.Fatalf("Latest = %q, want v9.9.9", ck.Latest)
	}
	if ck.Available {
		t.Fatal("Available = true, want false when asset is missing")
	}
	if !ck.AssetsPending {
		t.Fatal("AssetsPending = false, want true when tag is newer but asset is missing")
	}
	if ck.AssetURL != "" {
		t.Fatalf("AssetURL = %q, want empty", ck.AssetURL)
	}
}

func TestCheckSelfUpdateWithAssetIsAvailable(t *testing.T) {
	withAPI(t, releaseWithAsset)

	ck := CheckSelfUpdate("0.62.0")
	if !ck.Available {
		t.Fatal("Available = false, want true when asset exists")
	}
	if ck.AssetsPending {
		t.Fatal("AssetsPending = true, want false when asset exists")
	}
	if ck.AssetURL != "http://example.com/netgrip-linux-arm64" {
		t.Fatalf("AssetURL = %q", ck.AssetURL)
	}
}

func TestCheckSelfUpdateSameVersionNoPending(t *testing.T) {
	withAPI(t, releaseNoAssets)

	ck := CheckSelfUpdate("9.9.9")
	if ck.Available || ck.AssetsPending {
		t.Fatal("same version must not be available nor pending")
	}
}

func TestStartSelfUpdateRechecksEmptyCache(t *testing.T) {
	withAPI(t, releaseWithAsset)
	called, urls := stubUpdater(t)

	if err := StartSelfUpdate("0.62.0"); err != nil {
		t.Fatalf("StartSelfUpdate: %v", err)
	}
	waitFor(t, called.Load)
	if len(*urls) != 1 || (*urls)[0] != "http://example.com/netgrip-linux-arm64" {
		t.Fatalf("runner urls = %v", *urls)
	}
}

func TestStartSelfUpdateRechecksStaleCache(t *testing.T) {
	withAPI(t, releaseWithAsset)
	called, _ := stubUpdater(t)

	updateMu.Lock()
	updateCheck = &SelfUpdateCheck{Current: "0.62.0", Latest: "v9.9.9", AssetsPending: true}
	updateMu.Unlock()

	if err := StartSelfUpdate("0.62.0"); err != nil {
		t.Fatalf("StartSelfUpdate with stale cache: %v", err)
	}
	waitFor(t, called.Load)
}

func TestStartSelfUpdateAssetsPendingError(t *testing.T) {
	withAPI(t, releaseNoAssets)
	called, _ := stubUpdater(t)

	err := StartSelfUpdate("0.62.0")
	if err == nil {
		t.Fatal("expected error when assets are not published")
	}
	if !strings.Contains(err.Error(), "assets not published") {
		t.Fatalf("err = %v, want assets-not-published message", err)
	}
	if called.Load() {
		t.Fatal("runner must not launch while assets are missing")
	}
}

func TestStartSelfUpdateUsesValidCacheWithoutRecheck(t *testing.T) {
	resetUpdateState(t)
	// A failing API must not be consulted when the cache is valid.
	srv := serveRelease(t, `{"tag_name":"v0.0.1"}`)
	orig := githubAPIURL
	githubAPIURL = srv.URL
	t.Cleanup(func() { githubAPIURL = orig })

	called, urls := stubUpdater(t)

	updateMu.Lock()
	updateCheck = &SelfUpdateCheck{
		Current:   "0.62.0",
		Latest:    "v9.9.9",
		Available: true,
		AssetURL:  "http://example.com/cached",
		AssetSize: 42,
	}
	updateMu.Unlock()

	if err := StartSelfUpdate("0.62.0"); err != nil {
		t.Fatalf("StartSelfUpdate with valid cache: %v", err)
	}
	waitFor(t, called.Load)
	if len(*urls) != 1 || (*urls)[0] != "http://example.com/cached" {
		t.Fatalf("runner urls = %v, want cached url", *urls)
	}
}
