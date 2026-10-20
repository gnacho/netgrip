package modules

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	githubRepo = "gnacho/netgrip"
	tmpPath    = "/tmp/netgrip.update"
)

// prevBinaryPath is the post-install rescue copy of the previous binary.
// It lives on tmpfs, never on the overlay: tiny-overlay devices cannot
// afford a second persistent copy of the binary, and no rollback path ever
// consumed the old on-overlay .bak, which just accumulated dead weight and
// eventually made every update fail with ENOSPC (#528). tmpfs survives a
// service restart, which is the risky moment right after an update.
var prevBinaryPath = "/tmp/netgrip.prev"

// binaryAssetName is the release asset for this build's architecture, so an
// unsupported arch (e.g. mipsel before #236) never matches an existing asset
// and the update is not offered/applied with a wrong-architecture binary (#237).
// releaseArch is injected by deploy/build-binaries.sh because runtime.GOARCH
// reports "arm" for the distinct armv5, armv6 and armv7 release binaries.
// It stays empty in local builds and tests, which use the runtime fallback.
var releaseArch string

func defaultReleaseArch() string {
	if runtime.GOARCH == "arm" {
		return "armv7"
	}
	return runtime.GOARCH
}

func binaryAssetName() string {
	arch := releaseArch
	if arch == "" {
		arch = defaultReleaseArch()
	}
	return "netgrip-linux-" + arch
}

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

// selfUpdateOutcomeHook, when armed, receives the terminal outcome of the
// next self-update so a remote trigger (NetPulse's SSE "upgrade" command,
// #549) can show real progress instead of a silent hang. One-shot: the
// hook is consumed by the first terminal state (success before the
// restart, or any failure) and self-updates started from the panel or the
// scheduler never arm it, so they never report anywhere.
var (
	outcomeHookMu         sync.Mutex
	selfUpdateOutcomeHook func(ok bool, errMsg string)
)

func setSelfUpdateOutcomeHook(f func(ok bool, errMsg string)) {
	outcomeHookMu.Lock()
	defer outcomeHookMu.Unlock()
	selfUpdateOutcomeHook = f
}

// reportSelfUpdateOutcome entrega el resultado terminal al hook armado (si
// hay) y lo consume. Se llama desde el único punto de cada final: el path
// de éxito de runSelfUpdate (antes del restart) y failUpdate.
func reportSelfUpdateOutcome(ok bool, errMsg string) {
	outcomeHookMu.Lock()
	f := selfUpdateOutcomeHook
	selfUpdateOutcomeHook = nil
	outcomeHookMu.Unlock()
	if f != nil {
		f(ok, errMsg)
	}
}

func GetSelfUpdateStatus() SelfUpdateStatus {
	updateMu.Lock()
	defer updateMu.Unlock()
	return updateStatus
}

// failUpdate logs and records a terminal self-update failure. Errors used
// to disappear into the status struct only, which made overlayfs failures
// look like silent hangs (#299).
func failUpdate(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("netgrip: self-update failed: %s", msg)
	setUpdateStatus("error", 0, msg)
	reportSelfUpdateOutcome(false, msg)
	notifyAll("NetGrip",
		"🔴 <b>NetGrip</b>\nSelf-update failed: "+htmlEsc(msg),
		"🔴 NetGrip\nSelf-update failed: "+msg,
		true)
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
		if a.Name == binaryAssetName() {
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
		failUpdate("download: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		failUpdate("download: HTTP %d", resp.StatusCode)
		return
	}

	total := assetSize
	if total <= 0 {
		total = resp.ContentLength
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		failUpdate("create temp: %v", err)
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
				failUpdate("write: %v", writeErr)
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
				failUpdate("read: %v", readErr)
				return
			}
			break
		}
	}
	f.Close()
	os.Chmod(tmpPath, 0755)

	// Fail safe: never replace a working binary with one built for another
	// architecture. Check the ELF e_machine against this build's GOARCH.
	if ok, err := elfArchMatches(runtime.GOARCH, tmpPath); err != nil || !ok {
		os.Remove(tmpPath)
		failUpdate("arch mismatch: ok=%v err=%v", ok, err)
		return
	}

	setUpdateStatus("installing", 100, "")

	currentBin, err := os.Executable()
	if err != nil {
		currentBin = "/usr/sbin/netgrip"
	}
	currentBin = strings.TrimSuffix(currentBin, " (deleted)")

	// Unlink the running binary, then copy the new one into the freed
	// space. Peak overlay usage stays at one copy, which is what makes
	// self-update fit on tiny ubifs/jffs2 overlays (#528). The unlinked
	// inode stays alive for the running process, so the service keeps
	// working until the restart below; writing over a running binary
	// fails with ETXTBSY and renaming it gives ESTALE on overlayfs
	// (#299), which is why the copy is a remove + create.
	if err := installSelfUpdate(tmpPath, currentBin); err != nil {
		os.Remove(tmpPath)
		failUpdate("install: %v", err)
		return
	}
	os.Remove(tmpPath)

	notifyAll("NetGrip",
		"✅ <b>NetGrip</b>\nSelf-update applied - restarting",
		"✅ NetGrip\nSelf-update applied - restarting",
		true)

	// Refresh the init script (best effort) before restarting, so the new
	// binary is also launched by the current script (#433).
	syncInitScript(currentVersion)

	setUpdateStatus("restarting", 100, "")
	reportSelfUpdateOutcome(true, "")
	exec.Command("/etc/init.d/netgrip", "restart").Start()
}

