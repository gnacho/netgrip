package modules

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/certs"
	"github.com/gnacho/netgrip/internal/executor"
)

// AccessProbe is the read-only state of the three admin access surfaces:
// the panel itself, the router's web UI (uhttpd) and SSH (dropbear).
type AccessProbe struct {
	Panel PanelAccess `json:"panel"`
	Web   WebAccess   `json:"luci"` // wire name kept for older clients
	SSH   SSHAccess   `json:"ssh"`
}

// PanelAccess covers the netgrip web server.
type PanelAccess struct {
	HTTPPort     int    `json:"http_port"`
	HTTPSEnabled bool   `json:"https_enabled"`
	ForceHTTPS   bool   `json:"force_https"`
	SessionTtl   string `json:"session_ttl"` // human string, e.g. "12h0m0s"
}

// WebAccess covers uhttpd, which serves the router's web UI.
type WebAccess struct {
	HTTPPort   int  `json:"http_port"`
	HTTPSPort  int  `json:"https_port"`
	ForceHTTPS bool `json:"force_https"`
	Enabled    bool `json:"enabled"`
	// Installed: la interfaz web del router está instalada. Sin ella los
	// puertos de uhttpd no llevan a ningún panel y la tarjeta no debe
	// ofrecerlos (#638).
	Installed bool `json:"installed"`
}

// SSHAccess covers dropbear.
type SSHAccess struct {
	Enabled bool   `json:"enabled"`
	Port    string `json:"port"`
}

// ProbeAccess reads the current admin access state.
func ProbeAccess() *AccessProbe {
	return &AccessProbe{
		Panel: probePanelAccess(),
		Web:   probeWebAccess(),
		SSH:   probeSSHAccess(),
	}
}

// panelSessionTTLMinutePath is the UCI option (in minutes) that governs how
// long a panel session token lives. Absent means the default of 12h.
const panelSessionTTLMinutePath = "netgrip.main.session_timeout"

func probePanelAccess() PanelAccess {
	p := PanelAccess{
		HTTPPort:   8080,
		SessionTtl: PanelSessionTTLString(),
	}
	if p.HTTPSEnabled = uciGet("netgrip.main.https") == "1"; p.HTTPSEnabled {
		p.ForceHTTPS = uciGet("netgrip.main.force_https") == "1"
	}
	if v, err := strconv.Atoi(uciGet("netgrip.main.http_port")); err == nil && v > 0 {
		p.HTTPPort = v
	}
	return p
}

// PanelSessionTTLMinutes returns the configured panel session timeout in
// minutes, or 720 (12h) when unset or invalid.
func PanelSessionTTLMinutes() int {
	if v, err := strconv.Atoi(uciGet(panelSessionTTLMinutePath)); err == nil && v > 0 {
		return v
	}
	return 12 * 60
}

// PanelSessionTTLString returns a human-readable session TTL, e.g. "12h0m0s".
func PanelSessionTTLString() string {
	return (time.Duration(PanelSessionTTLMinutes()) * time.Minute).String()
}

// SetPanelSessionTTL persists the panel session timeout (minutes).
// It only affects tokens issued afterwards; existing tokens keep their
// original expiry.
func SetPanelSessionTTL(minutes int) error {
	if minutes <= 0 {
		return fmt.Errorf("session timeout must be > 0 minutes")
	}
	return applyNetgripConfig([]executor.Op{
		{Kind: "uci_set", Args: []string{panelSessionTTLMinutePath, strconv.Itoa(minutes)}},
	})
}

func probeWebAccess() WebAccess {
	l := WebAccess{
		Enabled:   executor.ServiceEnabled("uhttpd"),
		Installed: webUIInstalled(),
	}
	l.HTTPPort = parseListenPort(uciGet("uhttpd.main.listen_http"))
	if l.HTTPPort == 0 {
		l.HTTPPort = 80
	}
	l.HTTPSPort = parseListenPort(uciGet("uhttpd.main.listen_https"))
	if l.HTTPSPort == 0 {
		l.HTTPSPort = 443
	}
	l.ForceHTTPS = uciGet("uhttpd.main.redirect_https") == "1"
	return l
}

// webUIInstalled reports whether the router's web UI is present: the
// package is the reliable signal (a build can ship uhttpd without a UI).
func webUIInstalled() bool {
	if pkgInstalled("luci-base") {
		return true
	}
	if _, err := os.Stat("/www/luci-static"); err == nil { // stock web UI assets
		return true
	}
	return false
}

