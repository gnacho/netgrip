package modules

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/executor"
	"github.com/gnacho/netgrip/internal/ubus"
)

const (
	cpSection    = "netgrip" // named section in /etc/config/nodogsplash
	cpWebRoot    = "/etc/nodogsplash/htdocs"
	cpImageDir   = "/etc/nodogsplash/htdocs/images"
	cpAuthScript = "/etc/netgrip/nds_auth.sh"
	cpCodePath   = "/etc/netgrip/nds_code"
	cpImageExt   = "/etc/netgrip/nds_image_ext"

	cpTitleMax   = 64
	cpMessageMax = 512
	cpCodeMin    = 4
	cpCodeMax    = 32
	cpSessionMax = 43200 // minutes (30 days); 0 means never
	cpImageMax   = 512 * 1024
)

var (
	cpCodeRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	cpImageRe = regexp.MustCompile(`^(png|jpe?g|gif|webp)$`)
)

// CaptivePortalConfig is the user-provided captive portal configuration.
// access_code and custom_html are write-only: the code is stored as a 0600
// file (never read back), and custom_html is written as the splash page with
// only its presence flagged in the probe.
type CaptivePortalConfig struct {
	Enabled        bool   `json:"enabled"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	AccessCode     string `json:"access_code"`
	SessionMinutes int    `json:"session_minutes"`
	CustomHTML     string `json:"custom_html"`
}

// CaptivePortalImage is the optional logo/background upload (base64 payload).
type CaptivePortalImage struct {
	DataBase64 string `json:"image_base64"`
	Ext        string `json:"image_ext"`
}

// CaptivePortalProbe is the read-only captive portal state.
type CaptivePortalProbe struct {
	Applicable     bool   `json:"applicable"`
	Installed      bool   `json:"installed"`
	Active         bool   `json:"active"`
	Running        bool   `json:"running"`
	Interface      string `json:"interface"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	HasAccessCode  bool   `json:"has_access_code"`
	SessionMinutes int    `json:"session_minutes"`
	CustomHTML     bool   `json:"custom_html"`
	HasImage       bool   `json:"has_image"`
}

func cpInstalled() bool {
	_, err := os.Stat("/etc/init.d/nodogsplash")
	return err == nil
}