// elfMachineByGOARCH maps a Go arch to the ELF e_machine value it expects.
var elfMachineByGOARCH = map[string]uint16{
	"amd64":   62,  // EM_X86_64
	"arm64":   183, // EM_AARCH64
	"arm":     40,  // EM_ARM
	"386":     3,   // EM_386
	"mipsle":  8,   // EM_MIPS (soft-float)
	"mips":    8,   // EM_MIPS
	"riscv64": 243, // EM_RISCV
	"ppc64le": 21,  // EM_PPC64
	"loong64": 258, // EM_LOONGARCH
}

// elfArchMatches verifies the ELF e_machine of a downloaded binary matches the
// given Go architecture, refusing cross-architecture installs (#237).
func elfArchMatches(goarch, path string) (bool, error) {
	want, ok := elfMachineByGOARCH[goarch]
	if !ok {
		// Unknown arch: fail closed (refuse) to avoid bricking.
		return false, fmt.Errorf("unknown architecture %q", goarch)
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	hdr := make([]byte, 20)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, err
	}
	if hdr[0] != 0x7f || hdr[1] != 'E' || hdr[2] != 'L' || hdr[3] != 'F' {
		return false, fmt.Errorf("not an ELF file")
	}
	// e_machine is at offset 18 (2 bytes), in the endianness declared by
	// EI_DATA (offset 5): 1 = LSB (little-endian), 2 = MSB (big-endian).
	// Big-endian targets (GOARCH=mips) ship big-endian ELF headers, so a
	// fixed little-endian read rejects the correct binary (#513).
	var order binary.ByteOrder = binary.LittleEndian
	if hdr[5] == 2 {
		order = binary.BigEndian
	}
	return order.Uint16(hdr[18:20]) == want, nil
}

// installSelfUpdate replaces currentBin with the downloaded binary in
// tmpPath, using at most one copy's worth of space on the target
// filesystem: the previous binary is unlinked first, which frees its
// space for the new copy. Tiny overlays (tens of MB) cannot hold the old
// binary, a staged copy and a backup at the same time, and that triple
// accounting is what used to fail with ENOSPC (#528).
//
// The rescue copy of the old binary goes to tmpfs, best effort: it exists
// so a bad update can be reverted by hand after a service restart, not as
// an automatic rollback (there is none).
func installSelfUpdate(tmpPath, currentBin string) error {
	if err := copyFile(currentBin, prevBinaryPath); err != nil {
		log.Printf("netgrip: self-update: rescue copy to %s: %v (continuing)", prevBinaryPath, err)
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("stat downloaded binary: %w", err)
	}

	dir := filepath.Dir(currentBin)

	// The current binary is unlinked below before the new copy lands, so
	// its space counts towards the budget; without it the pre-check
	// demands the whole new binary from free space alone and every update
	// on a tiny overlay fails even though the swap itself would fit (#547).
	// Apparent size is used on purpose: on compressing filesystems (ubifs)
	// it overestimates what the unlink actually frees, which is the safe
	// direction for a pre-check (a false "not enough" only fails the
	// update; a false "enough" defers ENOSPC to the copy, which is already
	// handled gracefully).
	var freed uint64
	if cur, err := os.Stat(currentBin); err == nil && cur.Size() > 0 {
		freed = uint64(cur.Size())
	}

	if !spaceFor(dir, uint64(info.Size()), freed) {
		// Best effort: drop the package cache to make room on tiny
		// overlays. No-op where the directory does not exist.
		os.RemoveAll("/etc/apk/cache")
		if !spaceFor(dir, uint64(info.Size()), freed) {
			return fmt.Errorf("not enough space on %s for the new binary (need ~%d bytes)", dir, info.Size())
		}
	}

	// The running process keeps the unlinked inode alive, so the service
	// is unaffected until the post-update restart.
	if err := os.Remove(currentBin); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove current binary: %w", err)
	}
	if err := copyFile(tmpPath, currentBin); err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}
	return os.Chmod(currentBin, 0755)
}

// spaceFor reports whether dir has at least needed bytes available plus a
// small headroom, counting freed bytes that the install is about to
// release (the current binary being replaced). Unknown filesystems (statfs
// failure) are treated as having room so the update is never blocked by an
// exotic mount.
func spaceFor(dir string, needed, freed uint64) bool {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil || st.Bsize <= 0 {
		return true
	}
	avail := uint64(st.Bavail)*uint64(st.Bsize) + freed
	return avail >= needed+(1<<20)
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