func probeSSHAccess() SSHAccess {
	return SSHAccess{
		Enabled: executor.ServiceEnabled("dropbear"),
		Port:    uciGet("dropbear.main.Port"),
	}
}

// parseListenPort extracts the port from a uhttpd listen_* value such as
// "0.0.0.0:80 [::]:80". Returns 0 when not parseable.
func parseListenPort(v string) int {
	for _, token := range strings.Fields(v) {
		idx := strings.LastIndex(token, ":")
		if idx < 0 {
			continue
		}
		if port, err := strconv.Atoi(token[idx+1:]); err == nil {
			return port
		}
	}
	return 0
}

// SetWebAccess applies uhttpd HTTP/HTTPS ports and redirect_https.
func SetWebAccess(cfg WebAccess) (*AccessProbe, bool, error) {
	if !executor.ServiceEnabled("uhttpd") {
		if err := executor.Run(executor.Op{Kind: "initd", Args: []string{"uhttpd", "enable"}}); err != nil {
			return ProbeAccess(), false, fmt.Errorf("uhttpd service missing: %w", err)
		}
	}
	snap, err := executor.Snapshot("uhttpd")
	if err != nil {
		return ProbeAccess(), false, fmt.Errorf("snapshot uhttpd: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("uhttpd", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"uhttpd", "restart"}})
	}

	if err := executor.Apply(webOps(cfg), nil); err != nil {
		rollback()
		return ProbeAccess(), true, err
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"uhttpd", "restart"}})

	if !webHealth(cfg) {
		rollback()
		return ProbeAccess(), true, fmt.Errorf("uhttpd healthcheck failed, rolled back")
	}
	return ProbeAccess(), false, nil
}

func webOps(cfg WebAccess) []executor.Op {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	if cfg.HTTPPort > 0 && cfg.HTTPPort <= 65535 {
		set("uhttpd.main.listen_http", fmt.Sprintf("0.0.0.0:%d [::]:%d", cfg.HTTPPort, cfg.HTTPPort))
	} else {
		set("uhttpd.main.listen_http", "")
	}
	if cfg.HTTPSPort > 0 && cfg.HTTPSPort <= 65535 {
		set("uhttpd.main.listen_https", fmt.Sprintf("0.0.0.0:%d [::]:%d", cfg.HTTPSPort, cfg.HTTPSPort))
	} else {
		set("uhttpd.main.listen_https", "")
	}
	redirect := "0"
	if cfg.ForceHTTPS {
		redirect = "1"
	}
	set("uhttpd.main.redirect_https", redirect)
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"uhttpd"}})
	return ops
}

func webHealth(cfg WebAccess) bool {
	// The question is whether uhttpd came back after the restart, not
	// whether its certificate is trustworthy. Routers ship a self-signed
	// one - CN=OpenWrt, no IP SAN - so a verifying client fails the
	// handshake against loopback every time, and turning "force HTTPS" on
	// could never succeed: the check rolled back a change that had worked.
	// Skipping verification is safe here in a way it would not be anywhere
	// else: the request never leaves the box.
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	scheme := "http"
	port := cfg.HTTPPort
	if cfg.ForceHTTPS {
		scheme = "https"
		port = cfg.HTTPSPort
		if port <= 0 {
			port = 443
		}
	} else if port <= 0 {
		port = 80
	}
	url := fmt.Sprintf("%s://127.0.0.1:%d/", scheme, port)
	for i := 0; i < 10; i++ {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// SetSSHAccess applies dropbear enable + port.
func SetSSHAccess(cfg SSHAccess) (*AccessProbe, bool, error) {
	snap, err := executor.Snapshot("dropbear")
	if err != nil {
		return ProbeAccess(), false, fmt.Errorf("snapshot dropbear: %w", err)
	}
	rollback := func() {
		_ = executor.Restore("dropbear", snap)
		_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dropbear", "restart"}})
	}

	if err := executor.Apply(sshOps(cfg), nil); err != nil {
		rollback()
		return ProbeAccess(), true, err
	}
	_ = executor.Run(executor.Op{Kind: "initd", Args: []string{"dropbear", "restart"}})

	probe := ProbeAccess()
	if !sshHealthy(cfg) {
		rollback()
		return ProbeAccess(), true, fmt.Errorf("dropbear healthcheck failed, rolled back")
	}
	return probe, false, nil
}

func sshOps(cfg SSHAccess) []executor.Op {
	var ops []executor.Op
	set := func(key, value string) {
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{key, value}})
	}
	if cfg.Enabled {
		set("dropbear.main.enable", "1")
	} else {
		set("dropbear.main.enable", "0")
	}
	if cfg.Port != "" {
		set("dropbear.main.Port", strconv.Itoa(sshPortOrDefault(cfg.Port)))
	}
	if cfg.Enabled {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"dropbear", "enable"}})
	} else {
		ops = append(ops, executor.Op{Kind: "initd", Args: []string{"dropbear", "disable"}})
	}
	ops = append(ops,
		executor.Op{Kind: "uci_commit", Args: []string{"dropbear"}},
		executor.Op{Kind: "initd", Args: []string{"dropbear", "restart"}},
	)
	return ops
}

