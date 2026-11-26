// tlscert.go — the certificate/key pair the panel serves HTTPS with, re-read
// from disk when it changes.
//
// The default pair is uhttpd's, already on the router: the panel inherits the
// same trust decision as the router web UI instead of asking for a second one. px5g
// regenerates that pair when it expires, so loading it once at startup would
// leave the panel serving an expired certificate until the next restart -
// which on a router can be months.
package server

import (
	"crypto/tls"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/certs"
	"github.com/gnacho/netgrip/internal/modules"
)

// Both pairs come from the modules package, which is what writes and manages
// them: a second copy of the paths here would report "a configured
// certificate" for the panel's own the first time either moved.
//
// Variables so tests can pin them without a uci binary or a real /etc.
var (
	// panelPair is the pair the panel generates for itself from Settings
	// (GenerateSelfSignedCert). If it is there, it is the one the user
	// asked for.
	panelPair = modules.PanelCertPaths

	// routerPair is uhttpd's pair, the fallback: the one the router web UI serves, so a
	// browser that already accepted that certificate sees nothing new beyond
	// the different origin. Read from uhttpd's own configuration rather than
	// assumed, because the conventional paths are only a default - a build
	// that moves them would otherwise hide the option entirely.
	routerPair = modules.RouterCertPaths
)

const (

	// retryAfterFailure: after a failed reload (the pair half-written while
	// px5g regenerates it) the disk is left alone for this long, so a broken
	// pair does not mean a stat and a read on every handshake.
	retryAfterFailure = 30 * time.Second

	// statInterval: the longest a rotated pair goes unnoticed. Without it
	// every handshake stats two files, which serialises handshakes behind
	// the mutex for no benefit - px5g rotates a certificate every couple of
	// years, not every connection.
	statInterval = 10 * time.Second
)

// CertReloader serves a cert/key pair and repopulates it when either file's
// mtime moves. If the reload fails it keeps serving the last good pair: a
// half-written regeneration must not take the listener down or, worse, leave
// the panel unreachable.
type CertReloader struct {
	certPath string
	keyPath  string

	mu        sync.Mutex
	cert      *tls.Certificate
	mtime     time.Time
	lastFail  time.Time
	lastCheck time.Time
}

// NewCertReloader loads the initial pair. It returns an error if the pair is
// missing or does not parse: whoever asked for TLS should find out at startup,
// not at the first handshake.
func NewCertReloader(certPath, keyPath string) (*CertReloader, error) {
	defCert, defKey := routerPair()
	if certPath == "" {
		certPath = defCert
	}
	if keyPath == "" {
		keyPath = defKey
	}
	r := &CertReloader{certPath: certPath, keyPath: keyPath}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// TLSConfig is the listener's configuration: GetCertificate rather than a
// static list, so that reloading has any effect.
func (r *CertReloader) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: r.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}
}

// GetCertificate implements tls.Config.GetCertificate.
func (r *CertReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.lastCheck) < statInterval {
		return r.cert, nil
	}
	r.lastCheck = now
	mt, err := newestMtime(r.certPath, r.keyPath)
	if err != nil {
		return r.cert, nil // stat failed: serve what is cached
	}
	if mt.After(r.mtime) && now.Sub(r.lastFail) > retryAfterFailure {
		if err := r.reload(); err != nil {
			r.lastFail = now
			return r.cert, nil
		}
	}
	return r.cert, nil
}

// reload repopulates the cache from disk. Call it with r.mu held, except in
// the constructor.
//
// The mtime is taken BEFORE the read, not after: a pair rewritten in between
// would otherwise be recorded under the newer mtime while the older content
// is cached, and the update would never be picked up.
func (r *CertReloader) reload() error {
	mt, err := newestMtime(r.certPath, r.keyPath)
	if err != nil {
		return err
	}
	cert, err := certs.LoadPair(r.certPath, r.keyPath)
	if err != nil {
		return err
	}
	// Serve it either way - refusing would mean no panel at all - but say
	// so. A rotation that produces an expired pair, or the empty
	// subjectAltName px5g used to write, arrives by this path without ever
	// passing the gate in Settings, and the symptom is a browser that will
	// not open the panel with nothing in the log to explain it.
	if err := certs.Usable(r.certPath, r.keyPath, time.Now()); err != nil {
		log.Printf("tls: serving %s even though %v", r.certPath, err)
	}
	r.cert = &cert
	r.mtime = mt
	r.lastFail = time.Time{}
	return nil
}

// forgetStatCache makes the next handshake look at the disk again. Only the
// tests use it: they rewrite a pair in the same instant they check it, which
// is not something a router does.
func (r *CertReloader) forgetStatCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastCheck = time.Time{}
}

// newestMtime is the more recent mtime of the two files: one of them moving is
// enough for the pair to be a different one.
func newestMtime(paths ...string) (time.Time, error) {
	var newest time.Time
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return time.Time{}, err
		}
		if fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	return newest, nil
}

// certCandidates are the pairs to try, in the order a user would expect: what
// they asked for explicitly; failing that, the one they generated from the
// panel; failing that, uhttpd's, which a router always has.
//
// The panel's own comes before uhttpd's deliberately: generating it is a
// deliberate act from Settings, and whoever did it expected it to be used.
func certCandidates(certPath, keyPath string) [][2]string {
	var out [][2]string
	if certPath != "" && keyPath != "" {
		out = append(out, [2]string{certPath, keyPath})
	}
	panelCert, panelKey := panelPair()
	routerCert, routerKey := routerPair()
	return append(out,
		[2]string{panelCert, panelKey},
		[2]string{routerCert, routerKey},
	)
}

// ResolveCertPaths is the pair that would be served: the first that exists.
func ResolveCertPaths(certPath, keyPath string) (string, string) {
	for _, c := range certCandidates(certPath, keyPath) {
		if fileExists(c[0]) && fileExists(c[1]) {
			return c[0], c[1]
		}
	}
	return routerPair()
}

// OpenCertificate opens the first usable pair and says which one it was.
//
// A configured pair being missing or unloadable is no reason to leave the
// panel unstarted: a deleted certificate, a half-written file or a path
// pointing at something no longer there would leave the router with no panel
// until somebody comes in over SSH. Another is served and the fact is logged.
// What never happens is falling back to plaintext: if none of them serve, this
// returns an error and the caller refuses to start.
func OpenCertificate(certPath, keyPath string) (r *CertReloader, usedCert string, err error) {
	var firstErr error
	for _, c := range certCandidates(certPath, keyPath) {
		r, err := NewCertReloader(c[0], c[1])
		if err == nil {
			return r, c[0], nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, "", firstErr
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
