package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gnacho/netgrip/internal/auth"
	"github.com/gnacho/netgrip/internal/modules"
	"github.com/gnacho/netgrip/internal/server"
)

var version = "dev"

func main() {
	listen := flag.String("listen", "0.0.0.0", "listen address")
	showVersion := flag.Bool("version", false, "print version and exit")
	port := flag.Int("port", 8090, "listen port")
	rpcdURL := flag.String("rpcd-url", auth.DefaultRPCdURL, "rpcd JSON-RPC endpoint used for login validation")
	useTLS := flag.Bool("https", false, "serve the panel over HTTPS on the listen port")
	tlsCert := flag.String("https-cert", "", "certificate to serve with -https (default: the panel's own, else uhttpd's)")
	tlsKey := flag.String("https-key", "", "private key to serve with -https (default: the panel's own, else uhttpd's)")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	// The flag always has a value (its default), so only treat it as an
	// explicit override when it differs from the default endpoint.
	explicit := ""
	if *rpcdURL != auth.DefaultRPCdURL {
		explicit = *rpcdURL
	}
	resolvedRPCd := auth.DetectRPCdEndpoint(explicit)
	if resolvedRPCd == "" {
		log.Printf("no rpcd endpoint answered among the known candidates; falling back to %s", *rpcdURL)
		resolvedRPCd = *rpcdURL
	}

	addr := fmt.Sprintf("%s:%d", *listen, *port)
	modules.StartHistoryCollector()
	modules.StartMonitor()
	modules.StartNetPulseAgent(version)
	modules.StartMQTT(version)
	modules.StartSelfUpdateScheduler(version)
	modules.StartParentalScheduler()
	modules.StartQuotaScheduler()
	modules.StartFleetDiscovery(version, *port)
	modules.StartPoEWatchdog()
	modules.StartWifiScheduleScheduler()
	modules.StartBanipWarmup()
	modules.StartAnnouncements()
	// The login form posts the router's root password, so how the panel
	// listens is a security decision, not a preference. With -https it
	// serves TLS on the same port: plaintext then fails at the handshake,
	// which is the point - a redirect cannot protect a password that has
	// already been sent to it.
	//
	// The certificate is opened before the server is built, so the path it
	// ended up serving is set once, at construction, and never written
	// while handlers are reading it.
	var reloader *server.CertReloader
	servingCert := ""
	if *useTLS {
		wanted, wantedKey := server.ResolveCertPaths(*tlsCert, *tlsKey)
		reloaded, certFile, err := server.OpenCertificate(*tlsCert, *tlsKey)
		if err != nil {
			// No usable pair anywhere. Never fall back to plaintext: a
			// panel quietly serving the password in clear after HTTPS was
			// asked for is worse than one that refuses to start, because
			// nothing says so.
			//
			// Two lines, because the second is the one somebody can act
			// on. procd gives up after a bounded number of retries, so
			// this is what is left in the log when the panel is down; it
			// has to say how to get it back without reading the source.
			log.Printf("-https: no usable certificate (%s and %s): %v", wanted, wantedKey, err)
			log.Fatal("-https: the panel will not serve the router's password in clear, so it is not starting. " +
				"To bring it back on HTTP: uci set netgrip.main.https=0; uci commit netgrip; /etc/init.d/netgrip restart")
		}
		if certFile != wanted {
			log.Printf("-https: %s could not be used, serving %s instead", wanted, certFile)
		}
		reloader, servingCert = reloaded, certFile
	}
	panel := server.New(resolvedRPCd, version, *useTLS, servingCert)
	scheme, srv := "http", &http.Server{Addr: addr, Handler: panel}
	serve := srv.ListenAndServe
	if reloader != nil {
		srv.TLSConfig = reloader.TLSConfig()
		serve = func() error { return srv.ListenAndServeTLS("", "") }
		scheme = "https"
	}
	log.Printf("netgrip %s listening on %s://%s (rpcd: %s)", version, scheme, addr, resolvedRPCd)
	if err := serve(); err != nil {
		// One-shot actionable hint instead of a respawn loop of bare
		// "address already in use" lines (#210).
		log.Printf("cannot listen on %s: %v", addr, err)
		if strings.Contains(err.Error(), "address already in use") {
			log.Printf("port %d is busy; pick another with -port (GL.iNet firmware serves its own web UI on 8080)", *port)
		}
		log.Fatal(err)
	}
}
