package modules

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func parseGenerated(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	blk, _ := pem.Decode(certPEM)
	if blk == nil {
		t.Fatal("certificate is not PEM")
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return c
}

// The bug this replaced: the generated certificate carried a subjectAltName
// extension that was present and EMPTY. OpenSSL and curl accept that, so
// every command-line check passed, while Chromium refuses to load the
// certificate at all - "the website sent scrambled credentials", with no way
// to continue. A panel nobody can open in a browser is not served.
func TestGeneratedCertHasANonEmptySAN(t *testing.T) {
	certPEM, _, err := buildSelfSigned("router", []net.IP{net.ParseIP("192.0.2.1")}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := parseGenerated(t, certPEM)
	if len(c.DNSNames) == 0 && len(c.IPAddresses) == 0 {
		t.Fatal("subjectAltName is empty; Chromium rejects the certificate outright")
	}
}

// Browsers match the SAN, not the CN. A router is reached by address, so the
// addresses have to be in there or every visit warns about a name mismatch.
func TestGeneratedCertNamesTheAddressesItIsReachedBy(t *testing.T) {
	ips := []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("198.51.100.7")}
	certPEM, _, err := buildSelfSigned("router", ips, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := parseGenerated(t, certPEM)
	for _, want := range ips {
		found := false
		for _, got := range c.IPAddresses {
			if got.Equal(want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s missing from the SAN", want)
		}
	}
	if len(c.DNSNames) != 1 || c.DNSNames[0] != "router" {
		t.Errorf("DNS names = %v, want the hostname", c.DNSNames)
	}
}

// A router without a hostname still needs a certificate with a name in it.
func TestGeneratedCertFallsBackToAName(t *testing.T) {
	certPEM, _, err := buildSelfSigned("", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := parseGenerated(t, certPEM)
	if len(c.DNSNames) == 0 || c.DNSNames[0] == "" {
		t.Fatalf("DNS names = %v, want a fallback name", c.DNSNames)
	}
	if c.Subject.CommonName == "" {
		t.Error("CN is empty")
	}
}

// The pair has to be usable together, and valid now: a router without an RTC
// boots in 1970, so notBefore is backdated.
func TestGeneratedPairIsUsableAndValidNow(t *testing.T) {
	now := time.Now()
	certPEM, keyPEM, err := buildSelfSigned("router", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	c := parseGenerated(t, certPEM)
	if c.NotBefore.After(now) {
		t.Errorf("notBefore %v is in the future", c.NotBefore)
	}
	if !c.NotAfter.After(now.Add(365 * 24 * time.Hour)) {
		t.Errorf("notAfter %v is too soon", c.NotAfter)
	}
	blk, _ := pem.Decode(keyPEM)
	if blk == nil {
		t.Fatal("key is not PEM")
	}
	if _, err := x509.ParsePKCS1PrivateKey(blk.Bytes); err != nil {
		t.Fatalf("key does not parse: %v", err)
	}
	if c.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("certificate cannot be used for a TLS handshake")
	}
}

// The SAN is shown to everyone who opens the panel, and it must not
// advertise the router's uplink address: nothing reaches the panel from
// outside, and the entry goes stale the moment the provider changes it.
//
// Which address that is comes from the uplink device, not from the range.
// The range was the first attempt and it was wrong both ways round: it threw
// away the LAN's IPv6 global addresses, which any ISP that delegates a prefix
// hands out, and it kept the uplink of a router sitting behind a modem on
// 192.168.100.x.
func TestSANNamesEveryLANAddressAndNotTheUplink(t *testing.T) {
	ifaces := []ifaceAddrs{
		{name: "br-lan", addrs: []net.IP{
			net.ParseIP("192.168.99.1"),      // the LAN gateway
			net.ParseIP("2001:db8:1::1"),     // a global address from a delegated prefix
			net.ParseIP("fd00:dead:beef::1"), // ULA
			net.ParseIP("fe80::1"),           // link-local: never named
		}},
		{name: "tailscale0", addrs: []net.IP{net.ParseIP("100.100.0.9")}}, // CGNAT, a real way in
		{name: "pppoe-wan", addrs: []net.IP{
			net.ParseIP("203.0.113.9"), // the uplink: must stay out
			net.ParseIP("2001:db8:ff::9"),
		}},
	}
	got := map[string]bool{}
	for _, ip := range sanAddresses(ifaces, map[string]bool{"pppoe-wan": true}) {
		got[ip.String()] = true
	}

	for _, want := range []string{"192.168.99.1", "2001:db8:1::1", "fd00:dead:beef::1", "100.100.0.9"} {
		if !got[want] {
			t.Errorf("%s is reachable on the LAN and missing from the SAN", want)
		}
	}
	for _, unwanted := range []string{"203.0.113.9", "2001:db8:ff::9", "fe80::1"} {
		if got[unwanted] {
			t.Errorf("%s must not be named: it is the uplink or link-local", unwanted)
		}
	}
}

// With no uplink to exclude - a dumb AP, or ubus not answering - fall back to
// naming private addresses only. A name too few costs a browser warning; a
// name too many is on the wire for two years.
func TestSANFallsBackToPrivateOnlyWithNoUplink(t *testing.T) {
	ifaces := []ifaceAddrs{{name: "br-lan", addrs: []net.IP{
		net.ParseIP("192.168.99.1"),
		net.ParseIP("203.0.113.9"),
	}}}
	for _, ip := range sanAddresses(ifaces, nil) {
		if ip.String() == "203.0.113.9" {
			t.Error("with no uplink resolved, a public address must not be named")
		}
	}
}

func TestAdministrativeAddrSkipsWhatCannotBeConnectedTo(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
		why  string
	}{
		{"192.168.99.1", true, "RFC1918, a LAN gateway"},
		{"2001:db8::1", true, "a global address is reachable on the LAN too"},
		{"fd00:dead:beef::1", true, "ULA"},
		{"169.254.3.4", false, "link-local"},
		{"fe80::1", false, "link-local IPv6"},
		{"127.0.0.1", false, "loopback is appended separately"},
		{"0.0.0.0", false, "unspecified"},
		{"224.0.0.1", false, "multicast"},
	}
	for _, c := range cases {
		if got := administrativeAddr(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("%s (%s): got %v, want %v", c.ip, c.why, got, c.want)
		}
	}
}

// Loopback is still named, because the healthchecks reach the panel that way.
func TestLocalAddressesAlwaysIncludeLoopback(t *testing.T) {
	addrs := localAddresses()
	var v4, v6 bool
	for _, ip := range addrs {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			v4 = true
		}
		if ip.Equal(net.IPv6loopback) {
			v6 = true
		}
	}
	if !v4 || !v6 {
		t.Errorf("loopback missing from %v", addrs)
	}
}

// CertSource decides what the selector shows. Reporting the configured path
// alone meant an unset configuration always read as "the panel's own", even
// on a router where no such pair existed and the router's was being served -
// a selector naming something other than what is in use, which invites
// saving in the belief that nothing changes.
func TestCertSourceFollowsWhatWouldBeUsed(t *testing.T) {
	dir := t.TempDir()
	routerCert, routerKey := pinCertPaths(t, dir)

	// Nothing on disk: the panel's own is what would be generated.
	if got := CertSource(); got != CertSourcePanel {
		t.Errorf("with nothing available: %q, want %q", got, CertSourcePanel)
	}

	// Only the router's exists: that is what the fallback serves, so that is
	// what the selector must show.
	writeTestPair(t, routerCert, routerKey)
	if got := CertSource(); got != CertSourceRouter {
		t.Errorf("with only the router's pair: %q, want %q", got, CertSourceRouter)
	}

	// Once the panel has its own, that wins, matching the boot order.
	writeTestPair(t, certPath, keyPath)
	if got := CertSource(); got != CertSourcePanel {
		t.Errorf("with both: %q, want %q", got, CertSourcePanel)
	}
}

// pinCertPaths points both pairs at a temporary directory. RouterCertPaths is
// stubbed rather than having its fallback variables overridden, because it
// reads uhttpd's uci configuration FIRST: overriding only the fallback passes
// on a development machine, where there is no uci, and silently tests nothing
// on a router, where uci answers.
func pinCertPaths(t *testing.T, dir string) (routerCert, routerKey string) {
	t.Helper()
	origCert, origKey, origRouter := certPath, keyPath, RouterCertPaths
	t.Cleanup(func() {
		certPath, keyPath = origCert, origKey
		RouterCertPaths = origRouter
	})
	certPath = filepath.Join(dir, "panel.crt")
	keyPath = filepath.Join(dir, "panel.key")
	routerCert = filepath.Join(dir, "router.crt")
	routerKey = filepath.Join(dir, "router.key")
	RouterCertPaths = func() (string, string) { return routerCert, routerKey }
	return routerCert, routerKey
}

// writeTestPair puts a real, loadable pair on disk. Two files holding "x"
// used to be enough, which is precisely the gap: the card counted files while
// the listener needed a certificate, so it reported a pair the panel would
// have skipped straight over.
func writeTestPair(t *testing.T, certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM, err := buildSelfSigned("router", []net.IP{net.ParseIP("192.0.2.1")}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSelfSigned(certFile, keyFile, certPEM, keyPEM); err != nil {
		t.Fatal(err)
	}
}

// A pair that exists but cannot be served is not a pair the card may report.
// HasSelfSignedCert was two os.Stat calls, so a file left behind by a failed
// write counted as a certificate.
func TestUnservablePairIsNotReportedAsAvailable(t *testing.T) {
	dir := t.TempDir()
	pinCertPaths(t, dir)

	if err := os.WriteFile(certPath, []byte("not a certificate"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if HasSelfSignedCert() {
		t.Error("two unreadable files are not a certificate the panel can serve")
	}
}

// A pair somebody configured by hand must be reported as such. Reporting it
// as "the panel's own" was not just a wrong label: the card sent that value
// back on the next Save, which rewrote the paths to the panel's own pair and
// threw the configured one away without anyone asking.
func TestCertSourceNamesAHandConfiguredPair(t *testing.T) {
	dir := t.TempDir()
	pinCertPaths(t, dir)
	origPaths := HTTPSCertPaths
	t.Cleanup(func() { HTTPSCertPaths = origPaths })
	HTTPSCertPaths = func() (string, string) {
		return "/srv/ssl/wildcard.pem", "/srv/ssl/wildcard.key"
	}

	if got := CertSource(); got != CertSourceCustom {
		t.Errorf("CertSource = %q, want %q for a pair that is neither the panel's nor the router's",
			got, CertSourceCustom)
	}
}

// The selector must only offer what saving will accept. Offering on "it
// loads" while EnableHTTPS gated on "it is usable" put the router's option on
// screen - preselected, on a router with no pair of its own - and had Save
// answer 500, with nothing the card could do about it.
func TestTheRouterOptionIsOfferedOnlyWhenSaveWouldTakeIt(t *testing.T) {
	dir := t.TempDir()
	routerCert, routerKey := pinCertPaths(t, dir)
	writeEmptySANPair(t, routerCert, routerKey)

	// It loads, so the listener would serve it...
	if !certServable(routerCert, routerKey) {
		t.Fatal("the fixture must load, or this tests the wrong thing")
	}
	// ...and no Chromium-based browser would open it, so it is not offered.
	if HasRouterCert() {
		t.Error("a certificate Save would refuse must not be offered by the selector")
	}
}

// writeEmptySANPair writes the shape px5g used to produce: a pair that parses
// and loads perfectly, carrying a subjectAltName extension that is present
// and empty, which Chromium refuses outright.
func writeEmptySANPair(t *testing.T, certFile, keyFile string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "OpenWrt"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := writeSelfSigned(certFile, keyFile, certPEM, keyPEM); err != nil {
		t.Fatal(err)
	}
}

// A second uplink is still an uplink. mwan3 setups keep a standby link with
// its own address, and a failover promotes it - excluding only the active one
// put the standby's public address in the certificate.
func TestSANExcludesAStandbyUplinkToo(t *testing.T) {
	ifaces := []ifaceAddrs{
		{name: "br-lan", addrs: []net.IP{net.ParseIP("192.168.99.1")}},
		{name: "pppoe-wan", addrs: []net.IP{net.ParseIP("203.0.113.9")}},
		{name: "wwan0", addrs: []net.IP{net.ParseIP("198.51.100.4")}},
	}
	uplinks := map[string]bool{"pppoe-wan": true, "wwan0": true}
	for _, ip := range sanAddresses(ifaces, uplinks) {
		if s := ip.String(); s == "203.0.113.9" || s == "198.51.100.4" {
			t.Errorf("%s belongs to an uplink and must not be named", s)
		}
	}
}

// A dumb AP has no WAN: its only default route sits on br-lan itself, so the
// uplink detection names br-lan and the naive exclusion leaves the SAN with
// nothing but loopback. The one private address on br-lan is the panel's only
// way in, so it must be named anyway.
func TestSanNamesTheLANAddressWhenTheUplinkIsTheLANItself(t *testing.T) {
	ifaces := []ifaceAddrs{{name: "br-lan", addrs: []net.IP{
		net.ParseIP("192.168.99.1"),
		net.ParseIP("fe80::1"),
	}}}
	got := map[string]bool{}
	for _, ip := range sanAddresses(ifaces, map[string]bool{"br-lan": true}) {
		got[ip.String()] = true
	}
	if !got["192.168.99.1"] {
		t.Error("br-lan is the only way into the panel; its address must be in the SAN")
	}
	if got["fe80::1"] {
		t.Error("link-local is never named")
	}
}

// An uplink with only public addresses is still excluded when real
// administrative addresses exist elsewhere: the fallback only fires when
// nothing else is left.
func TestSanStillExcludesTheUplinkWhenALANAddressRemains(t *testing.T) {
	ifaces := []ifaceAddrs{
		{name: "br-lan", addrs: []net.IP{net.ParseIP("192.168.99.1")}},
		{name: "wan0", addrs: []net.IP{net.ParseIP("203.0.113.9")}},
	}
	got := map[string]bool{}
	for _, ip := range sanAddresses(ifaces, map[string]bool{"wan0": true}) {
		got[ip.String()] = true
	}
	if !got["192.168.99.1"] {
		t.Error("the LAN address must be named")
	}
	if got["203.0.113.9"] {
		t.Error("a public uplink address must stay out while a LAN address remains")
	}
}

// The routing tables name the uplinks. Fixtures are the real shape of the
// files, with documentation addresses.
func TestDefaultRouteDevicesAreFoundInBothTables(t *testing.T) {
	// 00000000 destination and mask: a default route. The /24 below is an
	// ordinary on-link route and must not be mistaken for one.
	v4 := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"pppoe-wan\t00000000\t00000000\t0003\t0\t0\t10\t00000000\t0\t0\t0\n" +
		"wwan0\t00000000\t0100A8C0\t0003\t0\t0\t20\t00000000\t0\t0\t0\n" +
		"br-lan\t0063A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n"
	got := defaultRouteDevices(v4)
	if len(got) != 2 || got[0] != "pppoe-wan" || got[1] != "wwan0" {
		t.Errorf("IPv4 default routes = %v, want both uplinks and not br-lan", got)
	}

	v6 := "00000000000000000000000000000000 00 00000000000000000000000000000000 00 " +
		"fe800000000000000000000000000001 00000400 00000001 00000000 00000003 pppoe-wan\n" +
		"20010db8000000000000000000000000 40 00000000000000000000000000000000 00 " +
		"00000000000000000000000000000000 00000100 00000000 00000000 00000001 br-lan\n" +
		"00000000000000000000000000000000 00 00000000000000000000000000000000 00 " +
		"00000000000000000000000000000000 ffffffff 00000001 00000000 00200200       lo\n"
	got6 := defaultRoute6Devices(v6)
	if len(got6) != 1 || got6[0] != "pppoe-wan" {
		t.Errorf("IPv6 default routes = %v, want only the uplink (never lo, never the LAN prefix)", got6)
	}
}
