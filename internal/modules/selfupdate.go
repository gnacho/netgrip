package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	githubRepo   = "gnacho/netgrip"
	githubAPIURL = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	assetName    = "netgrip-linux-arm64"
	tmpPath      = "/tmp/netgrip.update"
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
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
	AssetURL  string `json:"asset_url,omitempty"`
	AssetSize int64  `json:"asset_size,omitempty"`
}

type SelfUpdateStatus struct {
	Phase    string `json:"phase"`
	Progress int    `json:"progress"`
	Message  string `json:"message,omitempty"`
}

var (
	updateMu     sync.Mutex
	updateStatus = SelfUpdateStatus{Phase: "idle"}
	updateCheck  *SelfUpdateCheck
)

func GetSelfUpdateStatus() SelfUpdateStatus {
	updateMu.Lock()
	defer updateMu.Unlock()
	return updateStatus
}

func setUpdateStatus(phase string, progress int, msg string) {
	updateMu.Lock()
	updateStatus = SelfUpdateStatus{Phase: phase, Progress: progress, Message: msg}
	updateMu.Unlock()
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
	req.Header.Set("User-Agent", "netgrip/"+currentVersion)

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

	updateMu.Lock()
	updateCheck = result
	updateMu.Unlock()

	return result
}

func StartSelfUpdate(currentVersion string) error {
	updateMu.Lock()
	if updateStatus.Phase == "downloading" || updateStatus.Phase == "installing" {
		updateMu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	if updateCheck == nil || !updateCheck.Available || updateCheck.AssetURL == "" {
		updateMu.Unlock()
		return fmt.Errorf("no update available")
	}
	assetURL := updateCheck.AssetURL
	assetSize := updateCheck.AssetSize
	updateMu.Unlock()

	go runSelfUpdate(assetURL, assetSize, currentVersion)
	return nil
}

func runSelfUpdate(assetURL string, assetSize int64, currentVersion string) {
	setUpdateStatus("downloading", 0, "")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(assetURL)
	if err != nil {
		setUpdateStatus("error", 0, fmt.Sprintf("download: %v", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		setUpdateStatus("error", 0, fmt.Sprintf("download: HTTP %d", resp.StatusCode))
		return
	}

	total := assetSize
	if total <= 0 {
		total = resp.ContentLength
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		setUpdateStatus("error", 0, fmt.Sprintf("create temp: %v", err))
		return
	}

	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				os.Remove(tmpPath)
				setUpdateStatus("error", 0, fmt.Sprintf("write: %v", writeErr))
				return
			}
			downloaded += int64(n)
			if total > 0 {
				pct := int(downloaded * 100 / total)
				if pct > 100 {
					pct = 100
				}
				setUpdateStatus("downloading", pct, "")
			}
		}
		if readErr != nil {
			f.Close()
			if readErr != io.EOF {
				os.Remove(tmpPath)
				setUpdateStatus("error", 0, fmt.Sprintf("read: %v", readErr))
				return
			}
			break
		}
	}
	f.Close()
	os.Chmod(tmpPath, 0755)

	setUpdateStatus("installing", 100, "")

	currentBin, err := os.Executable()
	if err != nil {
		currentBin = "/usr/sbin/netgrip"
	}
	currentBin = strings.TrimSuffix(currentBin, " (deleted)")

	backupPath := currentBin + ".bak"
	if err := os.Rename(currentBin, backupPath); err != nil {
		os.Remove(tmpPath)
		setUpdateStatus("error", 0, fmt.Sprintf("backup: %v", err))
		return
	}

	if err := os.Rename(tmpPath, currentBin); err != nil {
		os.Rename(backupPath, currentBin)
		setUpdateStatus("error", 0, fmt.Sprintf("install: %v", err))
		return
	}

	setUpdateStatus("restarting", 100, "")
	exec.Command("/etc/init.d/netgrip", "restart").Start()
}
