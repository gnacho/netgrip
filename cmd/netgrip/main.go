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
	port := flag.Int("port", 8090, "listen port")
	rpcdURL := flag.String("rpcd-url", auth.DefaultRPCdURL, "rpcd JSON-RPC endpoint used for login validation")
	flag.Parse()

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
	modules.StartFleetDiscovery(version, *port)
	modules.StartPoEWatchdog()
	log.Printf("netgrip %s listening on %s (rpcd: %s)", version, addr, resolvedRPCd)
	if err := http.ListenAndServe(addr, server.New(resolvedRPCd, version)); err != nil {
		// One-shot actionable hint instead of a respawn loop of bare
		// "address already in use" lines (#210).
		log.Printf("cannot listen on %s: %v", addr, err)
		if strings.Contains(err.Error(), "address already in use") {
			log.Printf("port %d is busy; pick another with -port (GL.iNet firmware serves its own web UI on 8080)", *port)
		}
		log.Fatal(err)
	}
}
