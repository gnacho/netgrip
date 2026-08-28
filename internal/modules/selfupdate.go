package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	githubRepo   = "gnacho/owpanel"
	githubAPIURL = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	assetName    = "owpanel-linux-arm64"
	tmpPath      = "/tmp/owpanel.update"
)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Body    string    `json:"body"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type SelfUpdateCheck struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Available  bool   `json:"available"`
	Notes      string `json:"notes"`
	AssetURL   string `json:"asset_url,omitempty"`
	AssetSize  int64  `json:"asset_size,omitempty"`
}

func CheckSelfUpdate(currentVersion string) *SelfUpdateCheck {
	result := &SelfUpdateCheck{Current: currentVersion}
	if currentVersion == "" || currentVersion == "dev" {
		return result
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return result
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "owpanel/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return result
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return result
	}

	result.Latest = release.TagName
	result.Notes = release.Body
	result.Available = release.TagName != currentVersion && release.TagName != ""

	for _, a := range release.Assets {
		if a.Name == assetName {
			result.AssetURL = a.BrowserDownloadURL
			result.AssetSize = a.Size
			break
		}
	}
	return result
}

func ApplySelfUpdate(assetURL string) error {
	if assetURL == "" {
		return fmt.Errorf("no asset URL")
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(assetURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	f.Close()
	os.Chmod(tmpPath, 0755)

	currentBin, err := os.Executable()
	if err != nil {
		currentBin = "/usr/sbin/owpanel"
	}
	currentBin = strings.TrimSuffix(currentBin, " (deleted)")

	backupPath := currentBin + ".bak"
	if err := os.Rename(currentBin, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup: %w", err)
	}

	if err := os.Rename(tmpPath, currentBin); err != nil {
		os.Rename(backupPath, currentBin)
		return fmt.Errorf("install: %w", err)
	}

	exec.Command("/etc/init.d/owpanel", "restart").Start()
	return nil
}