// sshHealthy decides whether the change that was just applied took effect.
//
// Both directions are a question about the port, and they are opposite
// questions: enabling worked when something is listening, disabling worked
// when nothing is. Asking "is dropbear running" first - before the two cases
// were separated - meant disabling SSH could never succeed, because a
// stopped service is exactly what was asked for, so the check failed and
// rolled the change straight back. That guard belongs only to the enabled
// case.
//
// The state is polled rather than sampled once: dropbear needs a moment
// after the restart either to bind the port or to let go of it.
func sshHealthy(cfg SSHAccess) bool {
	port := sshPortOrDefault(cfg.Port)
	for i := 0; i < 10; i++ {
		if listeningOnPort(port) == cfg.Enabled {
			return sshHealthyFor(cfg.Enabled, cfg.Enabled, executor.ServiceRunning("dropbear"))
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// sshHealthyFor is the decision itself, separated from how the observations
// are made so that it can be read and tested without a router.
func sshHealthyFor(wantEnabled, listening, running bool) bool {
	if !wantEnabled {
		// Nothing listening is the whole of success here. dropbear not
		// running is the point, not a failure.
		return !listening
	}
	return listening && running
}

// sshPortOrDefault is dropbear's port, falling back to 22 when the value is
// missing or not a port.
func sshPortOrDefault(v string) int {
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		return 22
	}
	return port
}

// listeningOnPort reports whether anything holds a listening socket on the
// port, on any address.
//
// Dialling 127.0.0.1 answers a different question. dropbear bound to one
// interface - `option Interface 'lan'`, which is how you keep SSH off the
// WAN - never listens on loopback, so the dial is refused and a perfectly
// healthy daemon looks dead. The healthcheck then rolls back a change that
// was fine, and says SSH is broken when it is not.
//
// Reading /proc is also cheaper than a dial with a timeout, on a path that
// runs while the user waits for a save to come back.
func listeningOnPort(port int) bool {
	for _, p := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if hasListenerOnPort(string(b), port) {
			return true
		}
	}
	return false
}

// hasListenerOnPort parses one /proc/net/tcp table: the local address column
// is HEX_ADDRESS:HEX_PORT and state 0A is TCP_LISTEN. Split out so the
// parsing is testable against captured router output.
func hasListenerOnPort(table string, port int) bool {
	lines := strings.Split(table, "\n")
	if len(lines) < 2 {
		return false
	}
	for _, line := range lines[1:] { // the first line is the header
		f := strings.Fields(line)
		if len(f) < 4 || f[3] != "0A" {
			continue
		}
		colon := strings.LastIndex(f[1], ":")
		if colon < 0 {
			continue
		}
		p, err := strconv.ParseInt(f[1][colon+1:], 16, 32)
		if err == nil && int(p) == port {
			return true
		}
	}
	return false
}

const sslDir = "/etc/netgrip/ssl"

// Variables rather than constants so tests can point them at a temporary
// directory; nothing changes them at runtime.
var (
	certPath = sslDir + "/cert.pem"
	keyPath  = sslDir + "/key.pem"

	// uhttpd's pair, the one the router's web UI serves. Only the fallback: the paths come
	// from uhttpd's own configuration, see RouterCertPaths.
	routerCertPath = "/etc/uhttpd.crt"
	routerKeyPath  = "/etc/uhttpd.key"
)

func fileReadable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// RouterCertPaths is uhttpd's pair, read from uhttpd's own configuration.
//
// The conventional /etc/uhttpd.crt is only the default value of a UCI option,
// and a firmware that moves it would otherwise hide "the router's
// certificate" from the selector on a router that plainly has one, and
// mislabel what is being served.
//
// A variable so tests can pin the pair: reading uci first means a test that
// only overrides the fallback paths below passes on a development machine,
// where there is no uci, and quietly tests nothing on a router.
var RouterCertPaths = func() (string, string) {
	cert, key := uciGet("uhttpd.main.cert"), uciGet("uhttpd.main.key")
	if cert == "" || key == "" {
		return routerCertPath, routerKeyPath
	}
	return cert, key
}

// PanelCertPaths is the pair the panel generates for itself. Exported so the
// listener names the same two files this package writes, rather than keeping
// its own copy of the paths that would drift the first time either moved.
func PanelCertPaths() (string, string) { return certPath, keyPath }

// certServable says whether a pair could actually be served, which is the
// same question the listener asks at startup. This is what decides what to
// REPORT, so it has to agree with what the process would really do, or the
// card names one pair while another is on the wire.
func certServable(certFile, keyFile string) bool {
	if !fileReadable(certFile) || !fileReadable(keyFile) {
		return false
	}
	_, err := certs.LoadPair(certFile, keyFile)
	return err == nil
}

// HasRouterCert says whether uhttpd's pair is there to be CHOSEN, which is a
// stricter question than whether it could be served: the selector must only
// offer what saving will accept.
//
// Offering on "it loads" while EnableHTTPS gated on "it is usable" left the
// option on screen - preselected, on a router with no pair of its own - and
// Save answering 500, with nothing the card could do about it. uhttpd's
// certificate is not ours to regenerate, so the honest move is not to offer
// it.
func HasRouterCert() bool {
	cert, key := RouterCertPaths()
	return certs.Usable(cert, key, time.Now()) == nil
}

// CertSource says which pair would be used: the configured one if there is
// one and, failing that, the one startup would arrive at by elimination.
//
// Without this the selector showed "the panel's own" whenever the
// configuration was empty, even on a router where no such pair existed and
// the router's was being served. A selector naming something other than what
// is there is worse than no selector: it invites saving in the belief that
// nothing changes.
func CertSource() string {
	routerCert, _ := RouterCertPaths()
	switch c, _ := HTTPSCertPaths(); c {
	case "":
		// Unconfigured: fall through to the boot order below.
	case routerCert:
		return CertSourceRouter
	case certPath:
		return CertSourcePanel
	default:
		// A pair somebody configured by hand. Saying "the panel's own"
		// here was not merely a wrong label: the card sent that back on
		// the next Save, which rewrote the paths and threw the
		// configuration away without anyone asking for it.
		return CertSourceCustom
	}
	// Unconfigured: the same order startup follows.
	if certServable(certPath, keyPath) {
		return CertSourcePanel
	}
	if certServable(routerCert, routerCertKey()) {
		return CertSourceRouter
	}
	// Neither exists yet: the panel's own is the one that would be made.
	return CertSourcePanel
}

func routerCertKey() string {
	_, key := RouterCertPaths()
	return key
}

// HasSelfSignedCert says whether the panel's own pair is there AND could be
// served. Two os.Stat calls were not enough: a pair that exists but cannot
// serve made the card report a certificate the listener would skip over.
func HasSelfSignedCert() bool {
	return certServable(certPath, keyPath)
}

func GenerateSelfSignedCert() error {
	if err := os.MkdirAll(sslDir, 0700); err != nil {
		return fmt.Errorf("mkdir ssl: %w", err)
	}
	hostname, _ := os.Hostname()
	certPEM, keyPEM, err := buildSelfSigned(hostname, localAddresses(), time.Now())
	if err != nil {
		return err
	}
	return writeSelfSigned(certPath, keyPath, certPEM, keyPEM)
}

// HTTPSCertPaths is the pair the panel is configured to serve. A variable so
// tests can pin it without a uci binary.
var HTTPSCertPaths = func() (string, string) {
	return uciGet("netgrip.main.https_cert"), uciGet("netgrip.main.https_key")
}

// DisableHTTPS puts the panel back on HTTP. It does not delete the
// certificate: turning TLS on again need not regenerate it, and a pair
// generated by hand or supplied by the user is not ours to throw away.
func DisableHTTPS() error {
	if !uciSectionExists("netgrip.main") {
		return nil // never turned on: nothing to turn off
	}
	return applyNetgripConfig([]executor.Op{
		{Kind: "uci_set", Args: []string{"netgrip.main.https", "0"}},
	})
}

// HTTPSEnabled says whether the panel is configured to serve TLS. This is
// what the configuration holds, not how the process was started: between
// changing it and restarting, the two differ.
func HTTPSEnabled() bool {
	return uciGet("netgrip.main.https") == "1"
}

// Certificate sources the panel can serve. Named rather than inferred: the
// fallback order used to decide it silently, so clearing the configured
// paths still served the panel's own pair because the files were on disk,
// which is not what "use the router's certificate" means to anybody.
const (
	CertSourcePanel  = "panel"  // the panel's own pair, with the router's IPs in the SAN
	CertSourceRouter = "router" // uhttpd's, the same one the router web UI serves
	CertSourceCustom = "custom" // a pair configured by hand, which is left exactly as it is
)

// EnableHTTPS turns the panel's TLS on with the chosen certificate, writing
// the paths explicitly so what is served is what was asked for.
//
// The chosen pair is vetted before anything is written. Both the failure to
// write and the failure to vet leave the configuration exactly as it was:
// this used to call DisableHTTPS on the way out, which is not a rollback but
// a second state change - on a panel already serving HTTPS it committed
// https=0 without restarting, so the process kept serving TLS while the
// configuration said plaintext, and the next reboot quietly obeyed the
// configuration.
func EnableHTTPS(source string) error {
	certFile, keyFile, err := resolveSource(source)
	if err != nil {
		return err
	}
	ops := []executor.Op{
		{Kind: "uci_set", Args: []string{"netgrip.main.https", "1"}},
	}
	// A hand-configured pair is left exactly where it is. Rewriting the
	// paths to the panel's own would discard a choice nobody revisited.
	if source != CertSourceCustom {
		ops = append(ops,
			executor.Op{Kind: "uci_set", Args: []string{"netgrip.main.https_cert", certFile}},
			executor.Op{Kind: "uci_set", Args: []string{"netgrip.main.https_key", keyFile}},
		)
	}
	return applyNetgripConfig(ops)
}

// resolveSource vets the chosen pair and returns it. Nothing is written until
// this succeeds, so a pair that cannot serve leaves the configuration alone.
func resolveSource(source string) (certFile, keyFile string, err error) {
	switch source {
	case CertSourceRouter:
		certFile, keyFile = RouterCertPaths()
		if err := certs.Usable(certFile, keyFile, time.Now()); err != nil {
			return "", "", fmt.Errorf("the router's certificate at %s cannot be served: %w", certFile, err)
		}
	case CertSourceCustom:
		certFile, keyFile = HTTPSCertPaths()
		if certFile == "" || keyFile == "" {
			return "", "", fmt.Errorf("no certificate is configured to keep")
		}
		if err := certs.Usable(certFile, keyFile, time.Now()); err != nil {
			return "", "", fmt.Errorf("the configured certificate at %s cannot be served: %w", certFile, err)
		}
	default:
		certFile, keyFile = certPath, keyPath
		if err := certs.Usable(certFile, keyFile, time.Now()); err != nil {
			// Regenerate rather than refuse. Checking only that the files
			// existed meant a pair that no browser would open - the
			// empty-SAN one px5g used to write - survived every attempt to
			// replace it, with no way back except deleting it over SSH.
			if err := regenerateSelfSigned(); err != nil {
				return "", "", err
			}
			if err := certs.Usable(certFile, keyFile, time.Now()); err != nil {
				return "", "", fmt.Errorf("the generated certificate is not usable: %w", err)
			}
		}
	}
	return certFile, keyFile, nil
}

// regenerateSelfSigned replaces the panel's pair, moving whatever was there
// aside first. The pair may have been put there by hand, and even a broken
// one is worth more than nothing to whoever has to work out what happened.
func regenerateSelfSigned() error {
	stamp := time.Now().Format("20060102-150405")
	var moved []string
	for _, p := range []string{certPath, keyPath} {
		if !fileReadable(p) {
			continue
		}
		aside := p + replacedSuffix + stamp
		if err := os.Rename(p, aside); err != nil {
			// Put back whatever was already moved. Half a pair kept aside
			// is worse than none: the leftover half looks like a usable
			// backup and is not.
			for _, done := range moved {
				_ = os.Rename(done+replacedSuffix+stamp, done)
			}
			return fmt.Errorf("move the old certificate aside: %w", err)
		}
		moved = append(moved, p)
	}
	if err := GenerateSelfSignedCert(); err != nil {
		return err
	}
	pruneReplaced()
	return nil
}

// replacedSuffix marks a pair kept aside by a regeneration.
const replacedSuffix = ".replaced-"

// pruneReplaced keeps only the most recent set aside. Flash on these boards
// is measured in tens of megabytes, and without this every regeneration
// leaves another certificate and private key behind for good, with only SSH
// to clear them.
func pruneReplaced() {
	entries, err := os.ReadDir(sslDir)
	if err != nil {
		return
	}
	var stamps []string
	seen := map[string]bool{}
	for _, e := range entries {
		i := strings.LastIndex(e.Name(), replacedSuffix)
		if i < 0 {
			continue
		}
		if stamp := e.Name()[i+len(replacedSuffix):]; !seen[stamp] {
			seen[stamp] = true
			stamps = append(stamps, stamp)
		}
	}
	if len(stamps) <= 1 {
		return
	}
	sort.Strings(stamps) // the stamp sorts chronologically by construction
	for _, old := range stamps[:len(stamps)-1] {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), replacedSuffix+old) {
				_ = os.Remove(filepath.Join(sslDir, e.Name()))
			}
		}
	}
}

