package modules

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gnacho/netpulse/agent/runtime"
)

// registerExecutorToken comparte el token del executor con NetPulse (#411):
// lo necesita para delegar ops (p. ej. la propagación de MQTT) sin depender
// de que el router haya subido un backup de configuración antes. Fail-silent:
// si el servidor no está, el agente lo reintenta en su próximo arranque.
// executorHTTPClient builds the client used to reach the NetPulse server: the
// same SPKI-pinned transport the embedded agent uses, so a server with a
// private CA is accepted here too (#606). Without a usable pin it falls back
// to the default client and says so.
func executorHTTPClient(server, serverFP string) *http.Client {
	client := &http.Client{Timeout: 10 * time.Second}
	tr, err := runtime.ServerTransport(server, serverFP)
	if err != nil {
		log.Printf("netpulse: executor token: transporte sin pin (%v)", err)
		return client
	}
	client.Transport = tr
	return client
}

func registerExecutorToken(server, slug, agentToken, serverFP string) {
	body, err := json.Marshal(map[string]string{"token": GetExecutorToken()})
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST",
		strings.TrimRight(server, "/")+"/api/agents/executor-token", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agentToken)
	req.Header.Set("X-Router-ID", slug)
	client := executorHTTPClient(server, serverFP)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("netpulse: no se pudo registrar el executor token: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("netpulse: registro del executor token respondió %d", resp.StatusCode)
	}
}
