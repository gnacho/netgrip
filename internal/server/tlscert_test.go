package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pairOpts describes the pair to write. The defaults are the shape px5g
// leaves on an OpenWrt box: an EC key, no IP SAN.
type pairOpts struct {
	cn string
	// der writes both files as raw DER, which is how OpenWrt actually
	// stores uhttpd's pair. Writing only PEM is why the unit tests passed
	// while the panel refused to start on the router.
	der bool
	// mismatched writes a key belonging to a different pair, which is what
	// an interrupted regeneration leaves behind.
	mismatched bool
}

func writePairAt(t *testing.T, certPath, keyPath string, o pairOpts) {
	t.Helper()
	newKey := func() *ecdsa.PrivateKey {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	key := newKey()
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: o.cn},
		DNSNames:     []string{o.cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if o.mismatched {
		key = newKey() // the certificate stays; the key no longer belongs to it
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certBytes, keyBytes := certDER, keyDER
	if !o.der {
		certBytes = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
		keyBytes = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	}
	if err := os.WriteFile(certPath, certBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
}

// writePair is the ordinary PEM pair, the common case.
func writePair(t *testing.T, dir, cn string) (certPath, keyPath string) {
	t.Helper()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	writePairAt(t, certPath, keyPath, pairOpts{cn: cn})
	return certPath, keyPath
}

// pinRouterPair points the uhttpd fallback at a pair of the test's choosing,
// so nothing reaches for /etc or for a uci binary.
func pinRouterPair(t *testing.T, certPath, keyPath string) {
	t.Helper()
	orig := routerPair
	t.Cleanup(func() { routerPair = orig })
	routerPair = func() (string, string) { return certPath, keyPath }
}

// pinPanelPair does the same for the pair the panel generates for itself.
func pinPanelPair(t *testing.T, certPath, keyPath string) {
	t.Helper()
	orig := panelPair
	t.Cleanup(func() { panelPair = orig })
	panelPair = func() (string, string) { return certPath, keyPath }
}

func leafCN(t *testing.T, r *CertReloader) string {
	t.Helper()
	c, err := r.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf.Subject.CommonName
}

// Asking for TLS with a pair that is not there must fail at startup. The
// caller turns this into a refusal to run: serving the router's password in
// clear after TLS was requested is the one outcome nobody would notice.
func TestNewCertReloaderMissingPair(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewCertReloader(filepath.Join(dir, "nope.crt"), filepath.Join(dir, "nope.key")); err == nil {
		t.Fatal("a missing pair must be an error, not a silent fallback")
	}
}

// OpenWrt stores uhttpd's pair in DER, not PEM. Loading only PEM meant the
// panel refused to start with "failed to find any PEM data" the first time it
// was pointed at the router's own certificate - which is the whole point of
// the option. Every test here wrote PEM, so none of them saw it.
func TestLoadsTheDEREncodingOpenWrtActuallyUses(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "uhttpd.crt")
	keyPath := filepath.Join(dir, "uhttpd.key")
	writePairAt(t, certPath, keyPath, pairOpts{cn: "OpenWrt", der: true})

	r, err := NewCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatalf("a DER pair must load: %v", err)
	}
	if got := leafCN(t, r); got != "OpenWrt" {
		t.Errorf("CN = %q, want the DER certificate", got)
	}
}

// Both encodings must be accepted for each file independently: nothing says
// a hand-assembled pair uses the same one for the certificate and the key.
func TestLoadsAMixedEncodingPair(t *testing.T) {
	dir := t.TempDir()
	// A DER certificate beside a PEM key, and the other way round.
	derCert, pemKey := filepath.Join(dir, "a.crt"), filepath.Join(dir, "a.key")
	writePairAt(t, derCert, pemKey, pairOpts{cn: "mixed"})
	raw, err := os.ReadFile(derCert)
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(raw)
	if err := os.WriteFile(derCert, blk.Bytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCertReloader(derCert, pemKey); err != nil {
		t.Errorf("DER certificate with a PEM key must load: %v", err)
	}
}

// The check that separates "two files loaded" from "two files are a pair".
//
// tls.X509KeyPair makes it for PEM; the DER path hand-built the certificate
// and skipped it. A mismatched pair then loaded without complaint, the panel
// started, and the log said "listening on https://" while every handshake
// failed with "private key does not match public key" - unreachable, with no
// plaintext fallback by design, so the only way back in is SSH. An
// interrupted regeneration is all it takes.
func TestAMismatchedPairIsRefusedInBothEncodings(t *testing.T) {
	for _, der := range []bool{false, true} {
		dir := t.TempDir()
		certPath := filepath.Join(dir, "cert")
		keyPath := filepath.Join(dir, "key")
		writePairAt(t, certPath, keyPath, pairOpts{cn: "half-written", der: der, mismatched: true})

		if _, err := NewCertReloader(certPath, keyPath); err == nil {
			t.Errorf("der=%v: a key that does not match the certificate must be refused at load, "+
				"not one handshake at a time", der)
		}
	}
}

// px5g regenerates uhttpd's pair when it expires. A panel that cached the old
// one at boot would serve a dead certificate for months, so a changed file
// must be picked up without a restart.
func TestCertReloaderPicksUpARegeneratedPair(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writePair(t, dir, "first")
	r, err := NewCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := leafCN(t, r); got != "first" {
		t.Fatalf("initial CN = %q", got)
	}

	// Rewrite in place with a later mtime, as a regeneration would.
	writePairAt(t, certPath, keyPath, pairOpts{cn: "second"})
	future := time.Now().Add(2 * time.Second)
	for _, p := range []string{certPath, keyPath} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatal(err)
		}
	}
	r.forgetStatCache()
	if got := leafCN(t, r); got != "second" {
		t.Fatalf("after regeneration CN = %q, want the new pair", got)
	}
}