// RestoreHTTPSConfig puts back an earlier TLS configuration verbatim, paths
// included. It is the undo for a change that was written and then turned out
// not to serve; turning HTTPS off instead would be a different state, not the
// one the caller had.
func RestoreHTTPSConfig(enabled bool, certFile, keyFile string) error {
	if !uciSectionExists("netgrip.main") {
		return nil
	}
	ops := []executor.Op{{Kind: "uci_set", Args: []string{"netgrip.main.https", boolOption(enabled)}}}
	// Empty means the option was not set: delete it rather than writing an
	// empty string, which would pin the pair to a path that is not there.
	for _, opt := range []struct{ key, value string }{
		{"https_cert", certFile},
		{"https_key", keyFile},
	} {
		if opt.value == "" {
			ops = append(ops, executor.Op{Kind: "uci_delete", Args: []string{"netgrip.main." + opt.key}})
			continue
		}
		ops = append(ops, executor.Op{Kind: "uci_set", Args: []string{"netgrip.main." + opt.key, opt.value}})
	}
	// No snapshot here, deliberately. This IS the undo; taking a fresh one
	// would capture the state being undone, so a failure partway would
	// "roll back" to exactly what the caller is trying to get rid of.
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"netgrip"}})
	return executor.Apply(ops, nil)
}