// guestDevice resolves the L3 device of the guest network via ubus. It is
// never hardcoded to lan/wan: the guest interface is the only valid target.
func guestDevice() string {
	raw, err := ubus.Call("network.interface.guest", "status")
	if err != nil {
		return ""
	}
	var status struct {
		L3Device string `json:"l3_device"`
		Device   string `json:"device"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return ""
	}
	if status.L3Device != "" {
		return status.L3Device
	}
	return status.Device
}

// cpRunning reports whether the nodogsplash service is running.
func cpRunning() bool {
	return executor.ServiceRunning("nodogsplash")
}

// currentImageExt returns the stored image extension ("" when no image).
func currentImageExt() string {
	b, err := os.ReadFile(cpImageExt)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimSpace(string(b)))
	if !validImageExt(ext) {
		return ""
	}
	return ext
}

func validImageExt(ext string) bool {
	return cpImageRe.MatchString(strings.ToLower(strings.TrimSpace(ext)))
}

// ProbeCaptivePortal reads the captive portal state.
func ProbeCaptivePortal() *CaptivePortalProbe {
	p := &CaptivePortalProbe{Installed: cpInstalled()}
	g := ProbeGuest()
	if !g.Gateway {
		return p
	}
	dev := guestDevice()
	if dev == "" {
		return p // guest network not configured or not up
	}
	p.Applicable = true
	p.Interface = dev
	if !p.Installed {
		return p
	}
	base := "nodogsplash." + cpSection
	if !uciSectionExists(base) {
		return p
	}
	p.Active = uciGet(base+".enabled") == "1"
	p.Title = uciGet(base + ".gatewayname")
	p.Message = uciGet(base + ".message")
	p.HasAccessCode = uciGet(base+".binauth") != ""
	if v := uciGet(base + ".sessiontimeout"); v != "" {
		p.SessionMinutes, _ = strconv.Atoi(v)
	}
	p.CustomHTML = uciGet(base+".custom_html") == "1"
	p.HasImage = currentImageExt() != ""
	p.Running = cpRunning()
	return p
}

// SetCaptivePortal applies the captive portal configuration with snapshot,
// healthcheck and rollback, mirroring the other service toggles.
func SetCaptivePortal(cfg CaptivePortalConfig) (*CaptivePortalProbe, bool, error) {
	probe := ProbeCaptivePortal()
	if cfg.Enabled {
		if !probe.Applicable {
			return probe, false, fmt.Errorf("guest network is not configured: enable the guest WiFi first")
		}
		if err := validateCaptivePortal(cfg); err != nil {
			return probe, false, err
		}
		// Install before snapshotting so /etc/config/nodogsplash exists.
		if !cpInstalled() {
			if err := executor.Run(executor.Op{Kind: "pkg_add", Args: []string{"nodogsplash"}}); err != nil {
				return probe, false, fmt.Errorf("install nodogsplash: %w", err)
			}
		}
	}

	snap, snapOK := snapshotConfig("nodogsplash")
	rollback := func() {
		if snapOK {
			_ = executor.Restore("nodogsplash", snap)
		} else {
			// No prior config: neutralize anything the package or NetGrip added.
			for _, s := range nodogsplashSections() {
				_ = executor.Run(executor.Op{Kind: "uci_set", Args: []string{s + ".enabled", "0"}})
			}
			_ = executor.Run(executor.Op{Kind: "uci_commit", Args: []string{"nodogsplash"}})
		}
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"nodogsplash", "stop"}})
	}

	// Write the splash page and (when needed) the binauth script and code
	// file before the service starts.
	if cfg.Enabled {
		if err := writeCaptiveFiles(cfg); err != nil {
			return probe, false, err
		}
	}

	ops, err := captiveOps(cfg, guestDevice())
	if err != nil {
		return probe, false, err
	}
	if err := executor.Apply(ops, nil); err != nil {
		rollback()
		return ProbeCaptivePortal(), true, err
	}
	if !cfg.Enabled {
		_ = os.Remove(cpCodePath)
	}

	ok := func() bool {
		for range 15 {
			p := ProbeCaptivePortal()
			if cfg.Enabled {
				if p.Active && p.Running {
					return true
				}
			} else if !p.Running {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	if !ok() {
		rollback()
		return ProbeCaptivePortal(), true, fmt.Errorf("healthcheck failed after apply (enabled=%v), rolled back", cfg.Enabled)
	}
	return ProbeCaptivePortal(), false, nil
}

// SetCaptivePortalImage stores the uploaded logo/background image and, when
// the portal is active with a generated page, regenerates the splash page.
func SetCaptivePortalImage(img CaptivePortalImage) (*CaptivePortalProbe, error) {
	ext := strings.ToLower(strings.TrimSpace(img.Ext))
	if !validImageExt(ext) {
		return ProbeCaptivePortal(), fmt.Errorf("image must be png, jpg, gif or webp")
	}
	raw, err := base64.StdEncoding.DecodeString(img.DataBase64)
	if err != nil {
		return ProbeCaptivePortal(), fmt.Errorf("invalid base64 image data")
	}
	if len(raw) == 0 || len(raw) > cpImageMax {
		return ProbeCaptivePortal(), fmt.Errorf("image must be 1 byte to %d KB", cpImageMax/1024)
	}
	if err := os.MkdirAll(cpImageDir, 0755); err != nil {
		return ProbeCaptivePortal(), err
	}
	if old := currentImageExt(); old != "" && old != ext {
		_ = os.Remove(filepath.Join(cpImageDir, "logo."+old))
	}
	if err := os.WriteFile(filepath.Join(cpImageDir, "logo."+ext), raw, 0644); err != nil {
		return ProbeCaptivePortal(), err
	}
	if err := os.MkdirAll(filepath.Dir(cpImageExt), 0755); err != nil {
		return ProbeCaptivePortal(), err
	}
	if err := os.WriteFile(cpImageExt, []byte(ext+"\n"), 0600); err != nil {
		return ProbeCaptivePortal(), err
	}

	probe := ProbeCaptivePortal()
	if probe.Active && !probe.CustomHTML {
		html := splashHTML(probe.Title, probe.Message, ext, probe.HasAccessCode)
		if err := os.WriteFile(filepath.Join(cpWebRoot, "splash.html"), []byte(html), 0644); err != nil {
			return ProbeCaptivePortal(), err
		}
	}
	return ProbeCaptivePortal(), nil
}

// validateCaptivePortal validates the user config. It is a pure function.
func validateCaptivePortal(cfg CaptivePortalConfig) error {
	if len(cfg.Title) > cpTitleMax {
		return fmt.Errorf("title must be at most %d characters", cpTitleMax)
	}
	if len(cfg.Message) > cpMessageMax {
		return fmt.Errorf("message must be at most %d characters", cpMessageMax)
	}
	if code := cfg.AccessCode; code != "" {
		if len(code) < cpCodeMin || len(code) > cpCodeMax || !cpCodeRe.MatchString(code) {
			return fmt.Errorf("access code must be %d-%d characters using letters, digits, underscore or hyphen", cpCodeMin, cpCodeMax)
		}
	}
	if cfg.SessionMinutes < 0 || cfg.SessionMinutes > cpSessionMax {
		return fmt.Errorf("session duration must be between 0 (never) and %d minutes", cpSessionMax)
	}
	return nil
}

// captiveOps builds the UCI operations for the nodogsplash.netgrip section.
// On enable the section is recreated from scratch (idempotent, avoids list
// drift) and any other section (the package ships an anonymous one bound to
// br-lan) is disabled so nodogsplash never binds a non-guest interface.
func captiveOps(cfg CaptivePortalConfig, dev string) ([]executor.Op, error) {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	base := "nodogsplash." + cpSection

	if !cfg.Enabled {
		if uciSectionExists(base) {
			set(base+".enabled", "0")
			ops = append(ops,
				executor.Op{Kind: "uci_commit", Args: []string{"nodogsplash"}},
				executor.Op{Kind: "initd", Args: []string{"nodogsplash", "stop"}},
				executor.Op{Kind: "initd", Args: []string{"nodogsplash", "disable"}},
			)
		}
		return ops, nil
	}

	if dev == "" {
		return nil, fmt.Errorf("guest network is not configured: enable the guest WiFi first")
	}

	// Neutralize every section except our named one (the package default is
	// an anonymous section with enabled=1 and gatewayinterface=br-lan).
	for _, s := range nodogsplashSections() {
		if s != base {
			set(s+".enabled", "0")
		}
	}

	if uciSectionExists(base) {
		ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{base}})
	}
	set(base, "nodogsplash")
	set(base+".enabled", "1")
	set(base+".fwhook_enabled", "1")
	set(base+".gatewayinterface", dev)
	set(base+".gatewayname", strings.TrimSpace(cfg.Title))
	set(base+".message", cfg.Message)
	set(base+".sessiontimeout", strconv.Itoa(cfg.SessionMinutes))
	set(base+".checkinterval", "60")
	if cfg.AccessCode != "" {
		set(base+".binauth", cpAuthScript)
	}
	if cfg.CustomHTML != "" {
		set(base+".custom_html", "1")
	}

	addList := func(option, value string) {
		ops = append(ops, executor.Op{Kind: "uci_add_list", Args: []string{base + "." + option, value}})
	}
	// Preserve guest isolation: authenticated clients must not reach RFC1918
	// subnets (the LAN) even though nodogsplash inserts rules ahead of fw4.
	addList("authenticated_users", "block to 192.168.0.0/16")
	addList("authenticated_users", "block to 10.0.0.0/8")
	addList("authenticated_users", "block to 172.16.0.0/12")
	addList("authenticated_users", "allow all")
	// Only DNS/DHCP toward the router itself; no ssh/http for guests.
	addList("users_to_router", "allow udp port 53")
	addList("users_to_router", "allow tcp port 53")
	addList("users_to_router", "allow udp port 67")

	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"nodogsplash"}},
		executor.Op{Kind: "initd", Args: []string{"nodogsplash", "enable"}},
		executor.Op{Kind: "initd", Args: []string{"nodogsplash", "start"}},
	)
	return ops, nil
}

// nodogsplashSections returns the section keys currently present in
// /etc/config/nodogsplash (both anonymous and named).
func nodogsplashSections() []string {
	out, err := exec.Command("sh", "-c", "uci show nodogsplash 2>/dev/null | cut -d. -f1,2 | grep -v '=' | sort -u").Output()
	if err != nil {
		return []string{}
	}
	return strings.Fields(string(out))
}

// snapshotConfig exports a UCI config if it exists on disk. The bool reports
// whether a snapshot was actually taken.
func snapshotConfig(config string) (string, bool) {
	if _, err := os.Stat("/etc/config/" + config); err != nil {
		return "", false
	}
	s, err := executor.Snapshot(config)
	if err != nil {
		return "", false
	}
	return s, true
}

// escapeHTML escapes user text for the splash page. In addition to the usual
// HTML entities it escapes '$' so nodogsplash's template parser leaves the
// text untouched (it would otherwise strip a bare '$' or substitute a known
// variable like $tok/$redir).
func escapeHTML(s string) string {
	return strings.ReplaceAll(html.EscapeString(s), "$", "&#36;")
}

// splashHTML renders the mobile-first splash page from already-escaped user
// text. The $authaction/$tok/$redir tokens are left literal for nodogsplash.
func splashHTML(title, message, imageExt string, codeRequired bool) string {
	imgTag := ""
	if imageExt != "" {
		imgTag = `<img class="logo" src="/images/logo.` + imageExt + `" alt="">`
	}
	msgBlock := ""
	if strings.TrimSpace(message) != "" {
		msgBlock = `<p class="msg">` + escapeHTML(message) + `</p>`
	}
	codeForm := ""
	if codeRequired {
		codeForm = `<input type="password" name="password" placeholder="Access code" autocomplete="off" required>`
	}
	r := strings.NewReplacer(
		"@TITLE@", escapeHTML(title),
		"@MESSAGE@", msgBlock,
		"@IMGTAG@", imgTag,
		"@CODEFORM@", codeForm,
	)
	return r.Replace(splashShell)
}

const splashShell = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate">
<meta http-equiv="Pragma" content="no-cache">
<meta http-equiv="Expires" content="0">
<title>@TITLE@</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background: #f4f5f7; color: #1c2333; display: flex; align-items: center; justify-content: center;
  min-height: 100vh; padding: 16px; }
.card { background: #fff; border-radius: 16px; box-shadow: 0 1px 3px rgba(16,24,40,.08), 0 4px 16px rgba(16,24,40,.06);
  width: 100%; max-width: 360px; padding: 28px 24px; text-align: center; }
img.logo { display: block; max-width: 160px; max-height: 80px; margin: 0 auto 16px; }
h1 { font-size: 20px; font-weight: 600; margin-bottom: 8px; }
p.msg { font-size: 14px; line-height: 1.5; color: #556; margin-bottom: 20px; white-space: pre-wrap; overflow-wrap: break-word; }
input[type=password] { width: 100%; height: 44px; border: 1px solid #d0d5dd; border-radius: 10px;
  padding: 0 12px; font-size: 15px; margin-bottom: 12px; background: #fff; }
button { width: 100%; height: 44px; border: 0; border-radius: 10px; background: #0969da;
  color: #fff; font-size: 15px; font-weight: 600; cursor: pointer; }
button:active { background: #0757b2; }
</style>
</head>
<body>
<div class="card">
@IMGTAG@
<h1>@TITLE@</h1>
@MESSAGE@
<form method="GET" action="$authaction">
<input type="hidden" name="tok" value="$tok">
<input type="hidden" name="redir" value="$redir">
@CODEFORM@
<button type="submit">Continue</button>
</form>
</div>
</body>
</html>
`

// ndsAuthScript is the BinAuth script for the shared access code. nodogsplash
// calls it as `nds_auth.sh auth_client <mac> '<username>' '<password>'`; on
// success it echoes the session length in seconds and exits 0, on failure it
// exits non-zero (nodogsplash v5 rejects when the script fails or outputs
// nothing, so a rejected login must NOT echo "0").
const ndsAuthScript = `#!/bin/sh
method="$1"
[ "$method" = "auth_client" ] || exit 0
pass="$4"
[ -n "$pass" ] || pass="$3"
code="$(cat /etc/netgrip/nds_code 2>/dev/null)"
[ -n "$code" ] || exit 1
[ "$pass" = "$code" ] || exit 1
mins="$(uci -q get nodogsplash.netgrip.sessiontimeout 2>/dev/null)"
[ -n "$mins" ] || mins=120
echo $((mins * 60))
exit 0
`

// writeCaptiveFiles writes the splash page and, when an access code is set,
// the binauth script and the code file.
func writeCaptiveFiles(cfg CaptivePortalConfig) error {
	if err := os.MkdirAll(cpWebRoot, 0755); err != nil {
		return err
	}
	page := splashHTML(cfg.Title, cfg.Message, currentImageExt(), cfg.AccessCode != "")
	if cfg.CustomHTML != "" {
		page = cfg.CustomHTML
	}
	if err := os.WriteFile(filepath.Join(cpWebRoot, "splash.html"), []byte(page), 0644); err != nil {
		return err
	}

	if cfg.AccessCode != "" {
		if err := os.MkdirAll(filepath.Dir(cpAuthScript), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(cpAuthScript, []byte(ndsAuthScript), 0755); err != nil {
			return err
		}
		if err := os.Chmod(cpAuthScript, 0755); err != nil {
			return err
		}
		return os.WriteFile(cpCodePath, []byte(cfg.AccessCode+"\n"), 0600)
	}
	return nil
}