// Between checks the cached pair is served without touching the disk: two
// stats per handshake, under the one mutex, is a cost paid on every
// connection for a file that changes every couple of years.
func TestCertReloaderDoesNotStatOnEveryHandshake(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writePair(t, dir, "cached")
	r, err := NewCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	leafCN(t, r) // the first call is the one that looks
	writePairAt(t, certPath, keyPath, pairOpts{cn: "rotated"})
	future := time.Now().Add(2 * time.Second)
	for _, p := range []string{certPath, keyPath} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatal(err)
		}
	}
	if got := leafCN(t, r); got != "cached" {
		t.Errorf("CN = %q, want the cached pair until the next check is due", got)
	}
}

// A pair caught half-written must not take the listener down: keep serving
// the last good certificate and try again later.
func TestCertReloaderKeepsLastGoodPairOnGarbage(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writePair(t, dir, "good")
	r, err := NewCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\ntruncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(certPath, future, future); err != nil {
		t.Fatal(err)
	}
	r.forgetStatCache()
	if got := leafCN(t, r); got != "good" {
		t.Fatalf("CN = %q, want the last good pair while the new one is unreadable", got)
	}
}

// A pair that vanishes (a regeneration that removes before writing) is the
// same case: serve what we have rather than failing the handshake.
func TestCertReloaderSurvivesAVanishedPair(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writePair(t, dir, "good")
	r, err := NewCertReloader(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(certPath)
	r.forgetStatCache()
	if got := leafCN(t, r); got != "good" {
		t.Fatalf("CN = %q, want the cached pair when the file is gone", got)
	}
}

// The fallback is uhttpd's pair: that is what makes the panel inherit the
// trust decision the operator already made for the router web UI. The paths come from
// uhttpd's own configuration, because the conventional /etc/uhttpd.crt is
// only the default value of a UCI option and a firmware may move it.
func TestFallbackPairIsWhateverUhttpdIsConfiguredWith(t *testing.T) {
	dir := t.TempDir()
	moved, movedKey := filepath.Join(dir, "ssl", "moved.crt"), filepath.Join(dir, "ssl", "moved.key")
	if err := os.MkdirAll(filepath.Join(dir, "ssl"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePairAt(t, moved, movedKey, pairOpts{cn: "moved"})
	pinRouterPair(t, moved, movedKey)

	pinPanelPair(t, filepath.Join(dir, "none.crt"), filepath.Join(dir, "none.key"))

	gotCert, gotKey := ResolveCertPaths("", "")
	if gotCert != moved || gotKey != movedKey {
		t.Errorf("resolved %q/%q, want uhttpd's configured pair %q/%q", gotCert, gotKey, moved, movedKey)
	}
	r, err := NewCertReloader("", "")
	if err != nil {
		t.Fatalf("an empty path must fall back to uhttpd's configured pair: %v", err)
	}
	if got := leafCN(t, r); got != "moved" {
		t.Errorf("CN = %q, want the pair uhttpd names", got)
	}
}

// A configured pair that is missing must not take the panel down: the point
// of HTTPS here is that the panel keeps working over TLS, and a deleted or
// half-written certificate would otherwise leave the router with no panel
// until somebody logs in over SSH. Falling back is still TLS; falling back
// to plaintext is what must never happen.
func TestOpenCertificateFallsBackToAUsablePair(t *testing.T) {
	dir := t.TempDir()
	goodCert, goodKey := writePair(t, dir, "fallback")

	// Point the "configured" pair at nothing, and make the fallbacks the
	// good pair by overriding the package paths for the test.
	pinPanelPair(t, goodCert, goodKey)
	pinRouterPair(t, filepath.Join(dir, "no.crt"), filepath.Join(dir, "no.key"))

	r, usedCert, err := OpenCertificate(filepath.Join(dir, "gone.crt"), filepath.Join(dir, "gone.key"))
	if err != nil {
		t.Fatalf("should have fallen back, got %v", err)
	}
	if usedCert != goodCert {
		t.Errorf("used %s, want the fallback %s", usedCert, goodCert)
	}
	if got := leafCN(t, r); got != "fallback" {
		t.Errorf("serving CN %q", got)
	}
}

// The fallback covers an unloadable pair, not just a missing one: a
// mismatched or corrupt certificate must be stepped over the same way.
func TestOpenCertificateStepsOverAnUnloadablePair(t *testing.T) {
	dir := t.TempDir()
	badCert, badKey := filepath.Join(dir, "bad.crt"), filepath.Join(dir, "bad.key")
	writePairAt(t, badCert, badKey, pairOpts{cn: "broken", mismatched: true})
	goodCert, goodKey := writePair(t, dir, "good")

	pinPanelPair(t, goodCert, goodKey)
	pinRouterPair(t, filepath.Join(dir, "no.crt"), filepath.Join(dir, "no.key"))

	_, usedCert, err := OpenCertificate(badCert, badKey)
	if err != nil {
		t.Fatalf("an unloadable configured pair must fall back, got %v", err)
	}
	if usedCert != goodCert {
		t.Errorf("used %s, want the fallback %s", usedCert, goodCert)
	}
}

// With nothing usable anywhere it must fail, so the caller refuses to start
// rather than serving the router's password in clear.
func TestOpenCertificateFailsWhenNothingIsUsable(t *testing.T) {
	dir := t.TempDir()
	pinPanelPair(t, filepath.Join(dir, "a.crt"), filepath.Join(dir, "a.key"))
	pinRouterPair(t, filepath.Join(dir, "b.crt"), filepath.Join(dir, "b.key"))

	if _, _, err := OpenCertificate("", ""); err == nil {
		t.Fatal("no usable pair must be an error")
	}
}