func boolOption(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// ensureNetgripPackage makes sure /etc/config/netgrip and its panel section
// exist, because everything below assumes they do.
//
// `uci set` cannot create a missing package, and `uci export` - which the
// snapshot uses - cannot read one either, so on a router that has never
// written this file every panel save failed before it began. Only `uci
// import` creates it. Verified on a router: both return "Entry not found".
//
// The -m is the whole point, and it is a GLOBAL option that goes before the
// subcommand: `uci -m import`, not `uci import -m`, which is a usage error.
// Without it, import REPLACES the package, so a document holding only the
// panel section would take netgrip.wizard and netgrip.selfupdate with it.
//
// The section is named rather than anonymous so that `uci set
// netgrip.main.<option>` resolves; a bare `config panel` would be @panel[0],
// which cannot be addressed by name.
func ensureNetgripPackage() error {
	return EnsureNetgripSection("main", "panel")
}

// EnsureNetgripSection creates one named section of the netgrip package,
// and the package itself if it is not there. It is a no-op when the section
// already exists.
func EnsureNetgripSection(name, sectionType string) error {
	if uciSectionExists("netgrip." + name) {
		return nil
	}
	cmd := exec.Command("uci", "-m", "import", "netgrip")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("config %s '%s'\n", sectionType, name))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("init netgrip.%s: %s", name, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("uci", "commit", "netgrip").CombinedOutput(); err != nil {
		return fmt.Errorf("commit netgrip.%s: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}

// applyNetgripConfig writes and commits a set of netgrip options, restoring
// the file if any of them fails.
//
// The commit has to be the last op and the snapshot is what makes that safe.
// Without it a failure halfway left the earlier sets staged but uncommitted
// in /tmp/.uci, where the next unrelated `uci commit netgrip` - a plain
// session-timeout save, for instance - would pick them up and apply a change
// nobody had asked for.
func applyNetgripConfig(ops []executor.Op) error {
	if err := ensureNetgripPackage(); err != nil {
		return err
	}
	snap, err := executor.Snapshot("netgrip")
	if err != nil {
		return fmt.Errorf("snapshot netgrip: %w", err)
	}
	ops = append(ops, executor.Op{Kind: "uci_commit", Args: []string{"netgrip"}})
	if err := executor.Apply(ops, nil); err != nil {
		_ = executor.Restore("netgrip", snap)
		return err
	}
	return nil
}
