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
	githubRepo = "gnacho/netgrip"
	assetName  = "netgrip-linux-arm64"
	tmpPath    = "/tmp/netgrip.update"
)

// Overridable in tests.
var (
	githubAPIURL   = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	runSelfUpdateF = runSelfUpdate
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
	// AssetsPending is true when the tag is newer but the downloadable
	// asset has not been published yet (CI uploads assets a few minutes
	// after the release object appears).
	AssetsPending bool `json:"assets_pending,omitempty"`
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
	newer := isNewerVersion(release.TagName, currentVersion)

	for _, a := range release.Assets {
		if a.Name == assetName {
			result.AssetURL = a.BrowserDownloadURL
			result.AssetSize = a.Size
			break
		}
	}

	// Only advertise an update once the binary is actually downloadable:
	// the CI attaches release assets a few minutes after the release
	// object shows up, and an "available" update without an asset URL
	// cannot be applied.
	result.AssetsPending = newer && result.AssetURL == ""
	result.Available = newer && result.AssetURL != ""

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
	ck := updateCheck
	updateMu.Unlock()

	if ck == nil || !ck.Available || ck.AssetURL == "" {
		// The cached check may be empty (service restarted after the page
		// loaded) or may predate asset publication; re-check before
		// refusing so a retry from the same page works.
		ck = CheckSelfUpdate(currentVersion)
	}
	if ck == nil || !ck.Available || ck.AssetURL == "" {
		if ck != nil && ck.AssetsPending {
			return fmt.Errorf("release assets not published yet, retry shortly")
		}
		return fmt.Errorf("no update available")
	}

	go runSelfUpdateF(ck.AssetURL, ck.AssetSize, currentVersion)
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

	if err := copyFile(tmpPath, currentBin); err != nil {
		os.Rename(backupPath, currentBin)
		os.Remove(tmpPath)
		setUpdateStatus("error", 0, fmt.Sprintf("install: %v", err))
		return
	}
	os.Remove(tmpPath)
	os.Chmod(currentBin, 0755)

	setUpdateStatus("restarting", 100, "")
	exec.Command("/etc/init.d/netgrip", "restart").Start()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// isNewerVersion reports whether tag is strictly newer than current.
// Both accept an optional "v" prefix; comparison is numeric per semver
// field (major.minor.patch). Unparseable versions fall back to
// inequality so dev builds still see updates. The old inequality-only
// check nagged forever after updating (v0.23.0 vs "v0.23.0") and
// reported downgrades as updates (0.23.0 vs v0.22.1).
func isNewerVersion(tag, current string) bool {
	if tag == "" {
		return false
	}
	nt, okT := parseSemver(tag)
	nc, okC := parseSemver(current)
	if !okT || !okC {
		return normalizeVer(tag) != normalizeVer(current)
	}
	for i := 0; i < 3; i++ {
		if nt[i] != nc[i] {
			return nt[i] > nc[i]
		}
	}
	return false
}

func normalizeVer(v string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v), "v"), "V")
}

// parseSemver extracts major.minor.patch; ok=false when it does not parse.
func parseSemver(v string) (out [3]int, ok bool) {
	var a, b, c int
	if n, _ := fmt.Sscanf(normalizeVer(v), "%d.%d.%d", &a, &b, &c); n != 3 {
		return out, false
	}
	return [3]int{a, b, c}, true
}
