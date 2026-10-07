// selfsigned.go — the certificate the panel generates to serve itself over
// HTTPS.
//
// Built here rather than by calling px5g or openssl. Not to avoid the fork:
// the pair those produced carried a subjectAltName extension that was present
// and EMPTY, which OpenSSL and curl accept and Chromium refuses outright -
// "the website sent scrambled credentials", with no way to continue. A
// certificate no Chromium-based browser can load is no use to a web panel.
//
// Generating it with crypto/x509 also allows the router's own addresses into
// the SAN, which is what decides whether the browser warns about a name
// mismatch, and drops the dependency on px5g or openssl being installed.
package modules

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

// selfSignedValidity: two years. Long enough not to think about it, short
// enough that a leaked pair is not good for a decade.
const selfSignedValidity = 2 * 365 * 24 * time.Hour

// buildSelfSigned creates the pair in memory. Pure but for the source of
// randomness, so a test can check the thing that actually matters: that the
// SAN has something in it.
func buildSelfSigned(hostname string, addrs []net.IP, now time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("serial: %w", err)
	}
	if hostname == "" {
		hostname = "netgrip"
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: hostname},
		NotBefore:             now.Add(-time.Hour), // slack for clocks with no RTC
		NotAfter:              now.Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		// The SAN is what browsers look at; they stopped reading the CN
		// years ago. The hostname and every address go in, so reaching the
		// panel by IP - which is how a router is reached - does not warn
		// about a name mismatch.
		DNSNames:    []string{hostname},
		IPAddresses: addrs,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, nil
}

// ifaceAddrs is one interface's addresses, so the choice below can be a pure
// function and tested without a router.
type ifaceAddrs struct {
	name  string
	addrs []net.IP
}

// localAddresses are the addresses the router is administered on, and so the
// ones that go in the SAN.
func localAddresses() []net.IP {
	var found []ifaceAddrs
	ifaces, err := net.Interfaces()
	if err != nil {
		return []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		var ips []net.IP
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				ips = append(ips, ipn.IP)
			}
		}
		found = append(found, ifaceAddrs{name: ifc.Name, addrs: ips})
	}
	return sanAddresses(found, uplinkDevices())
}

// uplinkDevices are every device carrying a default route, plus whichever one
// ubus considers the active uplink.
//
// Every one of them, not just the active one: mwan3 setups have a second
// uplink sitting there with its own address, and a failover promotes it. The
// routing table is read rather than asked for because it names all of them at
// once and needs no daemon.
func uplinkDevices() map[string]bool {
	out := map[string]bool{}
	if dev := ubus.ActiveWANL3Device(); dev != "" {
		out[dev] = true
	}
	for path, parse := range map[string]func(string) []string{
		"/proc/net/route":      defaultRouteDevices,
		"/proc/net/ipv6_route": defaultRoute6Devices,
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, dev := range parse(string(raw)) {
			out[dev] = true
		}
	}
	return out
}

// defaultRouteDevices reads the IPv4 table: destination and mask both zero is
// a default route, and the interface is the first column.
func defaultRouteDevices(table string) []string {
	var out []string
	for i, line := range strings.Split(table, "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 8 {
			continue // the header, or a truncated line
		}
		if f[1] == "00000000" && f[7] == "00000000" {
			out = append(out, f[0])
		}
	}
	return out
}

// defaultRoute6Devices reads the IPv6 table, where a default route is the
// all-zero destination with prefix length 0 and the device is last.
func defaultRoute6Devices(table string) []string {
	var out []string
	for _, line := range strings.Split(table, "\n") {
		f := strings.Fields(line)
		if len(f) < 10 {
			continue
		}
		if f[0] == strings.Repeat("0", 32) && f[1] == "00" && f[len(f)-1] != "lo" {
			out = append(out, f[len(f)-1])
		}
	}
	return out
}

// sanAddresses picks what to name, given every interface's addresses and the
// devices the uplinks are on.
//
// What must stay out is an uplink's address, for two reasons: the certificate
// is shown to anyone who connects, and it effectively expires every time the
// provider changes that address, which is exactly when nobody is watching.
// Everything else goes in, because the panel is reached by address far more
// often than by name.
//
// The uplinks are resolved from live state rather than guessed from the
// range. Guessing was "RFC1918 only", and it was wrong in both directions: it
// dropped the LAN's IPv6 global addresses, which is what a delegated prefix
// gives every device on an ordinary ISP connection, and Tailscale's
// 100.64.0.0/10, while happily naming the uplink of a router behind a modem
// that hands out 192.168.100.x.
//
// The exclusion has a floor, though: on a dumb AP the only default route sits
// on br-lan itself, so br-lan reads as the uplink and dropping it leaves
// nothing but loopback - a SAN that names no way in at all. When no
// administrative address survives the exclusion, fall back to naming private
// addresses from every interface, the uplink included: that address is the
// panel's only way in, and on an AP it is a LAN address, not a provider one.
func sanAddresses(ifaces []ifaceAddrs, uplinks map[string]bool) []net.IP {
	var out []net.IP
	for _, ifc := range ifaces {
		if uplinks[ifc.name] {
			continue
		}
		for _, ip := range ifc.addrs {
			if !administrativeAddr(ip) {
				continue
			}
			if len(uplinks) == 0 && !ip.IsPrivate() {
				continue
			}
			out = append(out, ip)
		}
	}
	if len(out) == 0 {
		// The exclusion (or the absence of any candidate) left nothing to
		// name. A certificate whose SAN names no reachable address is worse
		// than one that names the uplink's private one.
		for _, ifc := range ifaces {
			for _, ip := range ifc.addrs {
				if administrativeAddr(ip) && ip.IsPrivate() {
					out = append(out, ip)
				}
			}
		}
	}
	// Loopback last: reaching the panel on 127.0.0.1 is unusual, but the
	// healthchecks do it.
	return append(out, net.IPv4(127, 0, 0, 1), net.IPv6loopback)
}

// administrativeAddr: an address something could actually connect to. Not
// loopback (appended separately), nor link-local, nor anything that is not a
// unicast address in the first place.
func administrativeAddr(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return true
}

// writeSelfSigned puts the pair on disk with the permissions a private key
// deserves.
func writeSelfSigned(certPath, keyPath string, certPEM, keyPEM []byte) error {
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return fmt.Errorf("write certificate: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	return nil
}
