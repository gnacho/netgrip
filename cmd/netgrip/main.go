package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gnacho/netgrip/internal/server"
)

var version = "dev"

func main() {
	listen := flag.String("listen", "0.0.0.0", "listen address")
	port := flag.Int("port", 8080, "listen port")
	rpcdURL := flag.String("rpcd-url", "http://127.0.0.1/ubus", "rpcd JSON-RPC endpoint used for login validation")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *listen, *port)
	log.Printf("netgrip %s listening on %s (rpcd: %s)", version, addr, *rpcdURL)
	log.Fatal(http.ListenAndServe(addr, server.New(*rpcdURL, version)))
}
