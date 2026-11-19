package openwrt

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPagesManifestUsesOnlySelfUpdateAssets(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	fixture := `{"tagName":"v9.9.9","body":"release notes","assets":[
		{"name":"netgrip-linux-arm64","size":1234},
		{"name":"netgrip-linux-arm64.sha256","size":65},
		{"name":"netgrip-linux-arm64.sha256.sig","size":64},
		{"name":"netgrip-linux-arm64.raw","size":9999},
		{"name":"netgrip-9.9.9-r1-arm64.apk","size":4321}
	]}`
	gh := "#!/bin/sh\ncat <<'EOF'\n" + fixture + "\nEOF\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0755); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "latest.json")
	cmd := exec.Command("sh", "pages-manifest.sh", "v9.9.9", out)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pages-manifest.sh: %v\n%s", err, output)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("invalid manifest: %v\n%s", err, data)
	}
	if manifest.TagName != "v9.9.9" || manifest.Body != "release notes" {
		t.Fatalf("release metadata changed: %+v", manifest)
	}
	if len(manifest.Assets) != 3 {
		t.Fatalf("assets = %d, want only binary, digest and signature: %s", len(manifest.Assets), data)
	}
	for i, name := range []string{"netgrip-linux-arm64", "netgrip-linux-arm64.sha256", "netgrip-linux-arm64.sha256.sig"} {
		asset := manifest.Assets[i]
		if asset.Name != name {
			t.Fatalf("asset %d = %q, want %q", i, asset.Name, name)
		}
		wantURL := "https://gnacho.github.io/netgrip/" + name
		if asset.URL != wantURL {
			t.Fatalf("asset %q URL = %q, want %q", name, asset.URL, wantURL)
		}
	}
	if manifest.Assets[0].Size != 1234 {
		t.Fatalf("binary size = %d, want 1234", manifest.Assets[0].Size)
	}
}
